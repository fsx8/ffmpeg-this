//go:build unix

package execx

import (
	"os/exec"
	"syscall"
)

// setProcessGroup puts the child into its own process group and cancels by
// killing that group. ffmpeg (and shells it may spawn) then die together on
// Ctrl+C instead of leaving descendants holding the output pipes open.
func setProcessGroup(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.Cancel = func() error {
		if c.Process == nil {
			return nil
		}
		return syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
	}
}
