package ffprobe

import (
	"encoding/json"
	"testing"
)

func TestProbeResultJSONUnmarshal(t *testing.T) {
	const sample = `{
  "streams": [
    {"index": 0, "codec_name": "h264", "codec_type": "video", "width": 1920, "height": 1080, "r_frame_rate": "30000/1001", "sample_aspect_ratio": "1:1"},
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
	if res.Streams[1].CodecType != "audio" || res.Streams[1].SampleRate != "48000" {
		t.Fatalf("unexpected audio stream: %#v", res.Streams[1])
	}
}
