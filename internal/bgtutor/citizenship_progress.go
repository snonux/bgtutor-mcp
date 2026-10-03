package bgtutor

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// CitizenshipTracker stores learner progress separately from prepared content.
// The same server should be the only writer, as with the vocabulary notebook.
type CitizenshipTracker struct {
	Path    string
	Library *CitizenshipLibrary
	mu      sync.Mutex
}

// CitizenshipSectionProgress is a tutor-assessed section and its content fingerprint.
// Fingerprints prevent an edited or reordered section from remaining mastered.
type CitizenshipSectionProgress struct {
	MaterialID  string `json:"material_id"`
	Index       int    `json:"index"`
	Status      string `json:"status"`
	Note        string `json:"note,omitempty"`
	ContentHash string `json:"content_hash"`
	UpdatedAt   string `json:"updated_at"`
}

// CitizenshipExamProgress records the latest attempt at a test and its weak topics.
type CitizenshipExamProgress struct {
	TestID     string   `json:"test_id"`
	Score      int      `json:"score"`
	Total      int      `json:"total"`
	WeakTopics []string `json:"weak_topics"`
	UpdatedAt  string   `json:"updated_at"`
}

// CitizenshipPlanSection is one section to drill; status is unseen, review or mastered.
type CitizenshipPlanSection struct {
	MaterialID string `json:"material_id"`
	Index      int    `json:"index"`
	Title      string `json:"title"`
	Status     string `json:"status"`
	Note       string `json:"note,omitempty"`
}

// CitizenshipPlan combines a complete coverage checklist and latest mock results.
type CitizenshipPlan struct {
	Phases   []string                  `json:"phases"`
	Sections []CitizenshipPlanSection  `json:"sections"`
	Mastered int                       `json:"mastered_sections"`
	Total    int                       `json:"total_sections"`
	Exams    []CitizenshipExamProgress `json:"latest_exams"`
}

type citizenshipProgressFile struct {
	FormatVersion int                          `json:"format_version"`
	Sections      []CitizenshipSectionProgress `json:"sections"`
	Exams         []CitizenshipExamProgress    `json:"exams"`
}

// NewCitizenshipTracker returns a tracker stored at path.
func NewCitizenshipTracker(path string, lib *CitizenshipLibrary) *CitizenshipTracker {
	return &CitizenshipTracker{Path: path, Library: lib}
}

// Plan returns every available study section and progress that still matches its content.
func (t *CitizenshipTracker) Plan() (*CitizenshipPlan, error) {
	catalog, err := t.Library.Catalog()
	if err != nil {
		return nil, err
	}
	t.mu.Lock()
	progress, err := t.read()
	t.mu.Unlock()
	if err != nil {
		return nil, err
	}
	plan := &CitizenshipPlan{Phases: citizenshipPhases(), Sections: []CitizenshipPlanSection{}, Exams: progress.Exams}
	for _, material := range catalog.Materials {
		if material.ID == "sources" || material.ID == "study/grammar-rules/README" || material.ID == "videos/README" {
			continue
		}
		for index := 1; index <= material.Sections; index++ {
			lesson, err := t.Library.Lesson(material.ID, index)
			if err != nil {
				return nil, err
			}
			section := planSection(lesson, progress.Sections)
			plan.Sections = append(plan.Sections, section)
			if section.Status == "mastered" {
				plan.Mastered++
			}
		}
	}
	plan.Total = len(plan.Sections)
	return plan, nil
}

// Record marks a section for review or mastered after the tutor has tested it.
func (t *CitizenshipTracker) Record(id string, index int, status, note string) (*CitizenshipSectionProgress, error) {
	if status != "review" && status != "mastered" {
		return nil, userErrorf("status must be review or mastered.")
	}
	lesson, err := t.Library.Lesson(id, index)
	if err != nil {
		return nil, err
	}
	record := CitizenshipSectionProgress{MaterialID: id, Index: index, Status: status, Note: note, ContentHash: citizenshipHash(lesson.Markdown), UpdatedAt: time.Now().UTC().Format(time.RFC3339)}
	t.mu.Lock()
	defer t.mu.Unlock()
	progress, err := t.read()
	if err != nil {
		return nil, err
	}
	for i, old := range progress.Sections {
		if old.MaterialID == id && old.Index == index {
			progress.Sections[i] = record
			return &record, WriteJSONAtomic(t.Path, progress)
		}
	}
	progress.Sections = append(progress.Sections, record)
	return &record, WriteJSONAtomic(t.Path, progress)
}

// RecordExam persists a completed mock's score and weak topics for the next session.
func (t *CitizenshipTracker) RecordExam(grade *CitizenshipGrade) error {
	record := CitizenshipExamProgress{TestID: grade.TestID, Score: grade.Score, Total: grade.Total, WeakTopics: []string{}, UpdatedAt: time.Now().UTC().Format(time.RFC3339)}
	seen := map[string]bool{}
	for _, feedback := range grade.Feedback {
		if !feedback.Correct && !seen[feedback.Topic] {
			record.WeakTopics = append(record.WeakTopics, feedback.Topic)
			seen[feedback.Topic] = true
		}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	progress, err := t.read()
	if err != nil {
		return err
	}
	for i, old := range progress.Exams {
		if old.TestID == grade.TestID {
			progress.Exams[i] = record
			return WriteJSONAtomic(t.Path, progress)
		}
	}
	progress.Exams = append(progress.Exams, record)
	return WriteJSONAtomic(t.Path, progress)
}

func (t *CitizenshipTracker) read() (*citizenshipProgressFile, error) {
	progress := &citizenshipProgressFile{FormatVersion: 1, Sections: []CitizenshipSectionProgress{}, Exams: []CitizenshipExamProgress{}}
	data, err := os.ReadFile(t.Path)
	if os.IsNotExist(err) {
		return progress, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read citizenship progress: %w", err)
	}
	if err := json.Unmarshal(data, progress); err != nil {
		return nil, fmt.Errorf("decode citizenship progress: %w", err)
	}
	if progress.FormatVersion != 1 {
		return nil, fmt.Errorf("unsupported citizenship progress format %d", progress.FormatVersion)
	}
	return progress, nil
}

func planSection(lesson *CitizenshipLesson, records []CitizenshipSectionProgress) CitizenshipPlanSection {
	title := strings.SplitN(lesson.Markdown, "\n", 2)[0]
	section := CitizenshipPlanSection{MaterialID: lesson.MaterialID, Index: lesson.Index, Title: strings.TrimLeft(title, "# "), Status: "unseen"}
	for _, record := range records {
		if record.MaterialID == lesson.MaterialID && record.Index == lesson.Index && record.ContentHash == citizenshipHash(lesson.Markdown) {
			section.Status, section.Note = record.Status, record.Note
		}
	}
	return section
}

func citizenshipHash(content string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(content))) }

func citizenshipPhases() []string {
	return []string{
		"Diagnostic: sit one complete sample and identify weak question types.",
		"Study: drill every study/ section, prioritising weak topics; test recall and use in new examples.",
		"Reading: practise passages and tables from every available test.",
		"Application: drill the guide, official materials and candidate experiences; preserve source dates and uncertainties. sources is a reference list, not a memorisation target.",
		"Interview: rehearse personal answers in Bulgarian about motivation, residence, work and family; this is separate from the written exam.",
		"Mocks: sit all practice and official samples with an external 60-minute timer; withhold keys until submission.",
		"Review: revisit mistakes and review sections, save difficult vocabulary, and resume until coverage and recall are strong.",
	}
}
