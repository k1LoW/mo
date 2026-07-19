//go:build !windows

package cmd

import (
	"errors"
	"os/exec"
	"syscall"
)

func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

// processAlive reports whether the process with the given pid is still running.
// A spawned child stays a zombie after death until reaped (the parent never
// calls Wait), and kill(pid, 0) succeeds on zombies — so try a non-blocking
// reap first and only fall back to signal 0 when pid is not our child.
func processAlive(pid int) bool {
	var ws syscall.WaitStatus
	wpid, err := syscall.Wait4(pid, &ws, syscall.WNOHANG, nil)
	if err == nil {
		// wpid == pid means the child exited and has now been reaped.
		return wpid != pid
	}
	if errors.Is(err, syscall.ECHILD) {
		kerr := syscall.Kill(pid, 0)
		return kerr == nil || errors.Is(kerr, syscall.EPERM)
	}
	return true
}
