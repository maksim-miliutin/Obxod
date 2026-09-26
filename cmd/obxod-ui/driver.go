package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed WinDivert.dll
var winDivertDLL []byte

//go:embed WinDivert64.sys
var winDivertSys []byte

// unpackDriver writes the embedded WinDivert files next to the exe so one
// downloaded file carries its own driver. A file already there is left as is.
func unpackDriver() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	dir := filepath.Dir(exe)

	for name, data := range map[string][]byte{
		"WinDivert.dll":   winDivertDLL,
		"WinDivert64.sys": winDivertSys,
	} {
		path := filepath.Join(dir, name)

		if _, err := os.Stat(path); err == nil {
			continue
		}

		if err := os.WriteFile(path, data, 0o644); err != nil {
			return fmt.Errorf("cannot unpack %s: %w", name, err)
		}
	}

	return nil
}
