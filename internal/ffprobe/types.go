package ffprobe

type ProbeResult struct {
	Streams []Stream `json:"streams"`
	Format  Format   `json:"format"`
}

type Format struct {
	Filename       string `json:"filename"`
	NBStreams      int    `json:"nb_streams"`
	NBPrograms     int    `json:"nb_programs"`
	FormatName     string `json:"format_name"`
	FormatLongName string `json:"format_long_name"`
	StartTime      string `json:"start_time"`
	Duration       string `json:"duration"`
	Size           string `json:"size"`
	BitRate        string `json:"bit_rate"`
	ProbeScore     int    `json:"probe_score"`
}

type Stream struct {
	Index             int               `json:"index"`
	CodecName         string            `json:"codec_name"`
	CodecLongName     string            `json:"codec_long_name"`
	CodecType         string            `json:"codec_type"` // "video" | "audio" | "subtitle"
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
	Disposition       map[string]any    `json:"disposition"`
	Tags              map[string]string `json:"tags"`
}
