package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yourusername/go-uptime-checker/checker"
)

func sampleResults() []checker.Result {
	return []checker.Result{
		{URL: "https://a.example", Up: true, StatusCode: 200, Duration: 10 * time.Millisecond},
		{URL: "https://b.example", Up: false, StatusCode: 500, Duration: 20 * time.Millisecond, Error: "unexpected status: 500"},
	}
}

func TestBuild(t *testing.T) {
	s := Build(sampleResults())
	if s.Total != 2 || s.Up != 1 || s.Down != 1 {
		t.Fatalf("bad counts: %+v", s)
	}
	if s.TotalTime != 30*time.Millisecond {
		t.Fatalf("bad total time: %v", s.TotalTime)
	}
}

func TestWriteText(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteText(&buf, Build(sampleResults())); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "https://a.example") || !strings.Contains(out, "https://b.example") {
		t.Fatalf("missing urls: %s", out)
	}
	if !strings.Contains(out, "UP") || !strings.Contains(out, "DOWN") {
		t.Fatalf("missing statuses: %s", out)
	}
}

func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteJSON(&buf, Build(sampleResults())); err != nil {
		t.Fatal(err)
	}
	var got Summary
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Total != 2 || len(got.Results) != 2 {
		t.Fatalf("bad json: %+v", got)
	}
}

func TestWriteCSV(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteCSV(&buf, Build(sampleResults())); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (header+2), got %d: %s", len(lines), buf.String())
	}
	if !strings.HasPrefix(lines[0], "url,up,status_code,duration_ms,error") {
		t.Fatalf("bad header: %s", lines[0])
	}
}