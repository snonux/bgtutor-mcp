package bgtutor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var citizenshipQuestionPattern = regexp.MustCompile(`^\*\*(\d+)\.\s*(.*?)\*\*$`)
var citizenshipExplanationPattern = regexp.MustCompile(`^(\d+)\.\s+(.*)$`)

func citizenshipSections(markdown string) (string, []string) {
	var title string
	var sections []string
	var lines []string
	flush := func() {
		if text := strings.TrimSpace(strings.Join(lines, "\n")); text != "" {
			sections = append(sections, text)
		}
		lines = nil
	}
	for _, line := range strings.Split(markdown, "\n") {
		if strings.HasPrefix(line, "# ") && title == "" {
			title = strings.TrimPrefix(line, "# ")
		}
		if strings.HasPrefix(line, "## ") {
			flush()
		}
		lines = append(lines, line)
	}
	flush()
	return title, sections
}

func parseCitizenshipMarkdown(markdown string) (*CitizenshipTest, error) {
	body, key, found := strings.Cut(markdown, "## Отговори и обяснения")
	if !found {
		return nil, fmt.Errorf("missing answer key heading")
	}
	title, _ := citizenshipSections(body)
	test := &CitizenshipTest{FormatVersion: 1, Title: title, Source: "original practice; reading text is invented"}
	reading, questions, err := parseCitizenshipQuestions(body)
	if err != nil {
		return nil, err
	}
	test.Reading, test.Questions = reading, questions
	test.Answers, err = parseCitizenshipKey(key)
	return test, err
}

func parseCitizenshipQuestions(body string) (string, []CitizenshipQuestion, error) {
	var questions []CitizenshipQuestion
	var heading string
	var reading []string
	var current *CitizenshipQuestion
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "## ") {
			heading = strings.TrimPrefix(line, "## ")
			continue
		}
		match := citizenshipQuestionPattern.FindStringSubmatch(line)
		if match != nil {
			index, err := strconv.Atoi(match[1])
			if err != nil {
				return "", nil, err
			}
			prompt := heading
			if match[2] != "" {
				prompt += "\n" + match[2]
			}
			questions = append(questions, CitizenshipQuestion{Index: index, Topic: citizenshipTopic(index), Prompt: prompt})
			current = &questions[len(questions)-1]
		} else if current != nil {
			if letter, text, ok := strings.Cut(line, ") "); ok && validCitizenshipLetter(letter) {
				current.Choices = append(current.Choices, CitizenshipChoice{Letter: letter, Text: text})
			}
		} else if heading != "" {
			reading = append(reading, line)
		}
	}
	return strings.TrimSpace(strings.Join(reading, "\n")), questions, nil
}

func parseCitizenshipKey(markdown string) ([]CitizenshipKey, error) {
	var answers []CitizenshipKey
	current := -1
	for _, line := range strings.Split(markdown, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "|") {
			cells := strings.Split(strings.Trim(line, "|"), "|")
			if len(cells) != 20 || !validCitizenshipLetter(strings.TrimSpace(cells[0])) {
				continue
			}
			if len(answers) > 0 {
				return nil, fmt.Errorf("duplicate answer key row")
			}
			for i, cell := range cells {
				answers = append(answers, CitizenshipKey{Index: i + 1, Answer: strings.TrimSpace(cell)})
			}
		} else if match := citizenshipExplanationPattern.FindStringSubmatch(line); match != nil {
			index, err := strconv.Atoi(match[1])
			if err != nil || index < 1 || index > len(answers) {
				return nil, fmt.Errorf("invalid explanation index %q", match[1])
			}
			current = index - 1
			answers[current].Explanation = match[2]
		} else if current >= 0 && line != "" {
			answers[current].Explanation += "\n" + line
		}
	}
	return answers, nil
}

func validateCitizenshipTest(test *CitizenshipTest) error {
	if test.FormatVersion != 1 {
		return fmt.Errorf("format_version must be 1")
	}
	if strings.TrimSpace(test.Title) == "" || strings.TrimSpace(test.Source) == "" || strings.TrimSpace(test.Reading) == "" {
		return fmt.Errorf("title, source and reading are required")
	}
	if len(test.Questions) != 20 || len(test.Answers) != 20 {
		return fmt.Errorf("expected 20 questions and 20 answers")
	}
	for i, q := range test.Questions {
		if q.Index != i+1 || strings.TrimSpace(q.Prompt) == "" || strings.TrimSpace(q.Topic) == "" || len(q.Choices) != 4 {
			return fmt.Errorf("question %d needs a sequential index, topic, prompt and four choices", i+1)
		}
		for j, choice := range q.Choices {
			if choice.Letter != []string{"А", "Б", "В", "Г"}[j] || strings.TrimSpace(choice.Text) == "" {
				return fmt.Errorf("question %d has an invalid choice", i+1)
			}
		}
		if test.Answers[i].Index != i+1 || !validCitizenshipLetter(test.Answers[i].Answer) {
			return fmt.Errorf("invalid answer key for question %d", i+1)
		}
	}
	return nil
}

func citizenshipTopic(index int) string {
	switch {
	case index <= 5:
		return "reading"
	case index <= 8:
		return "grammar"
	case index <= 11:
		return "synonyms"
	case index <= 14:
		return "antonyms"
	case index <= 16:
		return "idioms"
	case index <= 19:
		return "spelling"
	default:
		return "punctuation"
	}
}

func validCitizenshipLetter(letter string) bool {
	return letter == "А" || letter == "Б" || letter == "В" || letter == "Г"
}
