//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows/registry"
)

const runPath = `Software\Microsoft\Windows\CurrentVersion\Run`
const runName = "Obxod"

func autostartOn() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	_, _, err = k.GetStringValue(runName)

	return err == nil
}

func setAutostart(on bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	if !on {
		// An absent value is already off, so a delete error is not a failure.
		_ = k.DeleteValue(runName)

		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}

	return k.SetStringValue(runName, exe)
}
