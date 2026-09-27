//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

const taskName = "Obxod"

func autostartOn() bool {
	return schtasks("/Query", "/TN", taskName) == nil
}

func setAutostart(on bool) error {
	if !on {
		// An absent task is already off, so a delete error is not a failure.
		_ = schtasks("/Delete", "/TN", taskName, "/F")

		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}

	// ONLOGON with HIGHEST runs at sign-in, elevated, without a UAC prompt.
	return schtasks("/Create", "/TN", taskName, "/TR", "\""+exe+"\"", "/SC", "ONLOGON", "/RL", "HIGHEST", "/F")
}

func schtasks(args ...string) error {
	cmd := exec.Command("schtasks", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks: %w: %s", err, out)
	}

	return nil
}
