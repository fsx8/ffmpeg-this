package execx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
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
	RunStreaming(ctx context.Context, cmd Cmd, onStderr func(line string)) (exitCode int, err error)
}

type runner struct{}

func New() Runner { return runner{} }

func (runner) Run(ctx context.Context, cmd Cmd) (string, string, error) {
	c := exec.CommandContext(ctx, cmd.Name, cmd.Args...)
	setProcessGroup(c)
	var outBuf, errBuf bytes.Buffer
	c.Stdout = &outBuf
	c.Stderr = &errBuf
	err := c.Run()
	return outBuf.String(), errBuf.String(), err
}

func (runner) RunStreaming(ctx context.Context, cmd Cmd, onStderr func(line string)) (int, error) {
	c := exec.CommandContext(ctx, cmd.Name, cmd.Args...)
	setProcessGroup(c)
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
		_, _ = io.Copy(io.Discard, stdout)
	}()
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		var lineBuf bytes.Buffer
		for {
			n, readErr := stderr.Read(buf)
			if n > 0 {
				for _, ch := range buf[:n] {
					if ch == '\n' {
						onStderr(lineBuf.String())
						lineBuf.Reset()
						continue
					}
					_ = lineBuf.WriteByte(ch)
				}
			}
			if readErr != nil {
				if lineBuf.Len() > 0 {
					onStderr(lineBuf.String())
				}
				break
			}
		}
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
