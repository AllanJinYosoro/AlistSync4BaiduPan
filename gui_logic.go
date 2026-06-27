package main

import (
	"errors"
	"fmt"
	"strings"
)

func taskNames(cfg Config) []string {
	names := make([]string, 0, len(cfg.Tasks))
	for _, task := range cfg.Tasks {
		if task.Name != "" {
			names = append(names, task.Name)
		}
	}
	return names
}

func guiCommandArgs(action, configPath, selectedTask string, all bool) ([]string, error) {
	if strings.TrimSpace(configPath) == "" {
		configPath = defaultConfigPath
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
