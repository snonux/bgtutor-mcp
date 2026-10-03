package bgtutor

import "strings"

// CitizenshipQuestionPayload is one question plus its reading passage when needed.
type CitizenshipQuestionPayload struct {
	TestID    string              `json:"test_id"`
	Title     string              `json:"title"`
	Source    string              `json:"source"`
	Reading   string              `json:"reading,omitempty"`
	Question  CitizenshipQuestion `json:"question"`
	Position  string              `json:"position"`
	IsLast    bool                `json:"is_last"`
	NextIndex *int                `json:"next_index"`
}

// CitizenshipExam is a complete mock paper, with no answer keys.
// Duration is guidance for the learner's timer; the stateless server does not time sessions.
type CitizenshipExam struct {
	TestID          string                `json:"test_id"`
	Title           string                `json:"title"`
	Source          string                `json:"source"`
	DurationMinutes int                   `json:"duration_minutes"`
	PassMark        int                   `json:"pass_mark"`
	Reading         string                `json:"reading"`
	Questions       []CitizenshipQuestion `json:"questions"`
}

// CitizenshipAnswer is a submitted letter; an empty letter means unanswered.
type CitizenshipAnswer struct {
	Index  int    `json:"index"`
	Answer string `json:"answer"`
}

// CitizenshipFeedback explains one graded answer.
type CitizenshipFeedback struct {
	Index         int    `json:"index"`
	Topic         string `json:"topic"`
	Submitted     string `json:"submitted"`
	Correct       bool   `json:"correct"`
	CorrectAnswer string `json:"correct_answer"`
	Explanation   string `json:"explanation,omitempty"`
}

// CitizenshipGrade reports a mock score and feedback for subsequent study.
type CitizenshipGrade struct {
	TestID   string                `json:"test_id"`
	Score    int                   `json:"score"`
	Total    int                   `json:"total"`
	PassMark int                   `json:"pass_mark"`
	Passed   bool                  `json:"passed"`
	Feedback []CitizenshipFeedback `json:"feedback"`
}

// Question returns one question, keeping grading data private.
func (l *CitizenshipLibrary) Question(id string, index int) (*CitizenshipQuestionPayload, error) {
	test, err := l.Test(id)
	if err != nil {
		return nil, err
	}
	if index < 1 || index > len(test.Questions) {
		return nil, userErrorf("Question index %d is out of range. Valid indexes are 1 to %d.", index, len(test.Questions))
	}
	out := &CitizenshipQuestionPayload{TestID: id, Title: test.Title, Source: test.Source, Question: test.Questions[index-1], Position: positionMarker(index, len(test.Questions)), IsLast: index == len(test.Questions)}
	if index <= 5 {
		out.Reading = test.Reading
	}
	if !out.IsLast {
		next := index + 1
		out.NextIndex = &next
	}
	return out, nil
}

// Exam returns a full paper for a timed mock, without keys or explanations.
func (l *CitizenshipLibrary) Exam(id string) (*CitizenshipExam, error) {
	test, err := l.Test(id)
	if err != nil {
		return nil, err
	}
	return &CitizenshipExam{TestID: id, Title: test.Title, Source: test.Source, DurationMinutes: 60, PassMark: 12, Reading: test.Reading, Questions: test.Questions}, nil
}

// CheckAnswer provides immediate feedback for a practice question.
func (l *CitizenshipLibrary) CheckAnswer(id string, answer CitizenshipAnswer) (*CitizenshipFeedback, error) {
	test, err := l.Test(id)
	if err != nil {
		return nil, err
	}
	return citizenshipFeedback(test, answer)
}

// GradeExam requires a complete answer sheet, rejecting duplicates and bad letters.
// Blank answers are allowed and score zero. No session state is kept on the server.
func (l *CitizenshipLibrary) GradeExam(id string, answers []CitizenshipAnswer) (*CitizenshipGrade, error) {
	test, err := l.Test(id)
	if err != nil {
		return nil, err
	}
	if len(answers) != len(test.Questions) {
		return nil, userErrorf("Submit exactly %d answers; use an empty answer for an unanswered question.", len(test.Questions))
	}
	out := &CitizenshipGrade{TestID: id, Total: len(test.Questions), PassMark: 12, Feedback: make([]CitizenshipFeedback, len(test.Questions))}
	seen := make(map[int]bool, len(answers))
	for _, answer := range answers {
		if seen[answer.Index] {
			return nil, userErrorf("Duplicate answer for question %d.", answer.Index)
		}
		seen[answer.Index] = true
		feedback, err := citizenshipFeedback(test, answer)
		if err != nil {
			return nil, err
		}
		out.Feedback[answer.Index-1] = *feedback
		if feedback.Correct {
			out.Score++
		}
	}
	out.Passed = out.Score >= out.PassMark
	return out, nil
}

func citizenshipFeedback(test *CitizenshipTest, answer CitizenshipAnswer) (*CitizenshipFeedback, error) {
	if answer.Index < 1 || answer.Index > len(test.Questions) {
		return nil, userErrorf("Question index must be 1 to %d.", len(test.Questions))
	}
	submitted := strings.ToUpper(strings.TrimSpace(answer.Answer))
	if submitted != "" && !validCitizenshipLetter(submitted) {
		return nil, userErrorf("Answer must be one Bulgarian letter: А, Б, В or Г (or empty for unanswered).")
	}
	key := test.Answers[answer.Index-1]
	return &CitizenshipFeedback{Index: answer.Index, Topic: test.Questions[answer.Index-1].Topic, Submitted: submitted, Correct: submitted == key.Answer, CorrectAnswer: key.Answer, Explanation: key.Explanation}, nil
}
