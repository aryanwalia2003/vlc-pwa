package main

import "testing"

func TestIsWebSafe(t *testing.T) {
	cases := []struct {
		ext, v, a string
		want      bool
	}{
		{".mp4", "h264", "aac", true},
		{".mkv", "h264", "aac", false}, // container hi safe nahi
		{".mp4", "hevc", "aac", false}, // video codec safe nahi
		{".webm", "vp9", "opus", true},
	}
	for _, c := range cases {
		if got := isWebSafe(c.ext, c.v, c.a); got != c.want {
			t.Errorf("isWebSafe(%q,%q,%q) = %v, want %v", c.ext, c.v, c.a, got, c.want)
		}
	}
}

func TestNeedsRemux(t *testing.T) {
	info := &MediaInfo{Streams: []Stream{
		{Index: 0, Type: "video", Codec: "h264"},
		{Index: 1, Type: "audio", Codec: "aac"},
		{Index: 2, Type: "audio", Codec: "ac3"},
	}}
	if needsRemux("movie.mp4", info, 1) {
		t.Error("default track, safe codecs: remux nahi hona chahiye")
	}
	if !needsRemux("movie.mp4", info, 2) {
		t.Error("non-default audio track: remux hona chahiye")
	}
}
