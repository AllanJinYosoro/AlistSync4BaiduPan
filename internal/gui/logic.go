package gui

import (
	"errors"
	"fmt"
	"strconv"
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
	csiParams []rune

	inRcloneProgressBlock bool
	rcloneProgressStart   int
	rcloneProgressRestart bool
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
	if b.pendingCR {
		if r == '\n' {
			b.pendingCR = false
			b.appendNewline()
			return
		}
		b.pendingCR = false
		b.truncateCurrentLine()
	}
	if r == '\x1b' {
		b.inEsc = true
		b.inCSI = false
		return
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
			b.applyCSI(r)
			b.inEsc = false
			b.inCSI = false
			return
		}
		b.csiParams = append(b.csiParams, r)
		return
	}
	if r == '[' {
		b.inCSI = true
		b.csiParams = b.csiParams[:0]
		return
	}
	b.inEsc = false
}

func (b *terminalLogBuffer) applyCSI(final rune) {
	switch final {
	case 'A', 'F':
		b.moveUp(b.firstCSIParam(1))
		b.truncateCurrentLine()
	case 'K':
		b.truncateCurrentLine()
	}
}

func (b *terminalLogBuffer) firstCSIParam(defaultValue int) int {
	params := string(b.csiParams)
	if params == "" || strings.HasPrefix(params, "?") {
		return defaultValue
	}
	if i := strings.IndexByte(params, ';'); i >= 0 {
		params = params[:i]
	}
	if params == "" {
		return defaultValue
	}
	n, err := strconv.Atoi(params)
	if err != nil || n < 1 {
		return defaultValue
	}
	return n
}

func (b *terminalLogBuffer) moveUp(lines int) {
	for ; lines > 0 && b.lineStart > 0; lines-- {
		i := b.lineStart - 2
		for i >= 0 && b.runes[i] != '\n' {
			i--
		}
		b.lineStart = i + 1
	}
}

func (b *terminalLogBuffer) appendNewline() {
	start := b.lineStart
	b.runes = append(b.runes, '\n')
	b.lineStart = len(b.runes)
	b.finishLine(start)
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

func (b *terminalLogBuffer) finishLine(start int) {
	if start < 0 || start >= len(b.runes) {
		return
	}
	line := string(b.runes[start : len(b.runes)-1])
	trimmed := strings.TrimSpace(line)
	if isRcloneProgressStart(trimmed) && (!b.inRcloneProgressBlock || b.rcloneProgressRestart) {
		if b.inRcloneProgressBlock && b.rcloneProgressStart < start {
			tail := append([]rune(nil), b.runes[start:]...)
			b.runes = append(b.runes[:b.rcloneProgressStart], tail...)
			b.lineStart = len(b.runes)
			start = b.rcloneProgressStart
		}
		b.inRcloneProgressBlock = true
		b.rcloneProgressStart = start
		b.rcloneProgressRestart = false
		return
	}
	if !b.inRcloneProgressBlock {
		return
	}
	if trimmed == "" || line != trimmed || isRcloneProgressLine(trimmed) {
		if trimmed == "" || isRcloneProgressTail(trimmed) {
			b.rcloneProgressRestart = true
		}
		return
	}
	b.inRcloneProgressBlock = false
	b.rcloneProgressRestart = false
}

func isRcloneProgressStart(line string) bool {
	return line == "Errors:" || line == "Transferred:"
}

func isRcloneProgressLine(line string) bool {
	switch line {
	case "Errors:", "Checks:", "Transferred:", "Elapsed time:", "Checking:", "Transferring:", "Deleting:", "Renaming:":
		return true
	}
	return strings.HasPrefix(line, "* ") || strings.Contains(line, " / ") || strings.Contains(line, " B/s")
}

func isRcloneProgressTail(line string) bool {
	switch line {
	case "Checking:", "Transferring:", "Deleting:", "Renaming:":
		return true
	}
	return strings.HasPrefix(line, "* ")
}
