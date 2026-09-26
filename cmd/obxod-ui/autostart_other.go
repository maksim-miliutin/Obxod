//go:build !windows

package main

func autostartOn() bool {
	return false
}

func setAutostart(bool) error {
	return nil
}
