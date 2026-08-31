package ffmpeg

import "testing"

func TestScreenshotOutputName(t *testing.T) {
	if got, want := ScreenshotOutputName("/a/b/clip.mp4", "00:05:00", "png"), "clip_frame_00-05-00.png"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if got, want := ScreenshotOutputName("clip.mp4", "12.5", "jpg"), "clip_frame_12-5.jpg"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestBuildScreenshotCmd(t *testing.T) {
	cmd := BuildScreenshotCmd("in.mp4", "00:00:05", "out.png")
	want := []string{"-ss", "00:00:05", "-i", "in.mp4", "-frames:v", "1", "-y", "out.png"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("args = %#v, want %#v", cmd.Args, want)
	}
	for i := range want {
		if cmd.Args[i] != want[i] {
			t.Fatalf("args = %#v, want %#v", cmd.Args, want)
		}
	}
}
