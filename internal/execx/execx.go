package execx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

type Cmd struct {
	Name string
	Args []string
}

func (c Cmd) String() string {
	var b bytes.Buffer
	b.WriteString(c.Name)
	for _, a := range c.Args {
		b.WriteByte(' ')
		b.WriteString(a)
	}
	return b.String()
}

type Runner interface {
	Run(ctx context.Context, cmd Cmd) (stdout string, stderr string, err error)
	// RunStreaming delivers split lines from the child's stderr (and stdout,
	// when onStdout is set; nil means drain-and-discard). Either callback may
	// be nil.
	RunStreaming(ctx context.Context, cmd Cmd, onStdout, onStderr func(line string)) (exitCode int, err error)
}

type runner struct{}

func New() Runner { return runner{} }

// childProc applies the common hardening to every spawned process: it is
// placed in its own process group (platform-specific) and Wait is bounded
// by a delay once the process exits or the context is cancelled, so a
// grandchild holding the output pipes open cannot hang the caller forever.
const waitDelay = 5 * time.Second

func prepareCmd(c *exec.Cmd) {
	setProcessGroup(c)
	c.WaitDelay = waitDelay
}

func (runner) Run(ctx context.Context, cmd Cmd) (string, string, error) {
	c := exec.CommandContext(ctx, cmd.Name, cmd.Args...)
	prepareCmd(c)
	var outBuf, errBuf bytes.Buffer
	c.Stdout = &outBuf
	c.Stderr = &errBuf
	err := c.Run()
	return outBuf.String(), errBuf.String(), err
}

func (runner) RunStreaming(ctx context.Context, cmd Cmd, onStdout, onStderr func(line string)) (int, error) {
	c := exec.CommandContext(ctx, cmd.Name, cmd.Args...)
	prepareCmd(c)
	stdout, err := c.StdoutPipe()
	if err != nil {
		return 0, err
	}
	stderr, err := c.StderrPipe()
	if err != nil {
		return 0, err
	}

	if err := c.Start(); err != nil {
		return 0, err
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		scanLines(stdout, onStdout)
	}()
	go func() {
		defer wg.Done()
		scanLines(stderr, onStderr)
	}()

	// Drain both pipes before calling Wait: Wait closes them, and reads
	// racing with the close can lose the tail of the output (see os/exec
	// StderrPipe docs). The readers see EOF once the process exits.
	wg.Wait()
	waitErr := c.Wait()
	if waitErr != nil {
		var ee *exec.ExitError
		if errors.As(waitErr, &ee) {
			return ee.ExitCode(), fmt.Errorf("command failed (exit %d): %w", ee.ExitCode(), waitErr)
		}
		return 0, waitErr
	}
	return 0, nil
}

func scanLines(r io.Reader, cb func(line string)) {
	if cb == nil {
		_, _ = io.Copy(io.Discard, r)
		return
	}
	buf := make([]byte, 4096)
	var lineBuf bytes.Buffer
	flush := func() {
		cb(lineBuf.String())
		lineBuf.Reset()
	}
	// skipLF folds a LF directly following a CR into the same line end.
	skipLF := false
	for {
		n, readErr := r.Read(buf)
		if n > 0 {
			for _, ch := range buf[:n] {
				if skipLF {
					skipLF = false
					if ch == '\n' {
						continue
					}
				}
				switch ch {
				case '\n':
					flush()
				case '\r':
					// Bare CR is a line end too (e.g. ffmpeg status output
					// when -nostats is absent); fold a following LF.
					flush()
					skipLF = true
				default:
					lineBuf.WriteByte(ch)
				}
			}
		}
		if readErr != nil {
			if lineBuf.Len() > 0 {
				cb(lineBuf.String())
			}
			break
		}
	}
}
