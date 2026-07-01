package gui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"bdp-sync/internal/config"
)

func guiCommandArgs(action, configPath, selectedTask string, all bool, scopedPath string) ([]string, error) {
	if strings.TrimSpace(configPath) == "" {
		configPath = config.DefaultPath
	}
	scopedPath = strings.TrimSpace(scopedPath)

	switch action {
	case "doctor":
		return []string{"doctor", "--config", configPath}, nil
	case "dry-run", "update", "sync":
		args := []string{action, "--config", configPath}
		if all {
			if scopedPath != "" {
				return nil, errors.New("select one task to use Specific")
			}
			return append(args, "--all"), nil
		}
		if strings.TrimSpace(selectedTask) == "" {
			return nil, errors.New("select a task or enable all tasks")
		}
		if scopedPath != "" {
			args = append(args, "--path", scopedPath)
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
	rcloneProgressEnd     int
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
	b.clampRcloneProgressRange()
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
	if split := embeddedRcloneProgressStartIndex(line); split > 0 {
		b.runes = append(b.runes[:start], b.runes[start+split:]...)
		b.lineStart = len(b.runes)
		line = string(b.runes[start : len(b.runes)-1])
	}

	trimmed := strings.TrimSpace(line)
	if isRcloneProgressStart(trimmed) && (!b.inRcloneProgressBlock || b.rcloneProgressRestart) {
		b.startRcloneProgressBlock(start)
		return
	}
	if !b.inRcloneProgressBlock {
		return
	}
	if trimmed == "" || hasLeadingWhitespace(line) || isRcloneProgressLine(trimmed) {
		b.rcloneProgressEnd = len(b.runes)
		if trimmed == "" || isRcloneProgressTail(trimmed) {
			b.rcloneProgressRestart = true
		}
		return
	}
	b.inRcloneProgressBlock = false
}

func (b *terminalLogBuffer) startRcloneProgressBlock(start int) {
	if b.rcloneProgressStart < start && b.rcloneProgressEnd > b.rcloneProgressStart {
		removed := b.rcloneProgressEnd - b.rcloneProgressStart
		tail := append([]rune(nil), b.runes[b.rcloneProgressEnd:]...)
		b.runes = append(b.runes[:b.rcloneProgressStart], tail...)
		start -= removed
		if start < b.rcloneProgressStart {
			start = b.rcloneProgressStart
		}
		b.lineStart = len(b.runes)
	}
	b.inRcloneProgressBlock = true
	b.rcloneProgressStart = start
	b.rcloneProgressEnd = len(b.runes)
	b.rcloneProgressRestart = false
}

func (b *terminalLogBuffer) clampRcloneProgressRange() {
	if b.rcloneProgressStart > len(b.runes) {
		b.rcloneProgressStart = len(b.runes)
	}
	if b.rcloneProgressEnd > len(b.runes) {
		b.rcloneProgressEnd = len(b.runes)
	}
	if b.rcloneProgressEnd < b.rcloneProgressStart {
		b.rcloneProgressEnd = b.rcloneProgressStart
	}
}

func hasLeadingWhitespace(line string) bool {
	return strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
}

func embeddedRcloneProgressStartIndex(line string) int {
	for _, marker := range []string{"Errors:", "Transferred:"} {
		if i := strings.Index(line, marker); i > 0 && isRcloneProgressLine(strings.TrimSpace(line[:i])) {
			return len([]rune(line[:i]))
		}
	}
	return -1
}

func isRcloneProgressStart(line string) bool {
	return strings.HasPrefix(line, "Errors:") || strings.HasPrefix(line, "Transferred:")
}

func isRcloneProgressLine(line string) bool {
	for _, prefix := range []string{"Errors:", "Checks:", "Transferred:", "Elapsed time:", "Checking:", "Transferring:", "Deleting:", "Renaming:"} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return strings.HasPrefix(line, "* ") || strings.Contains(line, " / ") || strings.Contains(line, " B/s")
}

func isRcloneProgressTail(line string) bool {
	for _, prefix := range []string{"Checking:", "Transferring:", "Deleting:", "Renaming:"} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return strings.HasPrefix(line, "* ")
}
