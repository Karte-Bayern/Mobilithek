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
	Features []GeoJSONFeature `json:"features"`
}

type GeoJSONFeature struct {
	Type       string                 `json:"type"`
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
	features := []GeoJSONFeature{}
	for _, event := range events {
		if len(event.Coordinates) == 0 {
			continue
		}

		var geometry GeoJSONGeometry
		if len(event.Coordinates) == 1 {
			coordinate := event.Coordinates[0]
			geometry = GeoJSONGeometry{
				Type:        "Point",
				Coordinates: []float64{coordinate.Lon, coordinate.Lat},
			}
		} else {
			coordinates := make([][]float64, 0, len(event.Coordinates))
			for _, coordinate := range event.Coordinates {
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
			Geometry:   geometry,
			Properties: properties,
		})
	}

	return GeoJSON{
		Type:     "FeatureCollection",
		Features: features,
	}
}

func MarshalGeoJSON(geojson GeoJSON, pretty bool) ([]byte, error) {
	if pretty {
		return json.MarshalIndent(geojson, "", "  ")
	}
	return json.Marshal(geojson)
}
