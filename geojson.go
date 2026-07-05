package mobilithek

import "encoding/json"

type Coordinate struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type Event struct {
	Source      string              `json:"source,omitempty"`
	ID          string              `json:"id,omitempty"`
	Type        string              `json:"type,omitempty"`
	Title       string              `json:"title,omitempty"`
	Description string              `json:"description,omitempty"`
	Road        string              `json:"road,omitempty"`
	Start       string              `json:"start,omitempty"`
	End         string              `json:"end,omitempty"`
	Status      string              `json:"status,omitempty"`
	Severity    string              `json:"severity,omitempty"`
	Coordinates []Coordinate        `json:"coordinates,omitempty"`
	Extra       map[string][]string `json:"extra,omitempty"`
}

type GeoJSON struct {
	Type     string           `json:"type"`
	BBox     []float64        `json:"bbox,omitempty"`
	Features []GeoJSONFeature `json:"features"`
}

type GeoJSONFeature struct {
	Type       string                 `json:"type"`
	BBox       []float64              `json:"bbox,omitempty"`
	Geometry   GeoJSONGeometry        `json:"geometry"`
	Properties map[string]interface{} `json:"properties"`
}

type GeoJSONGeometry struct {
	Type        string      `json:"type"`
	Coordinates interface{} `json:"coordinates"`
}

func GeoJSONFromDATEX2XML(body []byte, sourceName string) (GeoJSON, error) {
	events, err := ExtractEventsFromDATEX2XML(body, sourceName)
	if err != nil {
		return GeoJSON{}, err
	}
	return EventsToGeoJSON(events), nil
}

func EventsToGeoJSON(events []Event) GeoJSON {
	features := make([]GeoJSONFeature, 0, len(events))
	var collectionBounds coordinateBounds
	for _, event := range events {
		// Keep the public GeoJSON output strict: invalid and duplicate
		// coordinates are dropped before geometry and bbox construction.
		coordinatesForEvent := uniqueCoordinates(event.Coordinates, 0)
		if len(coordinatesForEvent) == 0 {
			continue
		}

		var geometry GeoJSONGeometry
		var featureBounds coordinateBounds
		for _, coordinate := range coordinatesForEvent {
			featureBounds.extend(coordinate)
			collectionBounds.extend(coordinate)
		}

		if len(coordinatesForEvent) == 1 {
			coordinate := coordinatesForEvent[0]
			geometry = GeoJSONGeometry{
				Type: "Point",
				// RFC 7946 uses longitude, latitude order.
				Coordinates: []float64{coordinate.Lon, coordinate.Lat},
			}
		} else {
			coordinates := make([][]float64, 0, len(coordinatesForEvent))
			for _, coordinate := range coordinatesForEvent {
				// RFC 7946 uses longitude, latitude order.
				coordinates = append(coordinates, []float64{coordinate.Lon, coordinate.Lat})
			}
			geometry = GeoJSONGeometry{
				Type:        "LineString",
				Coordinates: coordinates,
			}
		}

		properties := map[string]interface{}{
			"source":      event.Source,
			"id":          event.ID,
			"type":        event.Type,
			"title":       event.Title,
			"description": event.Description,
			"road":        event.Road,
			"start":       event.Start,
			"end":         event.End,
			"status":      event.Status,
			"severity":    event.Severity,
		}

		features = append(features, GeoJSONFeature{
			Type:       "Feature",
			BBox:       featureBounds.bbox(),
			Geometry:   geometry,
			Properties: properties,
		})
	}

	return GeoJSON{
		Type:     "FeatureCollection",
		BBox:     collectionBounds.bbox(),
		Features: features,
	}
}

func MarshalGeoJSON(geojson GeoJSON, pretty bool) ([]byte, error) {
	if pretty {
		return json.MarshalIndent(geojson, "", "  ")
	}
	return json.Marshal(geojson)
}

type coordinateBounds struct {
	minLon float64
	minLat float64
	maxLon float64
	maxLat float64
	ok     bool
}

func (b *coordinateBounds) extend(coordinate Coordinate) {
	if !validLatLon(coordinate.Lat, coordinate.Lon) {
		return
	}
	if !b.ok {
		b.minLon = coordinate.Lon
		b.maxLon = coordinate.Lon
		b.minLat = coordinate.Lat
		b.maxLat = coordinate.Lat
		b.ok = true
		return
	}
	b.minLon = min(b.minLon, coordinate.Lon)
	b.maxLon = max(b.maxLon, coordinate.Lon)
	b.minLat = min(b.minLat, coordinate.Lat)
	b.maxLat = max(b.maxLat, coordinate.Lat)
}

func (b coordinateBounds) bbox() []float64 {
	if !b.ok {
		return nil
	}
	// GeoJSON bbox order for 2D geometries is west, south, east, north.
	return []float64{b.minLon, b.minLat, b.maxLon, b.maxLat}
}
