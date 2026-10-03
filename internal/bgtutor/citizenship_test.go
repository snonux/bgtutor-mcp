package bgtutor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func citizenshipFixture(t *testing.T) *CitizenshipLibrary {
	t.Helper()
	dir := t.TempDir()
	var paper strings.Builder
	paper.WriteString("# Synthetic practice fixture\n\n## Reading section\n\nA synthetic passage, not real exam content.\n\n")
	for i := 1; i <= 20; i++ {
		if i == 9 {
			paper.WriteString("## Choose the synonym\n\n")
		}
		_, err := paper.WriteString("**" + stringIndex(i) + ".**\nА) first\nБ) second\nВ) third\nГ) fourth\n\n")
		if err != nil {
			t.Fatal(err)
		}
	}
	paper.WriteString("## Отговори и обяснения\n\n|")
	for i := 1; i <= 20; i++ {
		paper.WriteString(" " + stringIndex(i) + " |")
	}
	paper.WriteString("\n|")
	for i := 1; i <= 20; i++ {
		paper.WriteString("---|")
	}
	paper.WriteString("\n|")
	for i := 1; i <= 20; i++ {
		paper.WriteString(" Б |")
	}
	paper.WriteString("\n\n1. Private explanation for question one.\n   Continued explanation.\n")
	if err := os.MkdirAll(filepath.Join(dir, "practice"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "practice", "test-01.md"), []byte(paper.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "guide.md"), []byte("# Guide\n\nIntro.\n\n## One rule\n\nTeach this rule.\n\n### Examples\n\nKeep examples with the rule.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return NewCitizenshipLibrary(dir)
}

func stringIndex(i int) string { return strconv.Itoa(i) }

func TestCitizenshipMarkdownAndPayloadIsolation(t *testing.T) {
	lib := citizenshipFixture(t)
	test, err := lib.Test("practice/test-01")
	if err != nil {
		t.Fatal(err)
	}
	if len(test.Questions) != 20 || len(test.Answers) != 20 || !strings.Contains(test.Answers[0].Explanation, "Continued") {
		t.Fatalf("parsed test: %+v", test)
	}
	if !strings.Contains(test.Questions[8].Prompt, "synonym") || test.Questions[8].Topic != "synonyms" {
		t.Fatal("missing section prompt or topic")
	}
	catalog, err := lib.Catalog()
	if err != nil || len(catalog.Tests) != 1 || !catalog.Tests[0].Ready || len(catalog.Materials) != 1 {
		t.Fatalf("catalog: %+v, %v", catalog, err)
	}
	q, err := lib.Question("practice/test-01", 1)
	if err != nil || q.Reading == "" || q.Position != "1 of 20" || *q.NextIndex != 2 {
		t.Fatalf("question: %+v, %v", q, err)
	}
	last, err := lib.Question("practice/test-01", 20)
	if err != nil || !last.IsLast || last.NextIndex != nil || last.Reading != "" {
		t.Fatalf("last: %+v, %v", last, err)
	}
	exam, err := lib.Exam("practice/test-01")
	if err != nil {
		t.Fatal(err)
	}
	for _, payload := range []any{q, exam} {
		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"correct_answer", "explanation", "answers", "Private explanation"} {
			if strings.Contains(string(data), forbidden) {
				t.Fatalf("pre-submission payload leaks %q: %s", forbidden, data)
			}
		}
	}
}

func TestCitizenshipValidationAndPathErrors(t *testing.T) {
	lib := citizenshipFixture(t)
	for _, id := range []string{"../guide", "/etc/passwd", "practice/../guide", "missing"} {
		if _, err := lib.Test(id); err == nil {
			t.Errorf("Test(%q) should fail", id)
		}
	}
	if _, err := lib.Lesson("practice/test-01", 1); err == nil {
		t.Error("lesson tool must not expose test keys")
	}
	for _, index := range []int{0, 21} {
		if _, err := lib.Question("practice/test-01", index); err == nil {
			t.Errorf("index %d should fail", index)
		}
	}
	path := filepath.Join(lib.Dir, "practice", "test-01.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), "Г) fourth", "Г) ", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := lib.Catalog()
	if err != nil || catalog.Tests[0].Ready || catalog.Tests[0].Problem == "" {
		t.Fatalf("invalid tests should remain visible: %+v %v", catalog, err)
	}
	if _, err := lib.Exam("practice/test-01"); err == nil {
		t.Error("invalid test should not be served")
	}
}

func TestCitizenshipGrading(t *testing.T) {
	lib := citizenshipFixture(t)
	for _, tc := range []struct {
		name, answer     string
		correct, wantErr bool
	}{
		{"correct", " Б ", true, false}, {"wrong", "А", false, false}, {"blank", "", false, false}, {"lowercase", "б", true, false}, {"latin", "B", false, true}, {"multiple", "БВ", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			feedback, err := lib.CheckAnswer("practice/test-01", CitizenshipAnswer{Index: 1, Answer: tc.answer})
			if (err != nil) != tc.wantErr {
				t.Fatalf("error %v", err)
			}
			if err == nil && feedback.Correct != tc.correct {
				t.Fatalf("feedback %+v", feedback)
			}
		})
	}
	for _, score := range []int{0, 11, 12, 20} {
		answers := make([]CitizenshipAnswer, 20)
		for i := range answers {
			answers[i] = CitizenshipAnswer{Index: 20 - i}
			if i < score {
				answers[i].Answer = "Б"
			}
		}
		grade, err := lib.GradeExam("practice/test-01", answers)
		if err != nil || grade.Score != score || grade.Passed != (score >= 12) || grade.Feedback[0].Index != 1 {
			t.Fatalf("score %d: %+v %v", score, grade, err)
		}
	}
	for _, tc := range []struct {
		name    string
		answers []CitizenshipAnswer
	}{
		{"incomplete", []CitizenshipAnswer{{Index: 1, Answer: "Б"}}},
		{"duplicates", make([]CitizenshipAnswer, 20)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := lib.GradeExam("practice/test-01", tc.answers); err == nil {
				t.Error("bad answer sheet should fail")
			}
		})
	}
}

func TestCitizenshipProgressPersistsAndInvalidatesEditedLessons(t *testing.T) {
	lib := citizenshipFixture(t)
	path := filepath.Join(t.TempDir(), "progress.json")
	tracker := NewCitizenshipTracker(path, lib)
	if _, err := tracker.Record("guide", 2, "mastered", "can explain the rule"); err != nil {
		t.Fatal(err)
	}
	restarted := NewCitizenshipTracker(path, lib)
	plan, err := restarted.Plan()
	if err != nil || plan.Mastered != 1 || plan.Total != 2 {
		t.Fatalf("saved plan: %+v %v", plan, err)
	}
	lesson, err := lib.Lesson("guide", 2)
	if err != nil || !strings.Contains(lesson.Markdown, "### Examples") {
		t.Fatalf("examples should stay in section: %+v %v", lesson, err)
	}
	if err := os.WriteFile(filepath.Join(lib.Dir, "guide.md"), []byte("# Guide\n\nIntro.\n\n## Changed rule\n\nNew content."), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err = restarted.Plan()
	if err != nil || plan.Mastered != 0 {
		t.Fatalf("edited lesson must invalidate mastery: %+v %v", plan, err)
	}
	if _, err := tracker.Record("guide", 1, "read", ""); err == nil {
		t.Error("reading alone is not mastery")
	}
	if _, err := tracker.Record("guide", 0, "review", ""); err == nil {
		t.Error("invalid section")
	}
}

func TestCitizenshipProgressConcurrentWritesAndCorruptFile(t *testing.T) {
	lib := citizenshipFixture(t)
	tracker := NewCitizenshipTracker(filepath.Join(t.TempDir(), "saved.json"), lib)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := tracker.Record("guide", 1, "review", "practise again"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	grade := &CitizenshipGrade{TestID: "practice/test-01", Score: 19, Total: 20, Feedback: []CitizenshipFeedback{{Topic: "grammar", Correct: false}}}
	if err := tracker.RecordExam(grade); err != nil {
		t.Fatal(err)
	}
	plan, err := tracker.Plan()
	if err != nil || len(plan.Exams) != 1 || plan.Exams[0].WeakTopics[0] != "grammar" {
		t.Fatalf("plan %+v %v", plan, err)
	}
	if err := os.WriteFile(tracker.Path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := tracker.Record("guide", 1, "mastered", ""); err == nil {
		t.Error("corrupt progress must not be overwritten")
	}
}

func TestCitizenshipLibraryWithSymlinkedRoot(t *testing.T) {
	lib := citizenshipFixture(t)
	link := filepath.Join(t.TempDir(), "citizenship-test")
	if err := os.Symlink(lib.Dir, link); err != nil {
		t.Fatal(err)
	}
	linked := NewCitizenshipLibrary(link)
	catalog, err := linked.Catalog()
	if err != nil || len(catalog.Tests) != 1 {
		t.Fatalf("symlinked root: %+v %v", catalog, err)
	}
	if _, err := linked.Question("practice/test-01", 1); err != nil {
		t.Fatal(err)
	}
}

func TestCitizenshipVideoSubtitles(t *testing.T) {
	lib := citizenshipFixture(t)
	dir := filepath.Join(lib.Dir, "videos")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	captions := "1\r\n00:00:00,000 --> 00:00:05,000\r\nПърво изречение.\r\n\r\n2\r\n00:00:05,000 --> 00:00:10,000\r\nВторо изречение.\r\n"
	if err := os.WriteFile(filepath.Join(dir, "story.srt"), []byte(captions), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := lib.Catalog()
	if err != nil || len(catalog.Materials) != 2 || catalog.Materials[1].MediaPath != "videos/story.mp4" {
		t.Fatalf("video catalog: %+v %v", catalog, err)
	}
	lesson, err := lib.Lesson("videos/story", 2)
	if err != nil || lesson.MediaPath != "videos/story.mp4" || !strings.Contains(lesson.Markdown, "Второ") || !lesson.IsLast {
		t.Fatalf("video lesson: %+v %v", lesson, err)
	}
	for _, bad := range []string{"bad subtitle", "1\nnot a timestamp\ncaption", "2\n00:00:00,000 --> 00:00:05,000\ncaption"} {
		if _, _, err := citizenshipDocumentSections("story.srt", bad); err == nil {
			t.Errorf("invalid subtitle accepted: %q", bad)
		}
	}
}
