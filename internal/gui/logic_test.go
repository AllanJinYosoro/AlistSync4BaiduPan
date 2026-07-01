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

func TestTerminalLogBufferOverwritesANSIProgressBlock(t *testing.T) {
	var b terminalLogBuffer
	got := b.Append("start\nErrors: 0\nChecks: 1\nTransferred: 1 MiB\n\x1b[3AErrors: 0\nChecks: 2\nTransferred: 2 MiB\nfinished\n")
	want := "start\nErrors: 0\nChecks: 2\nTransferred: 2 MiB\nfinished\n"
	if got != want {
		t.Fatalf("terminal log mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestTerminalLogBufferClearsCurrentLine(t *testing.T) {
	var b terminalLogBuffer
	got := b.Append("start\nTransferred: old\r\x1b[KTransferred: new\n")
	want := "start\nTransferred: new\n"
	if got != want {
		t.Fatalf("terminal log mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestTerminalLogBufferCollapsesRcloneProgressBlocksWithoutControlCodes(t *testing.T) {
	var b terminalLogBuffer
	got := b.Append("start\nTransferred:\n    1 MiB / 10 MiB, 10%, 1 MiB/s, ETA 9s\nChecks:\n    1 / 10, 10%\nChecking:\n * a.txt: checking\nTransferred:\n    2 MiB / 10 MiB, 20%, 1 MiB/s, ETA 8s\nChecks:\n    2 / 10, 20%\nChecking:\n * b.txt: checking\nfinished\n")
	want := "start\nTransferred:\n    2 MiB / 10 MiB, 20%, 1 MiB/s, ETA 8s\nChecks:\n    2 / 10, 20%\nChecking:\n * b.txt: checking\nfinished\n"
	if got != want {
		t.Fatalf("terminal log mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestTerminalLogBufferKeepsSingleRcloneStatsBlockFieldsTogether(t *testing.T) {
	var b terminalLogBuffer
	got := b.Append("Errors:\n    1 (retrying may help)\nChecks:\n    1 / 2, 50%\nTransferred:\n    1 MiB / 2 MiB, 50%, 1 MiB/s, ETA 1s\nChecking:\n * a.txt: checking\nErrors:\n    1 (retrying may help)\nChecks:\n    2 / 2, 100%\nTransferred:\n    2 MiB / 2 MiB, 100%, 1 MiB/s, ETA 0s\n")
	want := "Errors:\n    1 (retrying may help)\nChecks:\n    2 / 2, 100%\nTransferred:\n    2 MiB / 2 MiB, 100%, 1 MiB/s, ETA 0s\n"
	if got != want {
		t.Fatalf("terminal log mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestTerminalLogBufferCollapsesRcloneProgressAroundDryRunLines(t *testing.T) {
	var b terminalLogBuffer
	got := b.Append("start\nTransferred:\n    1 MiB / 10 MiB, 10%, 1 MiB/s, ETA 9s\nChecks: 1 / 10, 10%\nElapsed time: 0.1s\nChecking:\n * old.txt: checking\n= old.txt\nTransferred:\n    2 MiB / 10 MiB, 20%, 1 MiB/s, ETA 8s\nChecks: 2 / 10, 20%\nElapsed time: 0.2s\nChecking:\n * new.txt: checking\n= new.txt\nTransferred:\n    3 MiB / 10 MiB, 30%, 1 MiB/s, ETA 7s\nChecks: 3 / 10, 30%\nElapsed time: 0.3s\n")
	want := "start\n= old.txt\n= new.txt\nTransferred:\n    3 MiB / 10 MiB, 30%, 1 MiB/s, ETA 7s\nChecks: 3 / 10, 30%\nElapsed time: 0.3s\n"
	if got != want {
		t.Fatalf("terminal log mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestTerminalLogBufferCollapsesRealRcloneDryRunProgress(t *testing.T) {
	var b terminalLogBuffer
	got := b.Append(`026-5-25会议/滴滴报销/回程-滴滴电子发票.pdf
Transferred:   
          0 B / 0 B, -, 0 B/s, ETA -
Checks:                31 / 32, 97%, Listed 96
Elapsed time:         0.1s
Checking:
 *       Timeline/2026-5-25会议/滴滴报销/回程-滴滴电子发票.pdf: ch
= %SystemDrive%/ProgramData/SogouInput/Components/Picface/Cloud/sgim_picface_cloud.bin
Transferred:   
          0 B / 0 B, -, 0 B/s, ETA -
Checks:                32 / 34, 94%, Listed 105
Elapsed time:         0.1s
Checking:
 * %SystemDrive%/ProgramD…sgim_picface_cloud.bin: checking
 * %SystemDrive%/ProgramD…_picface_cloud_bak.bin: checking
= %SystemDrive%/ProgramData/SogouInput/Components/Picface/Cloud/sgim_picface_cloud_bak.bin
Transferred:   
          0 B / 0 B, -, 0 B/s, ETA -
Checks:                33 / 34, 97%, Listed 105
Elapsed time:         0.1s
Checking:
 * %SystemDrive%/ProgramD…_picface_cloud_bak.bin: checkingTransferred:   
          0 B / 0 B, -, 0 B/s, ETA -
Checks:                34 / 34, 100%, Listed 105
Elapsed time:         0.1s
2026/07/01 21:49:02 NOTICE: 
Transferred:   
          0 B / 0 B, -, 0 B/s, ETA -
Checks:                34 / 34, 100%, Listed 105
Elapsed time:         0.1s
`)
	want := `026-5-25会议/滴滴报销/回程-滴滴电子发票.pdf
= %SystemDrive%/ProgramData/SogouInput/Components/Picface/Cloud/sgim_picface_cloud.bin
= %SystemDrive%/ProgramData/SogouInput/Components/Picface/Cloud/sgim_picface_cloud_bak.bin
2026/07/01 21:49:02 NOTICE: 
Transferred:   
          0 B / 0 B, -, 0 B/s, ETA -
Checks:                34 / 34, 100%, Listed 105
Elapsed time:         0.1s
`
	if got != want {
		t.Fatalf("terminal log mismatch\n got: %q\nwant: %q", got, want)
	}
}
