// Command go-uptime-checker checks the availability of URLs concurrently
// and produces a report in text, JSON, or CSV format.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/yourusername/go-uptime-checker/checker"
	"github.com/yourusername/go-uptime-checker/report"
)

func main() {
	var (
		file        = flag.String("file", "", "file with one URL per line (use '-' for stdin)")
		format      = flag.String("format", "text", "output format: text|json|csv")
		concurrency = flag.Int("concurrency", 20, "max concurrent requests")
		timeout     = flag.Duration("timeout", 5*time.Second, "per-request timeout")
		rateLimit   = flag.Float64("rate", 0, "max requests started per second (0 = unlimited)")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] URL [URL...]\n\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	urls, err := collectURLs(*file, flag.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(urls) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	c := checker.New(checker.Options{
		Concurrency: *concurrency,
		Timeout:     *timeout,
		RateLimit:   *rateLimit,
	})

	results := c.Check(ctx, urls)
	summary := report.Build(results)

	var writeErr error
	switch strings.ToLower(*format) {
	case "text":
		writeErr = report.WriteText(os.Stdout, summary)
	case "json":
		writeErr = report.WriteJSON(os.Stdout, summary)
	case "csv":
		writeErr = report.WriteCSV(os.Stdout, summary)
	default:
		fmt.Fprintf(os.Stderr, "unknown format %q\n", *format)
		os.Exit(2)
	}
	if writeErr != nil {
		fmt.Fprintf(os.Stderr, "write error: %v\n", writeErr)
		os.Exit(1)
	}

	// Exit code: 1 if any URL is down.
	for _, r := range results {
		if !r.Up {
			os.Exit(1)
		}
	}
}

// collectURLs gathers URLs from an optional file (or stdin with "-") and any
// positional args. Lines starting with '#' and blank lines are ignored.
func collectURLs(file string, args []string) ([]string, error) {
	var urls []string
	if file != "" {
		var f *os.File
		var err error
		if file == "-" {
			f = os.Stdin
		} else {
			f, err = os.Open(file)
			if err != nil {
				return nil, err
			}
			defer f.Close()
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			urls = append(urls, line)
		}
		if err := sc.Err(); err != nil {
			return nil, err
		}
	}
	urls = append(urls, args...)
	return urls, nil
}