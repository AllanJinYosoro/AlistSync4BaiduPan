package main

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

const maxNameProblems = 20

type nameProblem struct {
	Task string
	Path string
	Why  string
}

func findUnsupportedUploadNames(tasks []Task, limit int) ([]nameProblem, error) {
	var problems []nameProblem
	for _, task := range tasks {
		root := toNativePath(task.Local)
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if why := unsupportedUploadNameReason(d.Name()); why != "" {
				problems = append(problems, nameProblem{
					Task: task.Name,
					Path: path,
					Why:  why,
				})
				if len(problems) >= limit {
					return filepath.SkipAll
				}
			}
			return nil
		})
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return problems, fmt.Errorf("scan task %q: %w", task.Name, err)
		}
		if len(problems) >= limit {
			break
		}
	}
	return problems, nil
}

func unsupportedUploadNameReason(name string) string {
	if strings.ContainsRune(name, '\uFF1A') {
		return "contains fullwidth colon U+FF1A; Baidu Netdisk through AList WebDAV can reject it with HTTP 405"
	}
	return ""
}

func formatNameProblems(problems []nameProblem) string {
	var b strings.Builder
	fmt.Fprintf(&b, "found %d file name(s) likely to fail on Baidu WebDAV upload:\n", len(problems))
	for _, p := range problems {
		fmt.Fprintf(&b, "- [%s] %s (%s)\n", p.Task, p.Path, p.Why)
	}
	b.WriteString("rename those files, for example replacing the fullwidth colon with a hyphen, then rerun the command")
	return strings.TrimRight(b.String(), "\n")
}
