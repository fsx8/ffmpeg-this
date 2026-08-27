//go:build unix

package execx

import (
	"context"
	"testing"
	"time"
)

func TestRunCapturesStdoutAndStderr(t *testing.T) {
	out, errOut, err := New().Run(context.Background(), Cmd{
		Name: "sh", Args: []string{"-c", "echo out; echo err >&2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "out\n" {
		t.Fatalf("stdout: %q", out)
	}
	if errOut != "err\n" {
		t.Fatalf("stderr: %q", errOut)
	}
}

func TestRunStreamingEmitsStderrLinesOnly(t *testing.T) {
	var lines []string
	exitCode, err := New().RunStreaming(context.Background(),
		Cmd{Name: "sh", Args: []string{"-c", "echo ignored-stdout; echo one >&2; echo two >&2; printf 'partial' >&2"}},
		func(line string) { lines = append(lines, line) },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exit code: %d", exitCode)
	}
	want := []string{"one", "two", "partial"}
	if len(lines) != len(want) {
		t.Fatalf("lines: %#v", lines)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("lines: got %#v want %#v", lines, want)
		}
	}
}

func TestRunStreamingReportsExitCode(t *testing.T) {
	exitCode, err := New().RunStreaming(context.Background(),
		Cmd{Name: "sh", Args: []string{"-c", "echo boom >&2; exit 3"}},
		func(string) {},
	)
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
	if exitCode != 3 {
		t.Fatalf("exit code: got %d want 3", exitCode)
	}
}

func TestRunStreamingMissingBinary(t *testing.T) {
	if _, err := New().RunStreaming(context.Background(),
		Cmd{Name: "definitely-not-a-real-binary-xyz"}, func(string) {},
	); err == nil {
		t.Fatal("expected error for missing binary")
	}
}

func TestRunStreamingCancelKillsProcess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	exitCode, err := New().RunStreaming(ctx,
		Cmd{Name: "sh", Args: []string{"-c", "sleep 30"}},
		func(string) {},
	)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected error after cancellation")
	}
	if elapsed > 5*time.Second {
		t.Fatalf("cancellation took too long: %v", elapsed)
	}
	if exitCode >= 0 {
		t.Fatalf("expected negative exit code after kill, got %d", exitCode)
	}
}
