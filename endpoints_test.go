package mobilithek

import "testing"

func TestEndpointURLDATEX2V2(t *testing.T) {
	got, err := EndpointURL("https://mobilithek.info:8443/", "123456789", EndpointDATEX2V2)
	if err != nil {
		t.Fatal(err)
	}

	want := "https://mobilithek.info:8443/mobilithek/api/v1.0/subscription/123456789/clientPullService?subscriptionID=123456789"
	if got != want {
		t.Fatalf("EndpointURL() = %q, want %q", got, want)
	}
}

func TestCandidateURLsAutoOrder(t *testing.T) {
	got, err := CandidateURLs(DefaultBaseURL, "123", EndpointAuto)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("CandidateURLs() returned %d URLs, want 4", len(got))
	}
	if got[0] != "https://mobilithek.info:8443/mobilithek/api/V1.0/subscription?subscriptionID=123" {
		t.Fatalf("unexpected first auto URL: %q", got[0])
	}
}

func TestEndpointURLRejectsNonNumericSubscriptionID(t *testing.T) {
	if _, err := EndpointURL(DefaultBaseURL, "abc", EndpointGeneric); err == nil {
		t.Fatal("EndpointURL() accepted non-numeric subscription ID")
	}
}
