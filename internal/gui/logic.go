package gui

import (
	"errors"
	"fmt"
	"strings"

	"bdp-sync/internal/config"
)

func guiCommandArgs(action, configPath, selectedTask string, all bool) ([]string, error) {
	if strings.TrimSpace(configPath) == "" {
		configPath = config.DefaultPath
	}

	switch action {
	case "doctor":
		return []string{"doctor", "--config", configPath}, nil
	case "dry-run", "update", "sync":
		args := []string{action, "--config", configPath}
		if all {
			return append(args, "--all"), nil
		}
		if strings.TrimSpace(selectedTask) == "" {
			return nil, errors.New("select a task or enable all tasks")
		}
		return append(args, selectedTask), nil
	default:
		return nil, fmt.Errorf("unknown action %q", action)
	}
}

type terminalLogBuffer struct {
	runes     []rune
	lineStart int
	pendingCR bool
	inEsc     bool
	inCSI     bool
}

type taskFormValues struct {
	Name         string
	Local        string
	Remote       string
	ExcludesText string
}

func taskLabels(tasks []config.Task) []string {
	labels := make([]string, 0, len(tasks))
	for i, task := range tasks {
		name := strings.TrimSpace(task.Name)
		if name == "" {
			name = "(unnamed task)"
		}
		labels = append(labels, fmt.Sprintf("%d. %s", i+1, name))
	}
	return labels
}

func selectedTaskIndex(options []string, selected string) int {
	for i, option := range options {
		if option == selected {
			return i
		}
	}
	return -1
}

func taskValues(task config.Task) taskFormValues {
	return taskFormValues{
		Name:         task.Name,
		Local:        task.Local,
		Remote:       task.Remote,
		ExcludesText: strings.Join(task.Excludes, "\n"),
	}
}

func applyTaskValues(cfg *config.Config, index int, values taskFormValues) {
	if index < 0 || index >= len(cfg.Tasks) {
		return
	}
	cfg.Tasks[index] = config.Task{
		Name:     strings.TrimSpace(values.Name),
		Local:    strings.TrimSpace(values.Local),
		Remote:   strings.TrimSpace(values.Remote),
		Excludes: splitLines(values.ExcludesText),
	}
}

func appendTask(cfg *config.Config) int {
	cfg.Tasks = append(cfg.Tasks, config.Task{})
	return len(cfg.Tasks) - 1
}

func deleteTask(cfg *config.Config, index int) int {
	if index < 0 || index >= len(cfg.Tasks) {
		return -1
	}
	cfg.Tasks = append(cfg.Tasks[:index], cfg.Tasks[index+1:]...)
	if len(cfg.Tasks) == 0 {
		return -1
	}
	if index >= len(cfg.Tasks) {
		return len(cfg.Tasks) - 1
	}
	return index
}

func (b *terminalLogBuffer) Append(text string) string {
	for _, r := range text {
		b.appendRune(r)
	}
	return string(b.runes)
}

func (b *terminalLogBuffer) Clear() {
	*b = terminalLogBuffer{}
}

func (b *terminalLogBuffer) appendRune(r rune) {
	if b.inEsc {
		b.consumeEscape(r)
		return
	}
	if r == '\x1b' {
		b.inEsc = true
		b.inCSI = false
		return
	}

	if b.pendingCR {
		if r == '\n' {
			b.pendingCR = false
			b.appendNewline()
			return
		}
		b.pendingCR = false
		b.truncateCurrentLine()
	}

	switch r {
	case '\r':
		b.pendingCR = true
	case '\n':
		b.appendNewline()
	case '\b':
		b.backspace()
	default:
		b.runes = append(b.runes, r)
	}
}

func (b *terminalLogBuffer) consumeEscape(r rune) {
	if b.inCSI {
		if r >= '@' && r <= '~' {
			b.inEsc = false
			b.inCSI = false
		}
		return
	}
	if r == '[' {
		b.inCSI = true
		return
	}
	b.inEsc = false
}

func (b *terminalLogBuffer) appendNewline() {
	b.runes = append(b.runes, '\n')
	b.lineStart = len(b.runes)
}

func (b *terminalLogBuffer) truncateCurrentLine() {
	if b.lineStart < 0 || b.lineStart > len(b.runes) {
		b.lineStart = len(b.runes)
		return
	}
	b.runes = b.runes[:b.lineStart]
}

func (b *terminalLogBuffer) backspace() {
	if len(b.runes) > b.lineStart {
		b.runes = b.runes[:len(b.runes)-1]
	}
}
