package mobilithek

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const DefaultBaseURL = "https://mobilithek.info:8443"

type EndpointKind string

const (
	EndpointAuto      EndpointKind = "auto"
	EndpointGeneric   EndpointKind = "generic"
	EndpointDATEX2V2  EndpointKind = "datex2-v2"
	EndpointDATEX2V3  EndpointKind = "datex2-v3"
	EndpointContainer EndpointKind = "container"
)

var numericIDPattern = regexp.MustCompile(`^[0-9]+$`)

func CandidateURLs(baseURL string, subscriptionID string, endpoint EndpointKind) ([]string, error) {
	endpoint = normalizeEndpoint(endpoint)
	if endpoint == EndpointAuto {
		kinds := []EndpointKind{
			EndpointGeneric,
			EndpointDATEX2V3,
			EndpointDATEX2V2,
			EndpointContainer,
		}
		urls := make([]string, 0, len(kinds))
		for _, kind := range kinds {
			u, err := EndpointURL(baseURL, subscriptionID, kind)
			if err != nil {
				return nil, err
			}
			urls = append(urls, u)
		}
		return urls, nil
	}

	u, err := EndpointURL(baseURL, subscriptionID, endpoint)
	if err != nil {
		return nil, err
	}
	return []string{u}, nil
}

func EndpointURL(baseURL string, subscriptionID string, endpoint EndpointKind) (string, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" {
		return "", fmt.Errorf("subscription ID is required")
	}
	if !numericIDPattern.MatchString(subscriptionID) {
		return "", fmt.Errorf("subscription ID must be numeric")
	}

	endpoint = normalizeEndpoint(endpoint)
	pathID := url.PathEscape(subscriptionID)
	queryID := url.QueryEscape(subscriptionID)

	switch endpoint {
	case EndpointGeneric:
		return fmt.Sprintf("%s/mobilithek/api/V1.0/subscription?subscriptionID=%s", baseURL, queryID), nil
	case EndpointDATEX2V2:
		return fmt.Sprintf("%s/mobilithek/api/v1.0/subscription/%s/clientPullService?subscriptionID=%s", baseURL, pathID, queryID), nil
	case EndpointDATEX2V3:
		return fmt.Sprintf("%s/mobilithek/api/v1.0/subscription/datexv3?subscriptionID=%s", baseURL, queryID), nil
	case EndpointContainer:
		return fmt.Sprintf("%s/mobilithek/api/v1.0/container/subscription?subscriptionID=%s", baseURL, queryID), nil
	default:
		return "", fmt.Errorf("unsupported endpoint kind %q", endpoint)
	}
}

func normalizeEndpoint(endpoint EndpointKind) EndpointKind {
	switch strings.ToLower(strings.TrimSpace(string(endpoint))) {
	case "", "auto", "mobilithek-auto":
		return EndpointAuto
	case "generic", "subscription", "mobilithek", "mobilithek-generic":
		return EndpointGeneric
	case "datex2-v2", "datex-v2", "datex2v2", "mobilithek-datex2-v2", "mobilithek-datex-v2":
		return EndpointDATEX2V2
	case "datex2-v3", "datex-v3", "datex2v3", "mobilithek-datex2-v3", "mobilithek-datex-v3":
		return EndpointDATEX2V3
	case "container", "mobilithek-container":
		return EndpointContainer
	default:
		return endpoint
	}
}
