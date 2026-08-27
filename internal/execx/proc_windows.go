//go:build windows

package execx

import (
	"os/exec"
)

// setProcessGroup is a no-op on Windows: the default Kill terminates the
// direct child, which covers the ffmpeg processes spawned by ffwiz.
func setProcessGroup(c *exec.Cmd) {}
