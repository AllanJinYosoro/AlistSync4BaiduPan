package gui

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"bdp-sync/internal/config"
	"bdp-sync/internal/deps"
	"bdp-sync/internal/runner"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func Run() {
	a := app.NewWithID("bdp-sync")
	w := a.NewWindow("bdp-sync")
	w.Resize(fyne.NewSize(1040, 720))

	configPath := widget.NewEntry()
	configPath.SetText(config.DefaultPath)
	configPath.SetPlaceHolder("config.yaml")

	status := widget.NewLabel("Ready")
	logOutput := widget.NewLabelWithStyle("Command output will appear here.", fyne.TextAlignLeading, fyne.TextStyle{Monospace: true})
	logOutput.Selectable = true
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

	alistURL := widget.NewEntry()
	alistUser := widget.NewEntry()
	passwordEnv := widget.NewEntry()
	serverCommand := widget.NewEntry()
	startupTimeout := widget.NewEntry()
	rcloneRemote := widget.NewEntry()
	rcloneConfig := widget.NewEntry()
	transfers := widget.NewEntry()
	checkers := widget.NewEntry()
	retries := widget.NewEntry()
	lowLevelRetries := widget.NewEntry()
	excludes := widget.NewMultiLineEntry()
	excludes.SetMinRowsVisible(4)
	configTaskSelect := widget.NewSelect(nil, nil)
	configTaskSelect.PlaceHolder = "Select task"
	taskName := widget.NewEntry()
	taskLocal := widget.NewEntry()
	taskRemote := widget.NewEntry()
	taskExcludes := widget.NewMultiLineEntry()
	taskExcludes.SetMinRowsVisible(4)
	yamlEntry := widget.NewMultiLineEntry()
	yamlEntry.SetMinRowsVisible(20)
	depStatus := widget.NewTextGrid()

	var currentCfg config.Config
	selectedConfigTask := -1
	loadingConfigTask := false
	var controls []fyne.Disableable
	var currentCancel context.CancelFunc
	currentRun := 0
	var stopButton *widget.Button
	var loadConfig func()
	var refreshDeps func(prompt bool)
	var saveSelectedConfigTask func()
	var refreshConfigTaskSelect func(int)
	var loadSelectedConfigTask func()

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

	taskFormValuesFromInputs := func() taskFormValues {
		return taskFormValues{
			Name:         taskName.Text,
			Local:        taskLocal.Text,
			Remote:       taskRemote.Text,
			ExcludesText: taskExcludes.Text,
		}
	}

	saveSelectedConfigTask = func() {
		applyTaskValues(&currentCfg, selectedConfigTask, taskFormValuesFromInputs())
	}

	loadSelectedConfigTask = func() {
		if selectedConfigTask < 0 || selectedConfigTask >= len(currentCfg.Tasks) {
			taskName.SetText("")
			taskLocal.SetText("")
			taskRemote.SetText("")
			taskExcludes.SetText("")
			return
		}
		values := taskValues(currentCfg.Tasks[selectedConfigTask])
		taskName.SetText(values.Name)
		taskLocal.SetText(values.Local)
		taskRemote.SetText(values.Remote)
		taskExcludes.SetText(values.ExcludesText)
	}

	refreshConfigTaskSelect = func(index int) {
		labels := taskLabels(currentCfg.Tasks)
		loadingConfigTask = true
		configTaskSelect.Options = labels
		if len(labels) == 0 {
			selectedConfigTask = -1
			configTaskSelect.ClearSelected()
			configTaskSelect.Refresh()
			loadingConfigTask = false
			loadSelectedConfigTask()
			return
		}
		if index < 0 {
			index = 0
		}
		if index >= len(labels) {
			index = len(labels) - 1
		}
		selectedConfigTask = index
		configTaskSelect.SetSelected(labels[index])
		configTaskSelect.Refresh()
		loadingConfigTask = false
		loadSelectedConfigTask()
	}

	configTaskSelect.OnChanged = func(selected string) {
		if loadingConfigTask {
			return
		}
		saveSelectedConfigTask()
		selectedConfigTask = selectedTaskIndex(configTaskSelect.Options, selected)
		loadSelectedConfigTask()
	}

	fillForm := func(cfg config.Config) {
		currentCfg = cfg.Clone()
		alistURL.SetText(cfg.AList.URL)
		alistUser.SetText(cfg.AList.Username)
		passwordEnv.SetText(cfg.AList.PasswordEnv)
		serverCommand.SetText(cfg.AList.ServerCommand)
		startupTimeout.SetText(strconv.Itoa(cfg.AList.StartupTimeoutSeconds))
		rcloneRemote.SetText(cfg.Rclone.Remote)
		rcloneConfig.SetText(cfg.Rclone.ConfigFile)
		transfers.SetText(strconv.Itoa(cfg.Rclone.Transfers))
		checkers.SetText(strconv.Itoa(cfg.Rclone.Checkers))
		retries.SetText(strconv.Itoa(cfg.Rclone.Retries))
		lowLevelRetries.SetText(strconv.Itoa(cfg.Rclone.LowLevelRetries))
		excludes.SetText(strings.Join(cfg.Rclone.Excludes, "\n"))
		refreshConfigTaskSelect(0)
	}

	loadConfig = func() {
		path := strings.TrimSpace(configPath.Text)
		if path == "" {
			path = config.DefaultPath
			configPath.SetText(path)
		}
		cfg, err := config.Load(path)
		if err != nil {
			status.SetText("Config load failed: " + err.Error())
			taskSelect.Options = nil
			taskSelect.ClearSelected()
			taskSelect.Refresh()
			return
		}
		names := config.TaskNames(cfg)
		taskSelect.Options = names
		taskSelect.ClearSelected()
		if len(names) == 1 {
			taskSelect.SetSelected(names[0])
		}
		taskSelect.Refresh()
		fillForm(cfg)
		if raw, err := config.LoadText(path); err == nil {
			yamlEntry.SetText(string(raw))
		} else if data, err := config.FormatYAML(cfg); err == nil {
			yamlEntry.SetText(string(data))
		}
		status.SetText(fmt.Sprintf("Loaded %d task(s) from %s", len(names), path))
	}

	applyForm := func() (config.Config, error) {
		saveSelectedConfigTask()
		cfg := currentCfg.Clone()
		cfg.AList.URL = strings.TrimSpace(alistURL.Text)
		cfg.AList.Username = strings.TrimSpace(alistUser.Text)
		cfg.AList.PasswordEnv = strings.TrimSpace(passwordEnv.Text)
		cfg.AList.ServerCommand = strings.TrimSpace(serverCommand.Text)
		var err error
		cfg.AList.StartupTimeoutSeconds, err = strconv.Atoi(strings.TrimSpace(startupTimeout.Text))
		if err != nil {
			return config.Config{}, fmt.Errorf("alist.startup_timeout_seconds must be a number")
		}
		cfg.Rclone.Remote = strings.TrimSpace(rcloneRemote.Text)
		cfg.Rclone.ConfigFile = strings.TrimSpace(rcloneConfig.Text)
		cfg.Rclone.Transfers, err = strconv.Atoi(strings.TrimSpace(transfers.Text))
		if err != nil {
			return config.Config{}, fmt.Errorf("rclone.transfers must be a number")
		}
		cfg.Rclone.Checkers, err = strconv.Atoi(strings.TrimSpace(checkers.Text))
		if err != nil {
			return config.Config{}, fmt.Errorf("rclone.checkers must be a number")
		}
		cfg.Rclone.Retries, err = strconv.Atoi(strings.TrimSpace(retries.Text))
		if err != nil {
			return config.Config{}, fmt.Errorf("rclone.retries must be a number")
		}
		cfg.Rclone.LowLevelRetries, err = strconv.Atoi(strings.TrimSpace(lowLevelRetries.Text))
		if err != nil {
			return config.Config{}, fmt.Errorf("rclone.low_level_retries must be a number")
		}
		cfg.Rclone.Excludes = splitLines(excludes.Text)
		if err := cfg.Validate(); err != nil {
			return config.Config{}, err
		}
		return cfg, nil
	}

	saveForm := func() {
		cfg, err := applyForm()
		if err != nil {
			status.SetText("Config save failed: " + err.Error())
			return
		}
		path := strings.TrimSpace(configPath.Text)
		if path == "" {
			path = config.DefaultPath
		}
		if err := config.Save(path, cfg); err != nil {
			status.SetText("Config save failed: " + err.Error())
			return
		}
		status.SetText("Saved " + path)
		loadConfig()
	}

	saveYAML := func() {
		path := strings.TrimSpace(configPath.Text)
		if path == "" {
			path = config.DefaultPath
		}
		if err := config.SaveRaw(path, []byte(yamlEntry.Text)); err != nil {
			status.SetText("YAML save failed: " + err.Error())
			return
		}
		status.SetText("Saved " + path)
		loadConfig()
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

		ctx, cancel := context.WithCancel(context.Background())
		currentRun++
		runID := currentRun
		currentCancel = cancel
		stopButton.Enable()
		setRunning(true)
		status.SetText("Running " + action + "...")
		log.Append("\n$ bdp-sync " + strings.Join(args, " ") + "\n")

		go func() {
			r := runner.NewContext(ctx, log, log)
			err := r.Run(args)
			fyne.Do(func() {
				if currentRun == runID {
					currentCancel = nil
				}
				stopButton.Disable()
				if ctx.Err() != nil {
					status.SetText("Stopped " + action)
				} else if err != nil {
					status.SetText("Failed: " + err.Error())
					log.Append("error: " + err.Error() + "\n")
				} else {
					status.SetText("Finished " + action + " at " + time.Now().Format("15:04:05"))
				}
				setRunning(false)
			})
		}()
	}

	installDeps := func(force bool) {
		setRunning(true)
		status.SetText("Installing dependencies...")
		log.Append("\n$ bdp-sync setup deps\n")
		go func() {
			err := deps.EnsureAll(force, log)
			fyne.Do(func() {
				if err != nil {
					status.SetText("Dependency install failed: " + err.Error())
					log.Append("error: " + err.Error() + "\n")
				} else {
					status.SetText("Dependencies ready")
				}
				setRunning(false)
				refreshDeps(false)
			})
		}()
	}

	chooseTool := func(name string) {
		d := dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
			if err != nil {
				status.SetText(err.Error())
				return
			}
			if rc == nil {
				return
			}
			defer rc.Close()
			if err := config.EnsureLocalDirs(); err != nil {
				status.SetText(err.Error())
				return
			}
			out, err := os.OpenFile(deps.LocalToolPath(name), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
			if err != nil {
				status.SetText(err.Error())
				return
			}
			defer out.Close()
			if _, err := io.Copy(out, rc); err != nil {
				status.SetText(err.Error())
				return
			}
			status.SetText("Saved " + deps.LocalToolPath(name))
			refreshDeps(false)
		}, w)
		d.SetFilter(storage.NewExtensionFileFilter([]string{".exe"}))
		d.Show()
	}

	refreshDeps = func(prompt bool) {
		st := deps.Check()
		var b strings.Builder
		for _, tool := range st.Tools {
			if tool.Available {
				fmt.Fprintf(&b, "[OK]   %s: %s (%s)\n", tool.Name, tool.Path, tool.Source)
			} else {
				fmt.Fprintf(&b, "[FAIL] %s: %s\n", tool.Name, tool.Detail)
			}
		}
		depStatus.SetText(strings.TrimRight(b.String(), "\n"))
		if prompt && !st.Ready() {
			dialog.NewConfirm("Install dependencies", "rclone or AList is missing. Install missing dependencies into .alist-sync/tools now?", func(ok bool) {
				if ok {
					installDeps(false)
				}
			}, w).Show()
		}
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
	stopButton = widget.NewButtonWithIcon("Stop", theme.MediaStopIcon(), func() {
		if currentCancel == nil {
			return
		}
		status.SetText("Stopping...")
		log.Append("stop requested\n")
		currentCancel()
		stopButton.Disable()
	})
	stopButton.Disable()

	newTaskButton := widget.NewButtonWithIcon("New task", theme.ContentAddIcon(), func() {
		saveSelectedConfigTask()
		refreshConfigTaskSelect(appendTask(&currentCfg))
		status.SetText("New task added. Fill it in and save.")
	})
	deleteTaskButton := widget.NewButtonWithIcon("Delete task", theme.DeleteIcon(), func() {
		if selectedConfigTask < 0 || selectedConfigTask >= len(currentCfg.Tasks) {
			status.SetText("Select a task to delete")
			return
		}
		name := strings.TrimSpace(currentCfg.Tasks[selectedConfigTask].Name)
		if name == "" {
			name = "this unnamed task"
		}
		dialog.NewConfirm("Delete task", "Delete "+name+" from the config form?", func(ok bool) {
			if !ok {
				return
			}
			refreshConfigTaskSelect(deleteTask(&currentCfg, selectedConfigTask))
			status.SetText("Task deleted. Save to write the config.")
		}, w).Show()
	})

	controls = []fyne.Disableable{configPath, refreshButton, taskSelect, allTasks, doctorButton, dryRunButton, updateButton, syncButton, clearButton, configTaskSelect, newTaskButton, deleteTaskButton}

	configRow := container.NewBorder(nil, nil, widget.NewLabel("Config"), refreshButton, configPath)
	taskRow := container.NewHBox(taskSelect, allTasks, doctorButton, dryRunButton, updateButton, syncButton, stopButton, clearButton)
	syncHeader := container.NewVBox(configRow, taskRow, status)
	logScroll := container.NewScroll(logOutput)
	log.scroll = logScroll
	syncTab := container.NewBorder(syncHeader, nil, nil, nil, logScroll)

	form := widget.NewForm(
		widget.NewFormItem("AList URL", alistURL),
		widget.NewFormItem("AList user", alistUser),
		widget.NewFormItem("Password env", passwordEnv),
		widget.NewFormItem("Server command", serverCommand),
		widget.NewFormItem("Startup timeout", startupTimeout),
		widget.NewFormItem("Rclone remote", rcloneRemote),
		widget.NewFormItem("Rclone config", rcloneConfig),
		widget.NewFormItem("Transfers", transfers),
		widget.NewFormItem("Checkers", checkers),
		widget.NewFormItem("Retries", retries),
		widget.NewFormItem("Low-level retries", lowLevelRetries),
		widget.NewFormItem("Global excludes", excludes),
		widget.NewFormItem("Task", container.NewHBox(configTaskSelect, newTaskButton, deleteTaskButton)),
		widget.NewFormItem("Task name", taskName),
		widget.NewFormItem("Task local", taskLocal),
		widget.NewFormItem("Task remote", taskRemote),
		widget.NewFormItem("Task excludes", taskExcludes),
	)
	formButtons := container.NewHBox(widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), saveForm), widget.NewButtonWithIcon("Reload", theme.ViewRefreshIcon(), loadConfig))
	formTab := container.NewBorder(nil, formButtons, nil, nil, container.NewScroll(form))
	yamlButtons := container.NewHBox(widget.NewButtonWithIcon("Save YAML", theme.DocumentSaveIcon(), saveYAML), widget.NewButtonWithIcon("Reload", theme.ViewRefreshIcon(), loadConfig))
	yamlTab := container.NewBorder(nil, yamlButtons, nil, nil, container.NewScroll(yamlEntry))
	configTabs := container.NewAppTabs(container.NewTabItem("Form", formTab), container.NewTabItem("YAML", yamlTab))

	depButtons := container.NewHBox(
		widget.NewButtonWithIcon("Recheck", theme.ViewRefreshIcon(), func() { refreshDeps(false) }),
		widget.NewButtonWithIcon("Install", theme.DownloadIcon(), func() { installDeps(false) }),
		widget.NewButtonWithIcon("Force reinstall", theme.DownloadIcon(), func() { installDeps(true) }),
		widget.NewButtonWithIcon("Use rclone", theme.FolderOpenIcon(), func() { chooseTool("rclone") }),
		widget.NewButtonWithIcon("Use AList", theme.FolderOpenIcon(), func() { chooseTool("alist") }),
	)
	depsTab := container.NewBorder(depButtons, nil, nil, nil, container.NewScroll(depStatus))

	tabs := container.NewAppTabs(
		container.NewTabItem("Sync", syncTab),
		container.NewTabItem("Config", configTabs),
		container.NewTabItem("Dependencies", depsTab),
	)

	w.SetContent(tabs)
	loadConfig()
	refreshDeps(false)
	w.Show()
	refreshDeps(true)
	a.Run()
}

func splitLines(text string) []string {
	var values []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			values = append(values, line)
		}
	}
	return values
}

type guiLogWriter struct {
	mu     sync.Mutex
	entry  *widget.Label
	scroll *container.Scroll
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
		if w.scroll != nil {
			w.scroll.ScrollToBottom()
		}
	})
}

func (w *guiLogWriter) Clear() {
	w.mu.Lock()
	w.buffer.Clear()
	w.mu.Unlock()
	w.entry.SetText("")
}
