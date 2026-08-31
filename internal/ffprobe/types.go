package ffprobe

import (
	"strconv"
	"strings"
)

type ProbeResult struct {
	Streams []Stream `json:"streams"`
	Format  Format   `json:"format"`
}

type Format struct {
	Filename       string            `json:"filename"`
	NBStreams      int               `json:"nb_streams"`
	NBPrograms     int               `json:"nb_programs"`
	FormatName     string            `json:"format_name"`
	FormatLongName string            `json:"format_long_name"`
	StartTime      string            `json:"start_time"`
	Duration       string            `json:"duration"`
	Size           string            `json:"size"`
	BitRate        string            `json:"bit_rate"`
	ProbeScore     int               `json:"probe_score"`
	Tags           map[string]string `json:"tags"`
}

type Stream struct {
	Index             int               `json:"index"`
	CodecName         string            `json:"codec_name"`
	CodecLongName     string            `json:"codec_long_name"`
	CodecType         string            `json:"codec_type"` // "video" | "audio" | "subtitle"
	CodecTagString    string            `json:"codec_tag_string"`
	Width             int               `json:"width"`
	Height            int               `json:"height"`
	RFrameRate        string            `json:"r_frame_rate"`
	SampleRate        string            `json:"sample_rate"`
	Channels          int               `json:"channels"`
	BitRate           string            `json:"bit_rate"`
	Duration          string            `json:"duration"`
	SampleAspectRatio string            `json:"sample_aspect_ratio"`
	Profile           string            `json:"profile"`
	Level             int               `json:"level"`
	PixFmt            string            `json:"pix_fmt"`
	ColorTransfer     string            `json:"color_transfer"`
	ColorPrimaries    string            `json:"color_primaries"`
	ColorSpace        string            `json:"color_space"`
	Disposition       map[string]any    `json:"disposition"`
	Tags              map[string]string `json:"tags"`
}

// Duration returns the container duration in seconds; ok is false when the
// probe carries no usable duration.
func (r *ProbeResult) Duration() (float64, bool) {
	if r == nil {
		return 0, false
	}
	d, err := strconv.ParseFloat(strings.TrimSpace(r.Format.Duration), 64)
	if err != nil || d <= 0 {
		return 0, false
	}
	return d, true
}

// StreamsOfType returns every stream with the given codec_type.
func (r *ProbeResult) StreamsOfType(codecType string) []Stream {
	if r == nil {
		return nil
	}
	var out []Stream
	for _, s := range r.Streams {
		if s.CodecType == codecType {
			out = append(out, s)
		}
	}
	return out
}

// HasAudio reports whether the file carries at least one audio stream.
func (r *ProbeResult) HasAudio() bool {
	return len(r.StreamsOfType("audio")) > 0
}

// VideoStreams returns the video streams, excluding attached pictures
// (embedded cover art), which are not playable video.
func (r *ProbeResult) VideoStreams() []Stream {
	if r == nil {
		return nil
	}
	var out []Stream
	for _, s := range r.Streams {
		if s.CodecType == "video" && !s.IsAttachedPic() {
			out = append(out, s)
		}
	}
	return out
}

// HasVideo reports whether the file carries playable video (cover art does
// not count).
func (r *ProbeResult) HasVideo() bool {
	return len(r.VideoStreams()) > 0
}

// FirstVideo returns the first playable video stream, or nil.
func (r *ProbeResult) FirstVideo() *Stream {
	for i := range r.Streams {
		s := &r.Streams[i]
		if s.CodecType == "video" && !s.IsAttachedPic() {
			return s
		}
	}
	return nil
}

// IsAttachedPic reports whether the stream is an embedded cover art picture
// (disposition.attached_pic); such streams show up as "video" in ffprobe
// output but must not be treated as playable video.
func (s Stream) IsAttachedPic() bool {
	switch v := s.Disposition["attached_pic"].(type) {
	case float64:
		return v != 0
	case int:
		return v != 0
	case bool:
		return v
	}
	return false
}
