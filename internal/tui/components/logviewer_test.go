package components

import "testing"

func TestExtractTimestamp(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		wantDate   string
		wantTime   string
		wantRest   string
	}{
		{
			name:     "fractional seconds",
			in:       "2026-04-04T17:01:15.5264109Z hello world",
			wantDate: "2026-04-04",
			wantTime: "17:01:15",
			wantRest: "hello world",
		},
		{
			name:     "whole seconds",
			in:       "2026-04-04T17:01:15Z message",
			wantDate: "2026-04-04",
			wantTime: "17:01:15",
			wantRest: "message",
		},
		{
			name:     "no timestamp",
			in:       "just a plain line",
			wantDate: "",
			wantTime: "",
			wantRest: "just a plain line",
		},
		{
			name:     "empty input",
			in:       "",
			wantDate: "",
			wantTime: "",
			wantRest: "",
		},
		{
			name:     "timestamp not at start is not consumed",
			in:       "foo 2026-04-04T17:01:15Z bar",
			wantDate: "",
			wantTime: "",
			wantRest: "foo 2026-04-04T17:01:15Z bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDate, gotTime, gotRest := extractTimestamp(tt.in)
			if gotDate != tt.wantDate {
				t.Errorf("date = %q, want %q", gotDate, tt.wantDate)
			}
			if gotTime != tt.wantTime {
				t.Errorf("time = %q, want %q", gotTime, tt.wantTime)
			}
			if gotRest != tt.wantRest {
				t.Errorf("rest = %q, want %q", gotRest, tt.wantRest)
			}
		})
	}
}

func TestLogViewerParseContent(t *testing.T) {
	lv := NewLogViewer()
	content := "job1\tstep1\t2026-04-04T17:01:15Z first line\n" +
		"job1\tstep1\t2026-04-04T17:01:16Z second line\n" +
		"\n" +
		"job2\tstep1\t2026-04-04T17:02:00Z third line"
	lv.SetContent("test", content)

	if got, want := len(lv.lines), 4; got != want {
		t.Fatalf("line count = %d, want %d", got, want)
	}
	if got, want := lv.logDate, "2026-04-04"; got != want {
		t.Errorf("logDate = %q, want %q", got, want)
	}
	if got, want := lv.lines[0].message, "first line"; got != want {
		t.Errorf("line[0].message = %q, want %q", got, want)
	}
	if got, want := lv.lines[0].section, "job1 / step1"; got != want {
		t.Errorf("line[0].section = %q, want %q", got, want)
	}
	// Empty line should carry the current section forward.
	if got, want := lv.lines[2].section, "job1 / step1"; got != want {
		t.Errorf("line[2].section = %q, want %q (empty line should inherit)", got, want)
	}
	if got, want := lv.lines[3].section, "job2 / step1"; got != want {
		t.Errorf("line[3].section = %q, want %q", got, want)
	}
}
