package ffprobe

import (
	"encoding/json"
	"testing"
)

func TestProbeResultJSONUnmarshal(t *testing.T) {
	const sample = `{
  "streams": [
    {"index": 0, "codec_name": "h264", "codec_type": "video", "width": 1920, "height": 1080, "r_frame_rate": "30000/1001", "sample_aspect_ratio": "1:1", "codec_tag_string": "avc1", "codec_tag": "0x31637661"},
    {"index": 2, "codec_name": "aac", "codec_type": "audio", "sample_rate": "48000", "channels": 2, "tags": {"language": "eng"}}
  ],
  "format": {"format_long_name": "Matroska / WebM", "duration": "12.34", "size": "123456", "bit_rate": "1000000"}
}`

	var res ProbeResult
	if err := json.Unmarshal([]byte(sample), &res); err != nil {
		t.Fatal(err)
	}
	if res.Format.FormatLongName != "Matroska / WebM" {
		t.Fatalf("unexpected format: %#v", res.Format)
	}
	if len(res.Streams) != 2 {
		t.Fatalf("unexpected streams: %#v", res.Streams)
	}
	if res.Streams[0].CodecType != "video" || res.Streams[0].Width != 1920 {
		t.Fatalf("unexpected video stream: %#v", res.Streams[0])
	}
	if res.Streams[0].CodecTagString != "avc1" {
		t.Fatalf("codec_tag_string = %q, want avc1", res.Streams[0].CodecTagString)
	}
	if res.Streams[1].CodecType != "audio" || res.Streams[1].SampleRate != "48000" {
		t.Fatalf("unexpected audio stream: %#v", res.Streams[1])
	}
}

func TestProbeResultHelpers(t *testing.T) {
	res := &ProbeResult{
		Streams: []Stream{
			{Index: 0, CodecType: "video", CodecName: "h264", Disposition: map[string]any{"attached_pic": float64(0)}},
			{Index: 1, CodecType: "audio", CodecName: "aac"},
			{Index: 2, CodecType: "video", CodecName: "mjpeg", Disposition: map[string]any{"attached_pic": float64(1)}},
			{Index: 3, CodecType: "subtitle", CodecName: "subrip"},
		},
		Format: Format{Duration: "12.5"},
	}

	if !res.HasAudio() || !res.HasVideo() {
		t.Fatal("expected audio and (playable) video")
	}
	if got := len(res.VideoStreams()); got != 1 {
		t.Fatalf("VideoStreams = %d, want 1 (cover art excluded)", got)
	}
	if s := res.FirstVideo(); s == nil || s.CodecName != "h264" {
		t.Fatalf("FirstVideo = %+v, want the h264 stream", s)
	}
	if got := len(res.StreamsOfType("subtitle")); got != 1 {
		t.Fatalf("StreamsOfType(subtitle) = %d, want 1", got)
	}
	if d, ok := res.Duration(); !ok || d != 12.5 {
		t.Fatalf("Duration = %v %v, want 12.5 true", d, ok)
	}

	coverOnly := &ProbeResult{Streams: []Stream{
		{CodecType: "video", Disposition: map[string]any{"attached_pic": float64(1)}},
	}}
	if coverOnly.HasVideo() {
		t.Fatal("cover-art-only input must not count as video")
	}
	if s := coverOnly.FirstVideo(); s != nil {
		t.Fatalf("FirstVideo must be nil for cover-art-only input, got %+v", s)
	}

	var nilRes *ProbeResult
	if _, ok := nilRes.Duration(); ok {
		t.Fatal("nil result must report no duration")
	}
	if nilRes.HasAudio() || nilRes.HasVideo() {
		t.Fatal("nil result must report no streams")
	}

	if !(Stream{Disposition: map[string]any{"attached_pic": true}}).IsAttachedPic() {
		t.Fatal("bool disposition values must be recognized")
	}
}

// ffprobe emits "N/A" for unknown durations; such values (and anything
// non-positive) must read as "no usable duration", not an error or 0s.
func TestProbeResultDurationHandlesNA(t *testing.T) {
	cases := []struct {
		raw    string
		want   float64
		wantOK bool
	}{
		{"12.5", 12.5, true},
		{"N/A", 0, false},
		{"", 0, false},
		{" 7 ", 7, true},
		{"0", 0, false},
		{"-3", 0, false},
		{"junk", 0, false},
	}
	for _, c := range cases {
		res := &ProbeResult{Format: Format{Duration: c.raw}}
		got, ok := res.Duration()
		if ok != c.wantOK || got != c.want {
			t.Fatalf("Duration(%q) = %v %v, want %v %v", c.raw, got, ok, c.want, c.wantOK)
		}
	}
}
