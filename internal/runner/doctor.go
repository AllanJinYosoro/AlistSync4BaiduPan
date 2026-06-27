package runner

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"bdp-sync/internal/alist"
	"bdp-sync/internal/config"
	"bdp-sync/internal/deps"
	"bdp-sync/internal/filename"
	"bdp-sync/internal/rclone"
)

func (r Runner) cmdDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(r.stderr)
	configPath := fs.String("config", config.DefaultPath, "config file path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	checks := []doctorCheck{
		checkCommand("git", "--version"),
		checkCommand("go", "version"),
		checkTool("alist"),
		checkTool("rclone"),
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		checks = append(checks, doctorCheck{"config", false, err.Error()})
	} else {
		validateErr := cfg.Validate()
		checks = append(checks, doctorCheck{"config", validateErr == nil, errString(validateErr)})
		checks = append(checks, checkPasswordEnv(cfg))
		checks = append(checks, checkUploadNames(cfg))
		if validateErr == nil {
			checks = append(checks, r.checkLiveServices(cfg)...)
		}
	}

	failed := false
	for _, c := range checks {
		if c.ok {
			fmt.Fprintf(r.stdout, "[OK]   %s\n", c.name)
			continue
		}
		failed = true
		fmt.Fprintf(r.stdout, "[FAIL] %s: %s\n", c.name, c.detail)
	}
	if failed {
		return errors.New("doctor found problems")
	}
	return nil
}

func (r Runner) checkLiveServices(cfg config.Config) []doctorCheck {
	var checks []doctorCheck
	stop, err := alist.EnsureReadyManaged(cfg, r.managedStarter(), r.stdout)
	if err != nil {
		checks = append(checks, doctorCheck{"alist.url", false, err.Error()})
		checks = append(checks, doctorCheck{"rclone config", false, "skipped because AList is not reachable"})
		checks = append(checks, doctorCheck{"rclone remote", false, "skipped because AList is not reachable"})
		return checks
	}
	checks = append(checks, checkAListURL(cfg))
	checks = append(checks, r.checkRcloneConfig(cfg))
	checks = append(checks, r.checkRcloneRemote(cfg))
	if stop != nil {
		if err := stop(); err != nil {
			checks = append(checks, doctorCheck{"alist shutdown", false, err.Error()})
		} else {
			checks = append(checks, doctorCheck{"alist shutdown", true, ""})
		}
	}
	return checks
}

func (r Runner) managedStarter() alist.ManagedStarter {
	if r.startManaged != nil {
		return r.startManaged
	}
	return nil
}

type doctorCheck struct {
	name   string
	ok     bool
	detail string
}

func checkCommand(name string, args ...string) doctorCheck {
	err := exec.Command(name, args...).Run()
	return doctorCheck{name: name, ok: err == nil, detail: errString(err)}
}

func checkTool(name string) doctorCheck {
	p, err := deps.FindTool(name)
	if err != nil {
		return doctorCheck{name: name, ok: false, detail: err.Error()}
	}
	return doctorCheck{name: name, ok: true, detail: p}
}

func checkPasswordEnv(cfg config.Config) doctorCheck {
	value := os.Getenv(cfg.AList.PasswordEnv)
	return doctorCheck{
		name:   cfg.AList.PasswordEnv,
		ok:     value != "",
		detail: "environment variable is empty",
	}
}

func checkAListURL(cfg config.Config) doctorCheck {
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(strings.TrimRight(cfg.AList.URL, "/"))
	if err != nil {
		return doctorCheck{name: "alist.url", ok: false, detail: err.Error()}
	}
	defer resp.Body.Close()
	return doctorCheck{name: "alist.url", ok: resp.StatusCode < 500, detail: resp.Status}
}

func (r Runner) checkRcloneConfig(cfg config.Config) doctorCheck {
	err := rclone.EnsureConfig(cfg, r.runOutput, r.stdout)
	return doctorCheck{name: "rclone config", ok: err == nil, detail: errString(err)}
}

func (r Runner) checkRcloneRemote(cfg config.Config) doctorCheck {
	rclonePath, err := deps.FindTool("rclone")
	if err != nil {
		return doctorCheck{name: "rclone remote", ok: false, detail: err.Error()}
	}
	err = r.exec(rclonePath, "lsd", cfg.Rclone.Remote+":", "--config", cfg.RcloneConfigPath())
	return doctorCheck{name: "rclone remote", ok: err == nil, detail: errString(err)}
}

func checkUploadNames(cfg config.Config) doctorCheck {
	problems, err := filename.FindUnsupportedUploadNames(cfg.Tasks, filename.MaxProblems)
	if err != nil {
		return doctorCheck{name: "upload filenames", ok: false, detail: err.Error()}
	}
	if len(problems) > 0 {
		return doctorCheck{name: "upload filenames", ok: false, detail: filename.FormatProblems(problems)}
	}
	return doctorCheck{name: "upload filenames", ok: true}
}
