package app

import (
	"context"
	"testing"
	"time"

	"github.com/fsx8/ffwiz/internal/execx"
	ffprobe "github.com/fsx8/ffwiz/internal/ffprobe"
)

func TestSpeedFactorFromArgs(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want float64
	}{
		{"no filter", []string{"-i", "a.mp4", "-y", "o.mp4"}, 1},
		{"2x speed", []string{"-i", "a.mp4", "-filter_complex", "[0:v]setpts=PTS/2[v];[0:a]atempo=2.0[a]", "-y", "o.mp4"}, 2},
		{"fractional speed", []string{"-filter_complex", "[0:v]setpts=PTS/1.25[v]"}, 1.25},
		{"slowdown", []string{"-filter_complex", "[0:v]setpts=PTS/0.5[v]"}, 0.5},
		{"join setpts is not speed", []string{"-filter_complex", "[0:v]scale=640:480,setpts=PTS-STARTPTS[v0]"}, 1},
		{"reverse is not speed", []string{"-filter_complex", "[0:v]reverse[v]"}, 1},
	}
	for _, c := range cases {
		if got := speedFactorFromArgs(c.args); got != c.want {
			t.Errorf("%s: speedFactor = %v, want %v", c.name, got, c.want)
		}
	}
}

// The progress bar total must account for setpts: a 2x sped-up encode only
// reaches half the input duration in ffmpeg's out_time stream.
func TestExecTotalDuration_AccountsForSpeedFactor(t *testing.T) {
	p := &fakeProber{results: map[string]*ffprobe.ProbeResult{
		"a.mp4": videoWithAudio("20"),
	}}
	args := []string{
		"-i", "a.mp4",
		"-filter_complex", "[0:v]setpts=PTS/2[v];[0:a]atempo=2.0[a]",
		"-map", "[v]", "-y", "o.mp4",
	}
	if got := execTotalDuration(context.Background(), p, args); got != 10*time.Second {
		t.Fatalf("total = %v, want 10s (half of the 20s input)", got)
	}

	slow := []string{
		"-i", "a.mp4",
		"-filter_complex", "[0:v]setpts=PTS/0.5[v]",
		"-y", "o.mp4",
	}
	if got := execTotalDuration(context.Background(), p, slow); got != 40*time.Second {
		t.Fatalf("total = %v, want 40s (0.5x doubles the output length)", got)
	}

	plain := []string{"-i", "a.mp4", "-y", "o.mp4"}
	if got := execTotalDuration(context.Background(), p, plain); got != 20*time.Second {
		t.Fatalf("total = %v, want 20s (no speed change)", got)
	}
}

func TestApplySpeedFactor(t *testing.T) {
	if got := applySpeedFactor([]string{"setpts=PTS/4[v]"}, time.Minute); got != 15*time.Second {
		t.Fatalf("applySpeedFactor(4x, 1m) = %v, want 15s", got)
	}
	if got := applySpeedFactor([]string{"-i", "a"}, time.Minute); got != time.Minute {
		t.Fatalf("applySpeedFactor(no speed, 1m) = %v, want 1m", got)
	}
	if got := applySpeedFactor([]string{"setpts=PTS/4[v]"}, 0); got != 0 {
		t.Fatalf("applySpeedFactor(4x, 0) = %v, want 0 (unknown total stays unknown)", got)
	}
}

func TestTimeFlagSeconds_TakesLastOccurrence(t *testing.T) {
	args := []string{"-ss", "5", "-i", "a.mp4", "-ss", "10", "-to", "20"}
	if s, ok := timeFlagSeconds(args, "-ss"); !ok || s != 10 {
		t.Fatalf("ss = %v %v, want 10 true (last occurrence wins)", s, ok)
	}
	if s, ok := timeFlagSeconds(args, "-to"); !ok || s != 20 {
		t.Fatalf("to = %v %v, want 20 true", s, ok)
	}
	if _, ok := timeFlagSeconds(args, "-missing"); ok {
		t.Fatal("missing flag must not be found")
	}
}

func TestProgressWidth_Clamped(t *testing.T) {
	if got := progressWidth(0); got != 20 {
		t.Fatalf("progressWidth(0) = %d, want the 20 floor", got)
	}
	if got := progressWidth(1000); got != 60 {
		t.Fatalf("progressWidth(1000) = %d, want the 60 ceiling", got)
	}
	if got := progressWidth(100); got != 60 {
		t.Fatalf("progressWidth(100) = %d, want 60", got)
	}
}

// The exec screen must not fire its context when it is already done: keys
// after completion only navigate.
func TestExecScreen_KeysAfterDoneDoNotCancel(t *testing.T) {
	m := newExecScreen(Config{}, "test", execx.Cmd{Name: "ffmpeg"})
	m.running = false
	m.done = &execDoneMsg{exitCode: 0}

	updated, cmd := m.Update(keyMsg("esc"))
	em := updated.(*execModel)
	select {
	case <-em.ctx.Done():
		t.Fatal("esc after completion must not cancel the context")
	default:
	}
	if cmd == nil {
		t.Fatal("esc after completion should pop the screen")
	}
	if !hasMsg[popMsg](msgsOf(cmd)) {
		t.Fatal("esc after completion should pop the screen")
	}
}

func TestExecScreen_EscWhileRunningCancels(t *testing.T) {
	m := newExecScreen(Config{}, "test", execx.Cmd{Name: "ffmpeg"})
	m.running = true // Init sets this; set it directly to skip the goroutines
	m.Update(keyMsg("esc"))
	select {
	case <-m.ctx.Done():
	default:
		t.Fatal("esc while running must cancel the exec context")
	}
}
