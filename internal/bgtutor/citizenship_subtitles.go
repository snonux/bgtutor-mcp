package bgtutor

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var citizenshipTimestampPattern = regexp.MustCompile(`^\d{2}:\d{2}:\d{2},\d{3} --> \d{2}:\d{2}:\d{2},\d{3}$`)

func citizenshipDocumentSections(path, content string) (string, []string, error) {
	if !strings.HasSuffix(path, ".srt") {
		title, sections := citizenshipSections(content)
		return title, sections, nil
	}
	title := strings.TrimSuffix(filepath.Base(path), ".srt")
	content = strings.ReplaceAll(content, "\r\n", "\n")
	var sections []string
	for _, block := range strings.Split(strings.TrimSpace(content), "\n\n") {
		lines := strings.Split(strings.TrimSpace(block), "\n")
		if len(lines) < 3 {
			return "", nil, fmt.Errorf("subtitle cue needs an index, timestamp and caption")
		}
		index, err := strconv.Atoi(strings.TrimSpace(lines[0]))
		if err != nil || index != len(sections)+1 || !citizenshipTimestampPattern.MatchString(strings.TrimSpace(lines[1])) {
			return "", nil, fmt.Errorf("invalid subtitle index or timestamp in cue %d", len(sections)+1)
		}
		caption := strings.TrimSpace(strings.Join(lines[2:], "\n"))
		if caption == "" {
			return "", nil, fmt.Errorf("empty subtitle caption")
		}
		sections = append(sections, fmt.Sprintf("# %s — caption %d\n\n%s\n\n%s", title, index, lines[1], caption))
	}
	return title, sections, nil
}

func citizenshipMediaPath(path string) string {
	if strings.HasSuffix(path, ".srt") {
		return strings.TrimSuffix(path, ".srt") + ".mp4"
	}
	return ""
}
