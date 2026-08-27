# ffwiz demo screencast — regenerate with:  vhs docs/demo.tape
# Prerequisites: `go build -o ffwiz ./cmd/ffwiz` and a local ffmpeg/ffprobe
# (the setup block below builds and generates the sample clip itself).
# NOTE: keystrokes below match the current menu order in internal/app
# (main menu, action menu, track editor). If menus are reordered, adjust.

Output docs/demo.gif
Set Shell bash
Set Theme "Catppuccin Mocha"
Set FontSize 20
Set Width 1200
Set Height 760
Set Padding 30

Hide
Type "go build -o ffwiz ./cmd/ffwiz && ./scripts/testmedia.sh testmedia && cp testmedia/basic.mp4 /tmp/ffwiz-demo.mp4"
Enter
Sleep 20s
Show

# Launch ffwiz straight into the action menu for the sample clip.
Type "./ffwiz /tmp/ffwiz-demo.mp4"
Enter
Sleep 1500ms

# --- Action menu: Inspect File Details ---
Enter
Sleep 1500ms
Escape
Sleep 300ms

# --- Action menu: Modify Tracks (one item down) ---
Down
Enter
Sleep 800ms

# --- Track editor: re-encode BOTH tracks so the progress bar has work ---
# The first (video) entry starts selected: convert to x264.
Type "c"
Sleep 600ms
Enter
Sleep 600ms
Down
Type "c"
Sleep 600ms
# Audio codec picker: accept the highlighted default (aac); this returns
# to the track list.
Enter
Sleep 600ms
# Continue from the track list to the output-name step.
Enter
Sleep 800ms
# Accept the suggested `<name>_modified.ext` → confirmation screen.
Enter
Sleep 2500ms

# --- Execution view with the live progress bar ---
Enter
Sleep 8000ms

# Back out of the exec and tracks screens, then quit.
Escape
Sleep 400ms
Escape
Sleep 400ms
Type "q"
