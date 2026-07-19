//go:build windows

package cmd

import (
	"os"
	"os/exec"
)

func setSysProcAttr(_ *exec.Cmd) {
	// On Windows, child processes are independent by default.
}

// processAlive reports whether the process with the given pid is still running.
func processAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	p.Release() //nolint:errcheck
	return true
}
