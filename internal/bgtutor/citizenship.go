package bgtutor

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// CitizenshipLibrary reads the citizenship-test folder in bgtutor-assets.
// Files are reloaded on each call, just like podcast episodes.
type CitizenshipLibrary struct{ Dir string }

// CitizenshipMaterial describes a Markdown study document.
type CitizenshipMaterial struct {
	ID        string `json:"material_id"`
	Title     string `json:"title"`
	Sections  int    `json:"section_count"`
	MediaPath string `json:"media_path,omitempty"`
}

// CitizenshipTestSummary describes a practice or official sample test.
type CitizenshipTestSummary struct {
	ID            string `json:"test_id"`
	Title         string `json:"title"`
	Source        string `json:"source"`
	QuestionCount int    `json:"question_count"`
	Ready         bool   `json:"ready"`
	Problem       string `json:"not_ready_reason,omitempty"`
}

// CitizenshipCatalog lists study material and validated tests.
type CitizenshipCatalog struct {
	Materials []CitizenshipMaterial    `json:"materials"`
	Tests     []CitizenshipTestSummary `json:"tests"`
}

// CitizenshipLesson is one section of a study document, in its original language.
type CitizenshipLesson struct {
	MaterialID string `json:"material_id"`
	Title      string `json:"title"`
	Index      int    `json:"index"`
	Markdown   string `json:"markdown"`
	Position   string `json:"position"`
	IsLast     bool   `json:"is_last"`
	NextIndex  *int   `json:"next_index"`
	MediaPath  string `json:"media_path,omitempty"`
}

// CitizenshipTest is a prepared exam, including private grading data.
// Only Question and Exam payloads are sent before the learner submits answers.
type CitizenshipTest struct {
	FormatVersion int                   `json:"format_version"`
	Title         string                `json:"title"`
	Source        string                `json:"source"`
	Reading       string                `json:"reading"`
	Questions     []CitizenshipQuestion `json:"questions"`
	Answers       []CitizenshipKey      `json:"answers"`
}

// CitizenshipQuestion contains no answer or explanation.
type CitizenshipQuestion struct {
	Index   int                 `json:"index"`
	Topic   string              `json:"topic"`
	Prompt  string              `json:"prompt"`
	Choices []CitizenshipChoice `json:"choices"`
}

// CitizenshipChoice is one of the four Bulgarian lettered options.
type CitizenshipChoice struct {
	Letter string `json:"letter"`
	Text   string `json:"text"`
}

// CitizenshipKey is grading data stored separately from question payloads.
type CitizenshipKey struct {
	Index       int    `json:"index"`
	Answer      string `json:"answer"`
	Explanation string `json:"explanation,omitempty"`
}

// NewCitizenshipLibrary returns a library rooted at citizenshipDir.
func NewCitizenshipLibrary(citizenshipDir string) *CitizenshipLibrary {
	return &CitizenshipLibrary{Dir: citizenshipDir}
}

// Catalog lists study documents and tests, including invalid tests with a reason.
func (l *CitizenshipLibrary) Catalog() (*CitizenshipCatalog, error) {
	files, err := l.files()
	if err != nil {
		return nil, err
	}
	out := &CitizenshipCatalog{Materials: []CitizenshipMaterial{}, Tests: []CitizenshipTestSummary{}}
	for _, path := range files {
		id := strings.TrimSuffix(path, filepath.Ext(path))
		if isCitizenshipTest(path) {
			test, err := l.Test(id)
			item := CitizenshipTestSummary{ID: id, Title: id}
			if err != nil {
				item.Problem = err.Error()
			} else {
				item.Title, item.Source, item.QuestionCount, item.Ready = test.Title, test.Source, len(test.Questions), true
			}
			out.Tests = append(out.Tests, item)
		} else {
			data, err := os.ReadFile(filepath.Join(l.Dir, filepath.FromSlash(path)))
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", path, err)
			}
			title, sections, err := citizenshipDocumentSections(path, string(data))
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", path, err)
			}
			out.Materials = append(out.Materials, CitizenshipMaterial{ID: id, Title: title, Sections: len(sections), MediaPath: citizenshipMediaPath(path)})
		}
	}
	return out, nil
}

// Lesson returns one section without exposing practice-test answer keys.
func (l *CitizenshipLibrary) Lesson(id string, index int) (*CitizenshipLesson, error) {
	path, err := l.find(id, false)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read lesson %q: %w", id, err)
	}
	title, sections, err := citizenshipDocumentSections(path, string(data))
	if err != nil {
		return nil, fmt.Errorf("read lesson %q: %w", id, err)
	}
	if index < 1 || index > len(sections) {
		return nil, userErrorf("Section index %d is out of range. Valid indexes are 1 to %d.", index, len(sections))
	}
	out := &CitizenshipLesson{MaterialID: id, Title: title, Index: index, Markdown: sections[index-1], Position: positionMarker(index, len(sections)), IsLast: index == len(sections)}
	if !out.IsLast {
		next := index + 1
		out.NextIndex = &next
	}
	out.MediaPath = citizenshipMediaPath(id + filepath.Ext(path))
	return out, nil
}

// Test loads and validates one practice Markdown or prepared JSON sample exam.
func (l *CitizenshipLibrary) Test(id string) (*CitizenshipTest, error) {
	path, err := l.find(id, true)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read test %q: %w", id, err)
	}
	var test *CitizenshipTest
	if strings.HasSuffix(path, ".md") {
		test, err = parseCitizenshipMarkdown(string(data))
	} else {
		test = &CitizenshipTest{}
		err = json.Unmarshal(data, test)
	}
	if err != nil {
		return nil, fmt.Errorf("test %q: %w", id, err)
	}
	if err := validateCitizenshipTest(test); err != nil {
		return nil, fmt.Errorf("test %q: %w", id, err)
	}
	return test, nil
}

func (l *CitizenshipLibrary) files() ([]string, error) {
	var files []string
	root, err := filepath.EvalSymlinks(l.Dir)
	if err != nil {
		return nil, fmt.Errorf("read citizenship library %q (use --data-dir pointing to bgtutor-assets): %w", l.Dir, err)
	}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type().IsRegular() && (strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".json") || strings.HasSuffix(path, ".srt")) {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read citizenship library %q (use --data-dir pointing to bgtutor-assets): %w", l.Dir, err)
	}
	return files, nil
}

// find resolves only catalogued files. This blocks traversal and prevents a
// lesson request from reading a test's answers through the document tool.
func (l *CitizenshipLibrary) find(id string, test bool) (string, error) {
	if !fs.ValidPath(id) || strings.Contains(id, "\\") {
		return "", userErrorf("Invalid citizenship material id %q.", id)
	}
	files, err := l.files()
	if err != nil {
		return "", err
	}
	for _, path := range files {
		if strings.TrimSuffix(path, filepath.Ext(path)) == id && isCitizenshipTest(path) == test {
			return filepath.Join(l.Dir, filepath.FromSlash(path)), nil
		}
	}
	return "", userErrorf("Unknown citizenship material %q. Use an id from list_citizenship_materials.", id)
}

func isCitizenshipTest(path string) bool {
	return strings.HasSuffix(path, ".json") || strings.HasPrefix(path, "practice/test-") && strings.HasSuffix(path, ".md")
}
