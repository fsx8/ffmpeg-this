package ffmpeg

import "testing"

func TestParseTimeSpec(t *testing.T) {
	cases := []struct {
		in      string
		want    float64
		wantErr bool
	}{
		{"90", 90, false},
		{"90.5", 90.5, false},
		{"1:30", 90, false},
		{"01:00:10", 3610, false},
		{"1:2:3", 3723, false},
		{"00:00:10.25", 10.25, false},
		{"  12  ", 12, false},
		{"", 0, true},
		{"abc", 0, true},
		{"1:xx", 0, true},
		{"-5", 0, true},
		{"1:60", 120, false}, // lenient like ffmpeg: rolls over
		{"1:1:1:1", 0, true},
		{"1.5:30", 0, true},
	}
	for _, c := range cases {
		got, err := ParseTimeSpec(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseTimeSpec(%q): expected error, got %v", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseTimeSpec(%q): unexpected error %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseTimeSpec(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestValidateTrim(t *testing.T) {
	if err := ValidateTrim("00:00:01", "00:00:10"); err != nil {
		t.Fatalf("valid range rejected: %v", err)
	}
	if err := ValidateTrim("10", "1"); err == nil {
		t.Fatal("start after end accepted")
	}
	if err := ValidateTrim("10", "10"); err == nil {
		t.Fatal("empty range accepted")
	}
	if err := ValidateTrim("nope", "10"); err == nil {
		t.Fatal("invalid start accepted")
	}
	if err := ValidateTrim("0", "bad"); err == nil {
		t.Fatal("invalid end accepted")
	}
}
