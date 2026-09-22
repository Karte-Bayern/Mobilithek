package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/karte-bayern/mobilithek"
)

func main() {
	log.SetFlags(0)

	in := flag.String("in", "", "input DATEX II XML file")
	out := flag.String("out", "-", "output GeoJSON file, or - for stdout")
	source := flag.String("source", "", "source name stored in GeoJSON properties")
	compact := flag.Bool("compact", false, "write compact JSON instead of indented JSON")
	bboxFlag := flag.String("bbox", "", "keep only events with a coordinate inside this box: west,south,east,north")
	statusFlag := flag.String("status", "", "keep only events with this comma-separated Status (e.g. current,future)")
	flag.Parse()

	if *in == "" {
		log.Fatal("missing -in XML file")
	}

	body, err := os.ReadFile(*in)
	if err != nil {
		log.Fatal(err)
	}

	sourceName := *source
	if sourceName == "" {
		sourceName = filepath.Base(*in)
	}

	events, err := mobilithek.ExtractEventsFromDATEX2XML(body, sourceName)
	if err != nil {
		log.Fatal(err)
	}

	if *bboxFlag != "" {
		bbox, err := parseBBox(*bboxFlag)
		if err != nil {
			log.Fatal(err)
		}
		events = mobilithek.FilterEventsByBBox(events, bbox)
	}
	if *statusFlag != "" {
		events = mobilithek.FilterEventsByStatus(events, strings.Split(*statusFlag, ",")...)
	}

	geojson := mobilithek.EventsToGeoJSON(events)

	output, err := mobilithek.MarshalGeoJSON(geojson, !*compact)
	if err != nil {
		log.Fatal(err)
	}

	if *out == "-" {
		_, _ = os.Stdout.Write(output)
		return
	}

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil && filepath.Dir(*out) != "." {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, output, 0o644); err != nil {
		log.Fatal(err)
	}

	fmt.Fprintf(os.Stderr, "wrote %s (%d features)\n", *out, len(geojson.Features))
}

func parseBBox(raw string) (mobilithek.BBox, error) {
	parts := strings.Split(raw, ",")
	if len(parts) != 4 {
		return mobilithek.BBox{}, fmt.Errorf("invalid -bbox %q: want west,south,east,north", raw)
	}

	values := make([]float64, 4)
	for i, part := range parts {
		value, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return mobilithek.BBox{}, fmt.Errorf("invalid -bbox %q: %w", raw, err)
		}
		values[i] = value
	}

	return mobilithek.BBox{West: values[0], South: values[1], East: values[2], North: values[3]}, nil
}
