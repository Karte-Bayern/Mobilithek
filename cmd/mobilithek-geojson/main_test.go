package main

import "testing"

func TestParseBBox(t *testing.T) {
	bbox, err := parseBBox("8.8,47.2,13.9,50.7")
	if err != nil {
		t.Fatal(err)
	}
	if bbox.West != 8.8 || bbox.South != 47.2 || bbox.East != 13.9 || bbox.North != 50.7 {
		t.Fatalf("parseBBox() = %+v, want {West:8.8 South:47.2 East:13.9 North:50.7}", bbox)
	}
}

func TestParseBBoxRejectsWrongComponentCount(t *testing.T) {
	if _, err := parseBBox("8.8,47.2,13.9"); err == nil {
		t.Fatal("parseBBox() accepted a bbox with only 3 components")
	}
}

func TestParseBBoxRejectsNonNumericComponent(t *testing.T) {
	if _, err := parseBBox("8.8,north,13.9,50.7"); err == nil {
		t.Fatal("parseBBox() accepted a non-numeric component")
	}
}
