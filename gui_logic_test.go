package main

import (
	"reflect"
	"testing"
)

func TestTaskNamesSkipsEmptyNames(t *testing.T) {
	got := taskNames(Config{Tasks: []Task{{Name: "PASSRec"}, {}, {Name: "BioGNN"}}})
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
			want:     []string{"sync", "--config", defaultConfigPath, "BioGNN"},
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
