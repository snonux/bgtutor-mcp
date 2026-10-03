package mcpserver

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/snonux/bgtutor-mcp/internal/bgtutor"
)

func citizenshipData(t *testing.T) string {
	t.Helper()
	dataDir := t.TempDir()
	dir := filepath.Join(dataDir, "citizenship-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	test := bgtutor.CitizenshipTest{FormatVersion: 1, Title: "Synthetic exam", Source: "unit test fixture", Reading: "A synthetic reading passage."}
	for i := 1; i <= 20; i++ {
		test.Questions = append(test.Questions, bgtutor.CitizenshipQuestion{Index: i, Topic: "grammar", Prompt: "Choose the form.", Choices: []bgtutor.CitizenshipChoice{{Letter: "А", Text: "one"}, {Letter: "Б", Text: "two"}, {Letter: "В", Text: "three"}, {Letter: "Г", Text: "four"}}})
		test.Answers = append(test.Answers, bgtutor.CitizenshipKey{Index: i, Answer: "Б", Explanation: "Secret tutor answer notes."})
	}
	if err := bgtutor.WriteJSONAtomic(filepath.Join(dir, "sample.json"), test); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lesson.md"), []byte("# Rule\n\nTeach one example and wait for the learner."), 0o644); err != nil {
		t.Fatal(err)
	}
	return dataDir
}

func citizenshipConnect(t *testing.T, dataDir string) *mcp.ClientSession {
	t.Helper()
	s, err := NewWithMode(dataDir, "citizenship")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(Handler(s, HTTPOptions{}))
	t.Cleanup(server.Close)
	client := mcp.NewClient(&mcp.Implementation{Name: "citizenship-test"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: server.URL + "/mcp", MaxRetries: -1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestCitizenshipLiveClassOverHTTP(t *testing.T) {
	dataDir := citizenshipData(t)
	session := citizenshipConnect(t, dataDir)
	instructions := session.InitializeResult().Instructions
	for _, want := range []string{"live classroom", "wait", "private tutor answer", "external 60-minute", "Interview"} {
		if !strings.Contains(instructions, want) {
			t.Errorf("missing classroom instruction %q", want)
		}
	}
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil || len(tools.Tools) != 11 {
		t.Fatalf("tools: %+v %v", tools, err)
	}
	catalog, isErr := call(t, session, "list_citizenship_materials", nil)
	if isErr || len(catalog["tests"].([]any)) != 1 {
		t.Fatalf("catalog: %v", catalog)
	}
	lesson, isErr := call(t, session, "get_citizenship_lesson", map[string]any{"material_id": "lesson", "index": 1})
	if isErr || !strings.Contains(lesson["markdown"].(string), "wait") {
		t.Fatalf("lesson: %v", lesson)
	}
	q, isErr := call(t, session, "get_citizenship_question", map[string]any{"test_id": "sample", "index": 1})
	if isErr || q["reading"] == nil || q["correct_answer"] != nil {
		t.Fatalf("question: %v", q)
	}
	feedback, isErr := call(t, session, "check_citizenship_answer", map[string]any{"test_id": "sample", "index": 1, "answer": "Б"})
	if isErr || feedback["correct"] != true {
		t.Fatalf("feedback: %v", feedback)
	}
	_, isErr = call(t, session, "record_citizenship_progress", map[string]any{"material_id": "lesson", "index": 1, "status": "review", "note": "needs more practice"})
	if isErr {
		t.Fatal("record progress failed")
	}
	_, isErr = call(t, session, "save_vocabulary", map[string]any{"term": "правило", "kind": "word", "translation": "rule"})
	if isErr {
		t.Fatal("shared vocabulary failed")
	}
	plan, isErr := call(t, citizenshipConnect(t, dataDir), "get_citizenship_plan", nil)
	if isErr || plan["sections"].([]any)[0].(map[string]any)["status"] != "review" {
		t.Fatalf("reloaded plan: %v", plan)
	}
}

func TestCitizenshipMockOverHTTP(t *testing.T) {
	session := citizenshipConnect(t, citizenshipData(t))
	exam, isErr := call(t, session, "get_citizenship_exam", map[string]any{"test_id": "sample"})
	if isErr || len(exam["questions"].([]any)) != 20 || exam["answers"] != nil {
		t.Fatalf("mock: %v", exam)
	}
	answers := make([]map[string]any, 20)
	for i := range answers {
		answers[i] = map[string]any{"index": i + 1, "answer": "Б"}
	}
	grade, isErr := call(t, session, "grade_citizenship_exam", map[string]any{"test_id": "sample", "answers": answers})
	if isErr || grade["score"] != float64(20) || grade["passed"] != true {
		t.Fatalf("grade: %v", grade)
	}
	plan, isErr := call(t, session, "get_citizenship_plan", nil)
	if isErr || plan["latest_exams"].([]any)[0].(map[string]any)["score"] != float64(20) {
		t.Fatalf("saved score: %v", plan)
	}
	_, isErr = call(t, session, "get_citizenship_lesson", map[string]any{"material_id": "sample", "index": 1})
	if !isErr {
		t.Fatal("lesson must not retrieve private exam file")
	}
	_, isErr = call(t, session, "grade_citizenship_exam", map[string]any{"test_id": "sample", "answers": answers[:1]})
	if !isErr {
		t.Fatal("partial mock must not be graded")
	}
}

func TestModesAndMissingCitizenshipAssets(t *testing.T) {
	if _, err := NewWithMode(t.TempDir(), "unknown"); err == nil {
		t.Error("unknown mode accepted")
	}
	session := citizenshipConnect(t, t.TempDir())
	out, isErr := call(t, session, "list_citizenship_materials", nil)
	if !isErr || !strings.Contains(out["error"].(string), "bgtutor-assets") {
		t.Fatalf("missing assets: %v", out)
	}
}
