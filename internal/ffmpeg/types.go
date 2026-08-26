package ffmpeg

type Cmd struct {
	Args []string
}

func (c Cmd) FullArgs() []string {
	out := make([]string, 0, 1+len(c.Args))
	out = append(out, "ffmpeg")
	out = append(out, c.Args...)
	return out
}

type TrackType string

const (
	TrackVideo    TrackType = "video"
	TrackAudio    TrackType = "audio"
	TrackSubtitle TrackType = "subtitle"
)

type Track struct {
	Index int
	Type  TrackType
	Codec string
}

type TrackAction string

const (
	ActionRemove  TrackAction = "remove"
	ActionKeep    TrackAction = "keep"
	ActionConvert TrackAction = "convert"
)

type TrackActionInfo struct {
	Action TrackAction
	Codec  string // for convert
}
