// Command mobilithek-autobahn fetches data from the public Autobahn API
// (verkehr.autobahn.de) — a second, independent open-data source alongside
// Mobilithek subscriptions — and converts it to GeoJSON using the same
// Event model, so it can be viewed with the bundled MapLibre demo or merged
// with Mobilithek-derived data via mobilithek.MergeGeoJSON.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/karte-bayern/mobilithek"
)

func main() {
	log.SetFlags(0)

	roadsFlag := flag.String("roads", "", "comma-separated Autobahn road IDs (e.g. A9,A92), or 'all' for every road the API currently lists")
	servicesFlag := flag.String("services", string(mobilithek.AutobahnRoadworks), "comma-separated service kinds (roadworks, warning, closure, electric_charging_station, parking_lorryparkingfeatures), or 'all'")
	out := flag.String("out", "-", "output GeoJSON file, or - for stdout")
	compact := flag.Bool("compact", false, "write compact JSON instead of indented JSON")
	baseURL := flag.String("base-url", mobilithek.DefaultAutobahnBaseURL, "Autobahn API base URL")
	timeout := flag.Duration("timeout", 30*time.Second, "per-request timeout")
	overallTimeout := flag.Duration("overall-timeout", 5*time.Minute, "timeout for the whole fetch, across every road and service")
	maxBytes := flag.Int64("max-bytes", 50*1024*1024, "maximum response size per request")
	concurrency := flag.Int("concurrency", 4, "maximum concurrent requests against the Autobahn API")
	flag.Parse()

	if strings.TrimSpace(*roadsFlag) == "" {
		log.Fatal("missing -roads (comma-separated road IDs, or 'all')")
	}
	if *concurrency < 1 {
		log.Fatal("-concurrency must be at least 1")
	}

	client, err := mobilithek.NewAutobahnClient(
		mobilithek.WithAutobahnBaseURL(*baseURL),
		mobilithek.WithAutobahnTimeout(*timeout),
		mobilithek.WithAutobahnMaxBytes(*maxBytes),
	)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *overallTimeout)
	defer cancel()

	roads, err := resolveRoads(ctx, client, *roadsFlag)
	if err != nil {
		log.Fatal(err)
	}
	services, err := resolveServices(*servicesFlag)
	if err != nil {
		log.Fatal(err)
	}

	events, failures := fetchAll(ctx, client, roads, services, *concurrency)
	for _, failure := range failures {
		fmt.Fprintf(os.Stderr, "warning: %v\n", failure)
	}

	totalRequests := len(roads) * len(services)
	if len(events) == 0 && len(failures) > 0 {
		log.Fatalf("all %d requests failed", totalRequests)
	}

	geojson := mobilithek.EventsToGeoJSON(events)
	output, err := mobilithek.MarshalGeoJSON(geojson, !*compact)
	if err != nil {
		log.Fatal(err)
	}

	if *out == "-" {
		_, _ = os.Stdout.Write(output)
		fmt.Fprintf(os.Stderr, "%d features, %d roads, %d/%d requests failed\n", len(geojson.Features), len(roads), len(failures), totalRequests)
		return
	}

	if dir := filepath.Dir(*out); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatal(err)
		}
	}
	if err := os.WriteFile(*out, output, 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%d features, %d roads, %d/%d requests failed)\n", *out, len(geojson.Features), len(roads), len(failures), totalRequests)
}

func resolveRoads(ctx context.Context, client *mobilithek.AutobahnClient, raw string) ([]string, error) {
	if strings.EqualFold(strings.TrimSpace(raw), "all") {
		roads, err := client.Roads(ctx)
		if err != nil {
			return nil, fmt.Errorf("list roads: %w", err)
		}
		return roads, nil
	}
	roads := splitNonEmpty(raw)
	if len(roads) == 0 {
		return nil, errors.New("no road IDs given")
	}
	return roads, nil
}

func resolveServices(raw string) ([]mobilithek.AutobahnServiceKind, error) {
	if strings.EqualFold(strings.TrimSpace(raw), "all") {
		return mobilithek.AllAutobahnServiceKinds(), nil
	}
	names := splitNonEmpty(raw)
	if len(names) == 0 {
		return nil, errors.New("no service kinds given")
	}
	kinds := make([]mobilithek.AutobahnServiceKind, 0, len(names))
	for _, name := range names {
		kinds = append(kinds, mobilithek.AutobahnServiceKind(name))
	}
	return kinds, nil
}

func splitNonEmpty(raw string) []string {
	parts := strings.Split(raw, ",")
	output := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			output = append(output, part)
		}
	}
	return output
}

type fetchJob struct {
	road string
	kind mobilithek.AutobahnServiceKind
}

// fetchAll fetches every (road, service) combination with bounded
// concurrency so a large -roads/-services selection doesn't hammer the
// public API with hundreds of simultaneous requests. A failure on one job
// is collected and reported, not fatal to the others.
func fetchAll(ctx context.Context, client *mobilithek.AutobahnClient, roads []string, services []mobilithek.AutobahnServiceKind, concurrency int) ([]mobilithek.Event, []error) {
	jobs := make([]fetchJob, 0, len(roads)*len(services))
	for _, road := range roads {
		for _, kind := range services {
			jobs = append(jobs, fetchJob{road: road, kind: kind})
		}
	}

	results := make([][]mobilithek.Event, len(jobs))
	errs := make([]error, len(jobs))

	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for index, job := range jobs {
		wg.Add(1)
		go func(index int, job fetchJob) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			events, err := client.FetchService(ctx, job.road, job.kind)
			if err != nil {
				errs[index] = fmt.Errorf("road %s service %s: %w", job.road, job.kind, err)
				return
			}
			results[index] = events
		}(index, job)
	}
	wg.Wait()

	events := []mobilithek.Event{}
	failures := []error{}
	for index := range jobs {
		if errs[index] != nil {
			failures = append(failures, errs[index])
			continue
		}
		events = append(events, results[index]...)
	}
	return events, failures
}
