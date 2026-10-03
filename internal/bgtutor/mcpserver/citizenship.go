package mcpserver

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/snonux/bgtutor-mcp/internal"
	"github.com/snonux/bgtutor-mcp/internal/bgtutor"
)

// CitizenshipInstructions directs the voice tutor through the complete citizenship curriculum.
const CitizenshipInstructions = `You are a Bulgarian citizenship preparation tutor. The assets cover the
Bulgarian language exam and the separate citizenship application and interview.
Use Bulgarian for practice; explain in English when helpful. Adapt to the learner.

At the beginning call get_citizenship_plan and list_citizenship_materials.
Resume review and unseen sections and use the latest exam's weak_topics to
prioritise drills. Offer a diagnostic if there is no exam result yet.
Cover every relevant study section, the guide, official-materials and experiences.
The sources document is provenance, not a list to memorise. Preserve source dates,
distinguish candidate reports from official facts, and say when the guide marks
logistics as uncertain. Do not invent current legal requirements or exam dates.

Guided study is a live classroom conversation, not a readout. Begin with a
warm-up question to check prior knowledge. Follow the chapter's opening,
examples, guided practice, answer notes, hints and independent-use checks.
Introduce one example, invite the learner to notice the rule, and explain it
briefly. Never read private tutor answer notes aloud before an attempt. Ask
one question, wait and respond to that specific answer. Let the learner finish;
give the smallest useful hint, let them repair the answer, then ask a new
transfer question. Fade support gradually and personalise examples using facts
the learner supplies. Check both understanding and independent use, revisit
mistakes later and keep each class around 15–25 minutes unless they prefer
otherwise. Written contrasts that sound identical must be shown or spelled.

get_citizenship_lesson retrieves one section. Teach a small part,
then ask ONE question and wait. Drill all rules and examples in the section:
recall, explain contrasts, fill gaps, correct mistakes and make new sentences.
Generated exercises are tutor practice, not official questions. Evaluate open
answers against the retrieved section, explaining uncertainty rather than
claiming an objective score. After testing, record_citizenship_progress with
review or mastered and a short note about mistakes; never mark mastery simply
because material was read. Recheck mastery later. Offer save_vocabulary for new
words, phrases and rules; list_vocabulary for review; delete only on request.

Question practice: get_citizenship_question returns a single numbered question
without a key. Read its passage/table for reading questions and preserve the
Bulgarian wording and options (А, Б, В, Г). Wait for the answer before calling
check_citizenship_answer; give feedback and drill the underlying weak rule.
Translate the learner's spoken option unambiguously to a Bulgarian letter.

Listening: video subtitle documents provide one caption and timestamp per
section plus a media_path relative to citizenship-test. Use the corresponding
video if the learner can open it, or read the caption naturally in Bulgarian.
Ask for a summary, repeat slowly, and check comprehension before showing the
written caption. Some captions adapt printed text; use the official exam
passage and table when grading reading questions. Video source code and the
video production README are not learning targets.

Interview: use study/interview and the guide to practise personal questions in
Bulgarian about motivation, residence, work, family and daily life. Ask follow-up
questions, correct language gently, and never invent the learner's personal facts.
The separate interview is not part of the written sample papers.

Mocks: get_citizenship_exam gives a full 20-question paper without keys. Ask the
learner to start an external 60-minute timer (the server does not time sessions).
Collect all 20 answers, including an empty answer for blanks. Withhold hints,
translations, check_citizenship_answer and feedback until the learner submits
or ends the mock. Then call grade_citizenship_exam once; it saves the score and
weak topics for later sessions. Explain errors and return to the relevant study
sections. The 12/20 pass mark describes these practice papers, not a guarantee
of citizenship. Aim for consistent 17+ across fresh papers, not memorised keys.
All tools use explicit material/test ids and indexes; no active session is stored.`

type citizenshipTools struct {
	lib     *bgtutor.CitizenshipLibrary
	tracker *bgtutor.CitizenshipTracker
}

type citizenshipIDIn struct {
	TestID string `json:"test_id" jsonschema:"test_id from list_citizenship_materials"`
}
type citizenshipQuestionIn struct {
	TestID string `json:"test_id" jsonschema:"test_id from list_citizenship_materials"`
	Index  int    `json:"index" jsonschema:"1-based question index, 1 through 20"`
}
type citizenshipAnswerIn struct {
	TestID string `json:"test_id"`
	Index  int    `json:"index"`
	Answer string `json:"answer" jsonschema:"one Bulgarian letter А, Б, В or Г; empty means unanswered"`
}
type citizenshipGradeIn struct {
	TestID  string                      `json:"test_id"`
	Answers []bgtutor.CitizenshipAnswer `json:"answers" jsonschema:"exactly 20 distinct indexed answers, including blanks"`
}
type citizenshipLessonIn struct {
	MaterialID string `json:"material_id" jsonschema:"material_id from list_citizenship_materials"`
	Index      int    `json:"index" jsonschema:"1-based section index; start at 1"`
}
type citizenshipProgressIn struct {
	MaterialID string `json:"material_id"`
	Index      int    `json:"index"`
	Status     string `json:"status" jsonschema:"review or mastered; assess only after testing recall"`
	Note       string `json:"note,omitempty" jsonschema:"short note about weak points to revisit"`
}

// NewWithMode builds either the default podcast server or citizenship preparation.
// Invalid modes fail before the HTTP listener starts.
func NewWithMode(dataDir, mode string) (*mcp.Server, error) {
	switch mode {
	case "podcast":
		return New(dataDir), nil
	case "citizenship":
		return newCitizenshipServer(dataDir), nil
	default:
		return nil, fmt.Errorf("unknown mode %q; choose podcast or citizenship", mode)
	}
}

func newCitizenshipServer(dataDir string) *mcp.Server {
	lib := bgtutor.NewCitizenshipLibrary(filepath.Join(dataDir, "citizenship-test"))
	tracker := bgtutor.NewCitizenshipTracker(filepath.Join(dataDir, "citizenship-progress", "saved.json"), lib)
	t := &citizenshipTools{lib: lib, tracker: tracker}
	server := mcp.NewServer(&mcp.Implementation{Name: "bulgarian-citizenship-tutor", Title: "Bulgarian Citizenship Tutor", Version: internal.Version}, &mcp.ServerOptions{Instructions: CitizenshipInstructions})
	addCitizenshipStudyTools(server, t)
	addCitizenshipExamTools(server, t)
	addVocabularyTools(server, &tools{nb: bgtutor.NewNotebook(filepath.Join(dataDir, "vocabulary", "saved.json"), bgtutor.NewLibrary(filepath.Join(dataDir, "episodes")))})
	return server
}

func addCitizenshipStudyTools(s *mcp.Server, t *citizenshipTools) {
	mcp.AddTool(s, &mcp.Tool{Name: "list_citizenship_materials", Description: "List all citizenship study documents and validated practice/official sample tests. IDs are relative paths without extensions.", Annotations: readOnly()}, t.listMaterials)
	mcp.AddTool(s, &mcp.Tool{Name: "get_citizenship_plan", Description: "Get the curriculum, complete section coverage and saved mock scores/weak topics. Resume unseen/review sections; sources are provenance.", Annotations: readOnly()}, t.plan)
	mcp.AddTool(s, &mcp.Tool{Name: "get_citizenship_lesson", Description: "Retrieve one study/guide section for active recall drills. Does not expose exam files or their answer keys.", Annotations: readOnly()}, t.lesson)
	mcp.AddTool(s, &mcp.Tool{Name: "record_citizenship_progress", Description: "After testing recall, mark a section review or mastered and save weak points. Content changes invalidate old mastery.", Annotations: &mcp.ToolAnnotations{IdempotentHint: true}}, t.recordProgress)
}

func addCitizenshipExamTools(s *mcp.Server, t *citizenshipTools) {
	mcp.AddTool(s, &mcp.Tool{Name: "get_citizenship_question", Description: "Get ONE question, options and any needed reading passage, without the answer. For guided practice or one question at a time during a mock.", Annotations: readOnly()}, t.question)
	mcp.AddTool(s, &mcp.Tool{Name: "check_citizenship_answer", Description: "Grade one submitted practice answer against the supplied key. Reveals the answer; do not call during an unfinished mock.", Annotations: readOnly()}, t.checkAnswer)
	mcp.AddTool(s, &mcp.Tool{Name: "get_citizenship_exam", Description: "Get a full 20-question mock without keys. Suggest an external 60-minute timer; collect answers before grading.", Annotations: readOnly()}, t.exam)
	mcp.AddTool(s, &mcp.Tool{Name: "grade_citizenship_exam", Description: "Grade a complete mock only after submission; return score and feedback and save the latest result/weak topics for later sessions."}, t.grade)
}

func (t *citizenshipTools) listMaterials(_ context.Context, _ *mcp.CallToolRequest, _ listEpisodesIn) (*mcp.CallToolResult, *bgtutor.CitizenshipCatalog, error) {
	out, err := t.lib.Catalog()
	return nil, out, err
}
func (t *citizenshipTools) plan(_ context.Context, _ *mcp.CallToolRequest, _ listEpisodesIn) (*mcp.CallToolResult, *bgtutor.CitizenshipPlan, error) {
	out, err := t.tracker.Plan()
	return nil, out, err
}
func (t *citizenshipTools) lesson(_ context.Context, _ *mcp.CallToolRequest, in citizenshipLessonIn) (*mcp.CallToolResult, *bgtutor.CitizenshipLesson, error) {
	out, err := t.lib.Lesson(in.MaterialID, in.Index)
	return nil, out, err
}
func (t *citizenshipTools) recordProgress(_ context.Context, _ *mcp.CallToolRequest, in citizenshipProgressIn) (*mcp.CallToolResult, *bgtutor.CitizenshipSectionProgress, error) {
	out, err := t.tracker.Record(in.MaterialID, in.Index, in.Status, in.Note)
	return nil, out, err
}
func (t *citizenshipTools) question(_ context.Context, _ *mcp.CallToolRequest, in citizenshipQuestionIn) (*mcp.CallToolResult, *bgtutor.CitizenshipQuestionPayload, error) {
	out, err := t.lib.Question(in.TestID, in.Index)
	return nil, out, err
}
func (t *citizenshipTools) checkAnswer(_ context.Context, _ *mcp.CallToolRequest, in citizenshipAnswerIn) (*mcp.CallToolResult, *bgtutor.CitizenshipFeedback, error) {
	out, err := t.lib.CheckAnswer(in.TestID, bgtutor.CitizenshipAnswer{Index: in.Index, Answer: in.Answer})
	return nil, out, err
}
func (t *citizenshipTools) exam(_ context.Context, _ *mcp.CallToolRequest, in citizenshipIDIn) (*mcp.CallToolResult, *bgtutor.CitizenshipExam, error) {
	out, err := t.lib.Exam(in.TestID)
	return nil, out, err
}
func (t *citizenshipTools) grade(_ context.Context, _ *mcp.CallToolRequest, in citizenshipGradeIn) (*mcp.CallToolResult, *bgtutor.CitizenshipGrade, error) {
	out, err := t.lib.GradeExam(in.TestID, in.Answers)
	if err != nil {
		return nil, nil, err
	}
	if err := t.tracker.RecordExam(out); err != nil {
		return nil, nil, fmt.Errorf("save exam progress: %w", err)
	}
	return nil, out, nil
}
