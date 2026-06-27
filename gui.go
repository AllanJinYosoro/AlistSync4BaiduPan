package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func runGUI() {
	a := app.NewWithID("bdp-sync")
	w := a.NewWindow("bdp-sync")
	w.Resize(fyne.NewSize(920, 620))

	configPath := widget.NewEntry()
	configPath.SetText(defaultConfigPath)
	configPath.SetPlaceHolder("config.yaml")

	status := widget.NewLabel("Ready")
	logOutput := widget.NewTextGrid()
	logOutput.SetText("Command output will appear here.")
	log := &guiLogWriter{entry: logOutput}

	taskSelect := widget.NewSelect(nil, nil)
	taskSelect.PlaceHolder = "Select task"
	allTasks := widget.NewCheck("All tasks", func(checked bool) {
		if checked {
			taskSelect.Disable()
			return
		}
		taskSelect.Enable()
	})

	var controls []fyne.Disableable
	setRunning := func(running bool) {
		for _, control := range controls {
			if running {
				control.Disable()
			} else {
				control.Enable()
			}
		}
		if allTasks.Checked {
			taskSelect.Disable()
		}
	}

	loadConfig := func() {
		path := strings.TrimSpace(configPath.Text)
		if path == "" {
			path = defaultConfigPath
			configPath.SetText(path)
		}
		cfg, err := LoadConfig(path)
		if err != nil {
			status.SetText("Config load failed: " + err.Error())
			taskSelect.Options = nil
			taskSelect.ClearSelected()
			taskSelect.Refresh()
			return
		}
		names := taskNames(cfg)
		taskSelect.Options = names
		taskSelect.ClearSelected()
		if len(names) == 1 {
			taskSelect.SetSelected(names[0])
		}
		taskSelect.Refresh()
		status.SetText(fmt.Sprintf("Loaded %d task(s) from %s", len(names), path))
	}

	refreshButton := widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), loadConfig)
	clearButton := widget.NewButtonWithIcon("Clear", theme.ContentClearIcon(), func() {
		log.Clear()
		status.SetText("Ready")
	})

	var runAction func(action string)
	runAction = func(action string) {
		args, err := guiCommandArgs(action, configPath.Text, taskSelect.Selected, allTasks.Checked)
		if err != nil {
			status.SetText(err.Error())
			return
		}

		setRunning(true)
		status.SetText("Running " + action + "...")
		log.Append("\n$ bdp-sync " + strings.Join(args, " ") + "\n")

		go func() {
			runner := NewRunner(log, log)
			err := runner.Run(args)
			fyne.Do(func() {
				if err != nil {
					status.SetText("Failed: " + err.Error())
					log.Append("error: " + err.Error() + "\n")
				} else {
					status.SetText("Finished " + action + " at " + time.Now().Format("15:04:05"))
				}
				setRunning(false)
			})
		}()
	}

	doctorButton := widget.NewButtonWithIcon("Doctor", theme.SearchIcon(), func() { runAction("doctor") })
	dryRunButton := widget.NewButtonWithIcon("Dry run", theme.VisibilityIcon(), func() { runAction("dry-run") })
	updateButton := widget.NewButtonWithIcon("Update", theme.UploadIcon(), func() { runAction("update") })
	syncButton := widget.NewButtonWithIcon("Sync", theme.MediaPlayIcon(), func() {
		dialog.NewConfirm("Confirm sync", "Sync makes the remote match the local folder and may delete remote-only files.", func(ok bool) {
			if ok {
				runAction("sync")
			}
		}, w).Show()
	})

	controls = []fyne.Disableable{configPath, refreshButton, taskSelect, allTasks, doctorButton, dryRunButton, updateButton, syncButton, clearButton}

	configRow := container.NewBorder(nil, nil, widget.NewLabel("Config"), refreshButton, configPath)
	taskRow := container.NewHBox(taskSelect, allTasks, doctorButton, dryRunButton, updateButton, syncButton, clearButton)
	header := container.NewVBox(configRow, taskRow, status)
	logScroll := container.NewScroll(logOutput)
	content := container.NewBorder(header, nil, nil, nil, logScroll)

	w.SetContent(content)
	loadConfig()
	w.ShowAndRun()
}

type guiLogWriter struct {
	mu     sync.Mutex
	entry  *widget.TextGrid
	buffer terminalLogBuffer
}

func (w *guiLogWriter) Write(p []byte) (int, error) {
	w.Append(string(p))
	return len(p), nil
}

func (w *guiLogWriter) Append(text string) {
	w.mu.Lock()
	current := w.buffer.Append(text)
	w.mu.Unlock()
	fyne.Do(func() {
		w.entry.SetText(current)
	})
}

func (w *guiLogWriter) Clear() {
	w.mu.Lock()
	w.buffer.Clear()
	w.mu.Unlock()
	w.entry.SetText("")
}
