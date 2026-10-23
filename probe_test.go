package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExternalSubs(t *testing.T) {
	dir := t.TempDir()
	names := []string{"movie.mkv", "movie.srt", "movie.en.srt", "movie.hindi.srt", "movie2.srt", "movie.png"}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	got := externalSubs(filepath.Join(dir, "movie.mkv"))
	if len(got) != 3 {
		t.Fatalf("got %d subs, want 3: %+v", len(got), got)
	}

	byID := map[string]TrackOption{}
	for _, g := range got {
		byID[g.ID] = g
	}
	if byID["ext:movie.srt"].Label != "Subtitle" {
		t.Errorf("movie.srt label = %q, want Subtitle", byID["ext:movie.srt"].Label)
	}
	if byID["ext:movie.en.srt"].Label != "English" {
		t.Errorf("movie.en.srt label = %q, want English", byID["ext:movie.en.srt"].Label)
	}
	if byID["ext:movie.hindi.srt"].Label != "hindi" {
		t.Errorf("movie.hindi.srt label = %q, want hindi (unrecognized tag shown as-is)", byID["ext:movie.hindi.srt"].Label)
	}
}
