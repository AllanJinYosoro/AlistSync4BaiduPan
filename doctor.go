package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

func (r Runner) cmdDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(r.stderr)
	configPath := fs.String("config", defaultConfigPath, "config file path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	checks := []doctorCheck{
		checkCommand("git", "--version"),
		checkCommand("go", "version"),
		checkTool("alist"),
		checkTool("rclone"),
	}
	cfg, err := LoadConfig(*configPath)
	if err != nil {
		checks = append(checks, doctorCheck{"config", false, err.Error()})
	} else {
		validateErr := cfg.Validate()
		checks = append(checks, doctorCheck{"config", validateErr == nil, errString(validateErr)})
		checks = append(checks, checkPasswordEnv(cfg))
		checks = append(checks, checkAListURL(cfg))
		checks = append(checks, checkRcloneRemote(cfg))
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
	p, err := findTool(name)
	if err != nil {
		return doctorCheck{name: name, ok: false, detail: err.Error()}
	}
	return doctorCheck{name: name, ok: true, detail: p}
}

func checkPasswordEnv(cfg Config) doctorCheck {
	value := os.Getenv(cfg.AList.PasswordEnv)
	return doctorCheck{
		name:   cfg.AList.PasswordEnv,
		ok:     value != "",
		detail: "environment variable is empty",
	}
}

func checkAListURL(cfg Config) doctorCheck {
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(strings.TrimRight(cfg.AList.URL, "/"))
	if err != nil {
		return doctorCheck{name: "alist.url", ok: false, detail: err.Error()}
	}
	defer resp.Body.Close()
	return doctorCheck{name: "alist.url", ok: resp.StatusCode < 500, detail: resp.Status}
}

func checkRcloneRemote(cfg Config) doctorCheck {
	rclonePath, err := findTool("rclone")
	if err != nil {
		return doctorCheck{name: "rclone remote", ok: false, detail: err.Error()}
	}
	err = exec.Command(rclonePath, "lsd", cfg.Rclone.Remote+":", "--config", cfg.RcloneConfigPath()).Run()
	return doctorCheck{name: "rclone remote", ok: err == nil, detail: errString(err)}
}
