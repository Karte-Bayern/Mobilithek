package main

import (
	"context"
	"testing"

	"github.com/karte-bayern/mobilithek"
)

func TestSplitNonEmpty(t *testing.T) {
	got := splitNonEmpty(" A9, A92 ,,A6")
	want := []string{"A9", "A92", "A6"}
	if len(got) != len(want) {
		t.Fatalf("splitNonEmpty() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("splitNonEmpty() = %v, want %v", got, want)
		}
	}
}

func TestResolveServicesAll(t *testing.T) {
	kinds, err := resolveServices("all")
	if err != nil {
		t.Fatal(err)
	}
	if len(kinds) != len(mobilithek.AllAutobahnServiceKinds()) {
		t.Fatalf("resolveServices(\"all\") returned %d kinds, want %d", len(kinds), len(mobilithek.AllAutobahnServiceKinds()))
	}
}

func TestResolveServicesRejectsEmpty(t *testing.T) {
	if _, err := resolveServices(" , "); err == nil {
		t.Fatal("resolveServices() accepted a list with no service kinds")
	}
}

func TestResolveRoadsRejectsEmpty(t *testing.T) {
	client, err := mobilithek.NewAutobahnClient()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolveRoads(context.Background(), client, " , "); err == nil {
		t.Fatal("resolveRoads() accepted a list with no road IDs")
	}
}

func TestFetchAllCollectsFailuresWithoutStoppingOtherJobs(t *testing.T) {
	client, err := mobilithek.NewAutobahnClient(mobilithek.WithAutobahnBaseURL("http://127.0.0.1:0"), mobilithek.WithAutobahnMaxRetries(0))
	if err != nil {
		t.Fatal(err)
	}

	events, failures := fetchAll(context.Background(), client, []string{"A9", "A92"}, []mobilithek.AutobahnServiceKind{mobilithek.AutobahnRoadworks}, 2)
	if len(events) != 0 {
		t.Fatalf("fetchAll() returned %d events from an unreachable server, want 0", len(events))
	}
	if len(failures) != 2 {
		t.Fatalf("fetchAll() returned %d failures, want 2 (one per road)", len(failures))
	}
}
