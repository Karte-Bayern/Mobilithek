package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8787"
	}
	address := "127.0.0.1:" + port
	dir := http.FileServer(http.Dir(filepath.Clean("examples/maplibre")))
	mux := http.NewServeMux()
	mux.Handle("/", dir)
	mux.HandleFunc("/converted/events.geojson", func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Clean("out/events.geojson")
		if _, err := os.Stat(path); err != nil {
			http.Error(w, "out/events.geojson not found. Run mobilithek-fetch with -geojson-out first.", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/geo+json")
		http.ServeFile(w, r, path)
	})

	log.Printf("MapLibre example: http://%s/", address)
	log.Fatal(http.ListenAndServe(address, mux))
}
