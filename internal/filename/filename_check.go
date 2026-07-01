package filename

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"bdp-sync/internal/config"
)

const MaxProblems = 20

type Problem struct {
	Task string
	Path string
	Why  string
}

type ZeroByteFile struct {
	Task    string
	Path    string
	Exclude string
}

func FindUnsupportedUploadNames(tasks []config.Task, limit int) ([]Problem, error) {
	return FindUnsupportedUploadNamesContext(context.Background(), tasks, limit)
}

func FindUnsupportedUploadNamesContext(ctx context.Context, tasks []config.Task, limit int) ([]Problem, error) {
	var problems []Problem
	for _, task := range tasks {
		if err := ctx.Err(); err != nil {
			return problems, err
		}
		root := config.ToNativePath(task.Local)
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if why := UnsupportedUploadNameReason(d.Name()); why != "" {
				problems = append(problems, Problem{
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

func UnsupportedUploadNameReason(name string) string {
	if strings.ContainsRune(name, '\uFF1A') {
		return "contains fullwidth colon U+FF1A; Baidu Netdisk through AList WebDAV can reject it with HTTP 405"
	}
	return ""
}

func FindZeroByteFiles(tasks []config.Task) ([]ZeroByteFile, error) {
	return FindZeroByteFilesContext(context.Background(), tasks)
}

func FindZeroByteFilesContext(ctx context.Context, tasks []config.Task) ([]ZeroByteFile, error) {
	var files []ZeroByteFile
	for _, task := range tasks {
		if err := ctx.Err(); err != nil {
			return files, err
		}
		root := config.ToNativePath(task.Local)
		relRoot := root
		if info, err := os.Stat(root); err == nil && !info.IsDir() {
			relRoot = filepath.Dir(root)
		}
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			if info.Size() != 0 {
				return nil
			}
			rel, err := filepath.Rel(relRoot, path)
			if err != nil {
				return err
			}
			files = append(files, ZeroByteFile{
				Task:    task.Name,
				Path:    path,
				Exclude: filepath.ToSlash(rel),
			})
			return nil
		})
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return files, fmt.Errorf("scan task %q: %w", task.Name, err)
		}
	}
	return files, nil
}

func FormatProblems(problems []Problem) string {
	var b strings.Builder
	fmt.Fprintf(&b, "found %d file name(s) likely to fail on Baidu WebDAV upload:\n", len(problems))
	for _, p := range problems {
		fmt.Fprintf(&b, "- [%s] %s (%s)\n", p.Task, p.Path, p.Why)
	}
	b.WriteString("rename those files, for example replacing the fullwidth colon with a hyphen, then rerun the command")
	return strings.TrimRight(b.String(), "\n")
}
