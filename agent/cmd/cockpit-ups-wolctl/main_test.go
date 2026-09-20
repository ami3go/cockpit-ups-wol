package main

import (
	"bytes"
	"testing"
)

func TestReadCandidateRejectsEmptyAndOversizedInput(t *testing.T) {
	if _, err := readCandidate(bytes.NewReader(nil)); err == nil {
		t.Fatal("empty candidate accepted")
	}
	tooLarge := bytes.Repeat([]byte{'x'}, maxCandidateBytes+1)
	if _, err := readCandidate(bytes.NewReader(tooLarge)); err == nil {
		t.Fatal("oversized candidate accepted")
	}
}

func TestReadCandidateAcceptsBoundedInput(t *testing.T) {
	want := []byte("config_version: 1\n")
	got, err := readCandidate(bytes.NewReader(want))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("candidate=%q want %q", got, want)
	}
}
