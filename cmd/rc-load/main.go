package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	base := strings.TrimRight(env("RC_API_URL", "http://localhost:8080"), "/")
	requests := envInt("RC_LOAD_REQUESTS", 100)
	concurrency := envInt("RC_LOAD_CONCURRENCY", 8)
	query := env("RC_LOAD_QUERY", "4254.97")
	endpoint := base + "/v1/network/search?q=" + url.QueryEscape(query) + "&limit=20"
	client := &http.Client{Timeout: 10 * time.Second}

	jobs := make(chan int)
	latencies := make(chan time.Duration, requests)
	var failures atomic.Int64
	var wg sync.WaitGroup

	start := time.Now()
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				t0 := time.Now()
				resp, err := client.Get(endpoint)
				latencies <- time.Since(t0)
				if err != nil {
					failures.Add(1)
					continue
				}
				_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
				_ = resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					failures.Add(1)
				}
			}
		}()
	}
	for i := 0; i < requests; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	close(latencies)

	values := make([]time.Duration, 0, requests)
	for d := range latencies {
		values = append(values, d)
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	duration := time.Since(start)
	failed := failures.Load()

	fmt.Printf("RC load endpoint=%s\n", endpoint)
	fmt.Printf("requests=%d concurrency=%d duration=%s rps=%.1f failures=%d\n", requests, concurrency, duration.Round(time.Millisecond), float64(requests)/duration.Seconds(), failed)
	fmt.Printf("p50=%s p95=%s p99=%s max=%s\n", percentile(values, 50), percentile(values, 95), percentile(values, 99), percentile(values, 100))

	maxFailurePct := envFloat("RC_LOAD_MAX_FAILURE_PCT", 1)
	maxP95MS := envFloat("RC_LOAD_MAX_P95_MS", 750)
	failurePct := float64(failed) * 100 / float64(requests)
	p95ms := float64(percentile(values, 95).Microseconds()) / 1000
	if failurePct > maxFailurePct || p95ms > maxP95MS {
		fmt.Fprintf(os.Stderr, "FAIL thresholds: failure_pct=%.2f/%.2f p95_ms=%.1f/%.1f\n", failurePct, maxFailurePct, p95ms, maxP95MS)
		os.Exit(1)
	}
	fmt.Println("PASS load baseline")
}

func percentile(values []time.Duration, p int) time.Duration {
	if len(values) == 0 {
		return 0
	}
	if p <= 0 {
		return values[0]
	}
	if p >= 100 {
		return values[len(values)-1]
	}
	idx := (len(values)*p + 99) / 100
	if idx < 1 {
		idx = 1
	}
	return values[idx-1]
}

func env(name, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return fallback
}

func envInt(name string, fallback int) int {
	v, err := strconv.Atoi(env(name, strconv.Itoa(fallback)))
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func envFloat(name string, fallback float64) float64 {
	v, err := strconv.ParseFloat(env(name, fmt.Sprint(fallback)), 64)
	if err != nil || v < 0 {
		return fallback
	}
	return v
}
