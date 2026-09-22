package main

import (
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	if err := mime.AddExtensionType(".geojson", "application/geo+json"); err != nil {
		log.Fatal(err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8787"
	}
	address := "127.0.0.1:" + port

	// Configurable (rather than hardcoded to a Mobilithek-fetched file) so
	// the same demo can serve GeoJSON from any source this package can
	// produce it from, e.g. out/events.geojson (mobilithek-fetch) or
	// out/autobahn.geojson (mobilithek-autobahn).
	eventsPath := os.Getenv("EVENTS_GEOJSON")
	if eventsPath == "" {
		eventsPath = "out/events.geojson"
	}
	eventsPath = filepath.Clean(eventsPath)

	dir := http.FileServer(http.Dir(filepath.Clean("examples/maplibre")))
	mux := http.NewServeMux()
	mux.Handle("/", dir)
	mux.HandleFunc("/converted/events.geojson", func(w http.ResponseWriter, r *http.Request) {
		if _, err := os.Stat(eventsPath); err != nil {
			http.Error(w, fmt.Sprintf("%s not found. Run mobilithek-fetch or mobilithek-autobahn with an output flag first.", eventsPath), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/geo+json")
		http.ServeFile(w, r, eventsPath)
	})

	log.Printf("MapLibre example: http://%s/ (serving %s)", address, eventsPath)
	log.Fatal(http.ListenAndServe(address, mux))
}
