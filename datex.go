package mobilithek

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func ExtractEventsFromDATEX2XML(body []byte, sourceName string) ([]Event, error) {
	decoder := xml.NewDecoder(bytes.NewReader(body))
	decoder.Strict = false

	events := []Event{}
	path := []string{}
	var current *Event
	var currentDepth int
	var lat *float64
	var lon *float64
	var parseErr error

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			parseErr = err
			break
		}

		switch typed := token.(type) {
		case xml.StartElement:
			name := typed.Name.Local
			path = append(path, name)
			if isEventStart(name) {
				if current != nil {
					finalizeEvent(current)
					events = append(events, *current)
				}
				current = &Event{Source: sourceName, Extra: map[string][]string{}}
				currentDepth = len(path)
				for _, attr := range typed.Attr {
					lower := strings.ToLower(attr.Name.Local)
					if lower == "id" || strings.HasSuffix(lower, "id") {
						current.ID = attr.Value
					}
					if lower == "type" {
						current.Type = attr.Value
					}
				}
			}
			if current != nil {
				for _, attr := range typed.Attr {
					if strings.EqualFold(attr.Name.Local, "type") && current.Type == "" {
						current.Type = attr.Value
					}
				}
			}

		case xml.EndElement:
			name := typed.Name.Local
			if current != nil && len(path) == currentDepth && strings.EqualFold(name, path[len(path)-1]) {
				finalizeEvent(current)
				events = append(events, *current)
				current = nil
				lat, lon = nil, nil
			}
			if len(path) > 0 {
				path = path[:len(path)-1]
			}

		case xml.CharData:
			text := strings.TrimSpace(string(typed))
			if text == "" || len(path) == 0 {
				continue
			}

			leaf := strings.ToLower(path[len(path)-1])
			if number, err := parseFloatLoose(text); err == nil {
				if leaf == "latitude" || strings.HasSuffix(leaf, "latitude") {
					value := number
					lat = &value
				}
				if leaf == "longitude" || leaf == "lon" || strings.HasSuffix(leaf, "longitude") {
					value := number
					lon = &value
				}
				if lat != nil && lon != nil && validLatLon(*lat, *lon) {
					if current != nil {
						current.Coordinates = append(current.Coordinates, Coordinate{Lat: *lat, Lon: *lon})
					}
					lat, lon = nil, nil
				}
			}

			if current != nil {
				applyEventText(current, path, text)
			}
		}
	}

	if current != nil {
		finalizeEvent(current)
		events = append(events, *current)
	}

	for index := range events {
		events[index].Coordinates = uniqueCoordinates(events[index].Coordinates, 250)
		if len(events[index].Extra) == 0 {
			events[index].Extra = nil
		}
	}

	return events, parseErr
}

func isEventStart(name string) bool {
	return strings.EqualFold(name, "situationRecord") ||
		strings.EqualFold(name, "elaboratedData") ||
		strings.EqualFold(name, "roadworks")
}

func finalizeEvent(event *Event) {
	if event.Title == "" {
		event.Title = deriveEventTitle(*event)
	}
}

func applyEventText(event *Event, path []string, text string) {
	leaf := strings.ToLower(path[len(path)-1])
	joined := strings.ToLower(strings.Join(path, "."))
	text = truncate(text, 2000)

	if (strings.Contains(joined, "comment") || strings.Contains(joined, "description")) && len(text) > 2 {
		if event.Description == "" {
			event.Description = text
		}
		if event.Title == "" && len([]rune(text)) < 160 {
			event.Title = text
		}
		appendExtra(event, "comment", text)
		return
	}

	switch leaf {
	case "overallstarttime", "starttime", "validitystarttime":
		if event.Start == "" {
			event.Start = text
		}
	case "overallendtime", "endtime", "validityendtime":
		if event.End == "" {
			event.End = text
		}
	case "validitystatus", "operatoractionstatus", "status":
		if event.Status == "" {
			event.Status = text
		}
	case "overallseverity", "severity":
		if event.Severity == "" {
			event.Severity = text
		}
	case "roadname", "roadnumber", "roadidentifier":
		if event.Road == "" {
			event.Road = text
		} else if !strings.Contains(event.Road, text) {
			event.Road += " " + text
		}
	case "trafficconstrictiontype", "roadmaintenancetype", "mobilitytype", "causetype", "equipmentorsystemfaulttype":
		if event.Type == "" {
			event.Type = text
		}
		appendExtra(event, leaf, text)
	default:
		if strings.Contains(joined, "road") && len(text) < 200 {
			appendExtra(event, "road_context", text)
		}
		if strings.Contains(joined, "location") && len(text) < 200 {
			appendExtra(event, "location_context", text)
		}
	}
}

func appendExtra(event *Event, key string, value string) {
	if event.Extra == nil {
		event.Extra = map[string][]string{}
	}
	values := event.Extra[key]
	for _, existing := range values {
		if existing == value {
			return
		}
	}
	if len(values) < 10 {
		event.Extra[key] = append(values, value)
	}
}

func deriveEventTitle(event Event) string {
	parts := []string{}
	if event.Type != "" {
		parts = append(parts, event.Type)
	}
	if event.Road != "" {
		parts = append(parts, event.Road)
	}
	if event.Description != "" && len([]rune(event.Description)) < 160 {
		parts = append(parts, event.Description)
	}
	if len(parts) == 0 && event.ID != "" {
		return event.ID
	}
	return strings.Join(parts, " - ")
}

func parseFloatLoose(value string) (float64, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, ",", "."))
	return strconv.ParseFloat(value, 64)
}

func validLatLon(lat float64, lon float64) bool {
	return lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180 && !(lat == 0 && lon == 0)
}

func uniqueCoordinates(input []Coordinate, limit int) []Coordinate {
	seen := map[string]bool{}
	output := []Coordinate{}
	for _, coordinate := range input {
		key := fmt.Sprintf("%.7f,%.7f", coordinate.Lat, coordinate.Lon)
		if seen[key] || !validLatLon(coordinate.Lat, coordinate.Lon) {
			continue
		}
		seen[key] = true
		output = append(output, coordinate)
		if limit > 0 && len(output) >= limit {
			break
		}
	}
	return output
}

func truncate(value string, max int) string {
	if len([]rune(value)) <= max {
		return value
	}
	runes := []rune(value)
	return string(runes[:max]) + "..."
}
