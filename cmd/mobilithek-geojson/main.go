package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/karte-bayern/mobilithek"
)

func main() {
	log.SetFlags(0)

	in := flag.String("in", "", "input DATEX II XML file")
	out := flag.String("out", "-", "output GeoJSON file, or - for stdout")
	source := flag.String("source", "", "source name stored in GeoJSON properties")
	compact := flag.Bool("compact", false, "write compact JSON instead of indented JSON")
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

	geojson, err := mobilithek.GeoJSONFromDATEX2XML(body, sourceName)
	if err != nil {
		log.Fatal(err)
	}

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
