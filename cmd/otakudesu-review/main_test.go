package main

import "testing"

func TestParseCSVList(t *testing.T) {
	t.Parallel()

	got := parseCSVList(" kusuriya-no-hitorigoto, ,zom-100-bucket-list-of-the-dead ")
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0] != "kusuriya-no-hitorigoto" {
		t.Fatalf("unexpected first slug %q", got[0])
	}
	if got[1] != "zom-100-bucket-list-of-the-dead" {
		t.Fatalf("unexpected second slug %q", got[1])
	}
}
