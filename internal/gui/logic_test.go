package gui

import (
	"reflect"
	"testing"

	"bdp-sync/internal/config"
)

func TestTaskNamesSkipsEmptyNames(t *testing.T) {
	got := config.TaskNames(config.Config{Tasks: []config.Task{{Name: "PASSRec"}, {}, {Name: "BioGNN"}}})
	want := []string{"PASSRec", "BioGNN"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("task names mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestGUICommandArgs(t *testing.T) {
	tests := []struct {
		name     string
		action   string
		config   string
		selected string
		all      bool
		want     []string
		wantErr  bool
	}{
		{
			name:   "doctor",
			action: "doctor",
			config: "custom.yaml",
			want:   []string{"doctor", "--config", "custom.yaml"},
		},
		{
			name:     "dry run selected task",
			action:   "dry-run",
			config:   "config.yaml",
			selected: "PASSRec",
			want:     []string{"dry-run", "--config", "config.yaml", "PASSRec"},
		},
		{
			name:   "update all tasks",
			action: "update",
			config: "config.yaml",
			all:    true,
			want:   []string{"update", "--config", "config.yaml", "--all"},
		},
		{
			name:     "default config path",
			action:   "sync",
			selected: "BioGNN",
			want:     []string{"sync", "--config", config.DefaultPath, "BioGNN"},
		},
		{
			name:    "missing task",
			action:  "sync",
			config:  "config.yaml",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := guiCommandArgs(tt.action, tt.config, tt.selected, tt.all)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("args mismatch\n got: %#v\nwant: %#v", got, tt.want)
			}
		})
	}
}

func TestTaskFormHelpers(t *testing.T) {
	cfg := config.Config{Tasks: []config.Task{
		{Name: "one", Local: "C:/one", Remote: "/one"},
		{Name: "two", Local: "C:/two", Remote: "/two"},
	}}

	applyTaskValues(&cfg, 0, taskFormValues{
		Name:         " updated ",
		Local:        " C:/updated ",
		Remote:       " /updated ",
		ExcludesText: "tmp/**\n\ncache/**",
	})
	wantTask := config.Task{
		Name:     "updated",
		Local:    "C:/updated",
		Remote:   "/updated",
		Excludes: []string{"tmp/**", "cache/**"},
	}
	if !reflect.DeepEqual(cfg.Tasks[0], wantTask) {
		t.Fatalf("task update mismatch\n got: %#v\nwant: %#v", cfg.Tasks[0], wantTask)
	}

	newIndex := appendTask(&cfg)
	if newIndex != 2 || len(cfg.Tasks) != 3 {
		t.Fatalf("append mismatch: index=%d tasks=%#v", newIndex, cfg.Tasks)
	}
	labels := taskLabels(cfg.Tasks)
	if labels[0] != "1. updated" || labels[2] != "3. (unnamed task)" {
		t.Fatalf("unexpected labels: %#v", labels)
	}
	if selectedTaskIndex(labels, labels[1]) != 1 {
		t.Fatalf("selected index mismatch for labels %#v", labels)
	}

	next := deleteTask(&cfg, 1)
	if next != 1 || len(cfg.Tasks) != 2 {
		t.Fatalf("delete middle mismatch: next=%d tasks=%#v", next, cfg.Tasks)
	}
	next = deleteTask(&cfg, 1)
	if next != 0 || len(cfg.Tasks) != 1 {
		t.Fatalf("delete last mismatch: next=%d tasks=%#v", next, cfg.Tasks)
	}
	next = deleteTask(&cfg, 0)
	if next != -1 || len(cfg.Tasks) != 0 {
		t.Fatalf("delete only mismatch: next=%d tasks=%#v", next, cfg.Tasks)
	}
}

func TestTerminalLogBufferOverwritesCarriageReturnProgress(t *testing.T) {
	var b terminalLogBuffer
	got := b.Append("start\nTransferred: 1 MiB / 10 MiB\rTransferred: 5 MiB / 10 MiB\rTransferred: 10 MiB / 10 MiB\nfinished\n")
	want := "start\nTransferred: 10 MiB / 10 MiB\nfinished\n"
	if got != want {
		t.Fatalf("terminal log mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestTerminalLogBufferPreservesCRLFAndStripsANSI(t *testing.T) {
	var b terminalLogBuffer
	got := b.Append("one\r\n\x1b[32mtwo\x1b[0m\rthree\n")
	want := "one\nthree\n"
	if got != want {
		t.Fatalf("terminal log mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestTerminalLogBufferHandlesSplitCarriageReturn(t *testing.T) {
	var b terminalLogBuffer
	if got := b.Append("Transferred: 1 MiB\r"); got != "Transferred: 1 MiB" {
		t.Fatalf("unexpected first chunk: %q", got)
	}
	got := b.Append("Transferred: 2 MiB\n")
	want := "Transferred: 2 MiB\n"
	if got != want {
		t.Fatalf("terminal log mismatch\n got: %q\nwant: %q", got, want)
	}

	b.Clear()
	b.Append("line\r")
	got = b.Append("\n")
	want = "line\n"
	if got != want {
		t.Fatalf("CRLF split mismatch\n got: %q\nwant: %q", got, want)
	}
}
