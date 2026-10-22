// Package report renders availability results as text, JSON, or CSV.
package report

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/yourusername/go-uptime-checker/checker"
)

// Summary aggregates a set of results.
type Summary struct {
	Total     int           `json:"total"`
	Up        int           `json:"up"`
	Down      int           `json:"down"`
	TotalTime time.Duration `json:"total_time"`
	Timestamp time.Time     `json:"timestamp"`
	Results   []checker.Result `json:"results"`
}

// Build computes a Summary from results.
func Build(results []checker.Result) Summary {
	s := Summary{
		Timestamp: time.Now().UTC(),
		Results:   results,
		Total:     len(results),
	}
	for _, r := range results {
		if r.Up {
			s.Up++
		} else {
			s.Down++
		}
		s.TotalTime += r.Duration
	}
	return s
}

// WriteText writes a human-readable table.
func WriteText(w io.Writer, s Summary) error {
	if _, err := fmt.Fprintf(w, "Uptime Report — %s\n", s.Timestamp.Format(time.RFC3339)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Total: %d  Up: %d  Down: %d  Cumulative: %s\n\n",
		s.Total, s.Up, s.Down, s.TotalTime.Round(time.Millisecond)); err != nil {
		return err
	}
	for _, r := range s.Results {
		status := "UP  "
		if !r.Up {
			status = "DOWN"
		}
		line := fmt.Sprintf("[%s] %-4d %-40s %s",
			status, r.StatusCode, r.URL, r.Duration.Round(time.Millisecond))
		if r.Error != "" {
			line += "  (" + r.Error + ")"
		}
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}

// WriteJSON writes the summary as indented JSON.
func WriteJSON(w io.Writer, s Summary) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(s)
}

// WriteCSV writes the results as CSV.
func WriteCSV(w io.Writer, s Summary) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()
	if err := cw.Write([]string{"url", "up", "status_code", "duration_ms", "error"}); err != nil {
		return err
	}
	for _, r := range s.Results {
		rec := []string{
			r.URL,
			fmt.Sprintf("%t", r.Up),
			fmt.Sprintf("%d", r.StatusCode),
			fmt.Sprintf("%d", r.Duration.Milliseconds()),
			r.Error,
		}
		if err := cw.Write(rec); err != nil {
			return err
		}
	}
	return cw.Error()
}