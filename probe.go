package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Stream struct {
	Index int
	Type  string // video, audio, subtitle
	Codec string
	Lang  string
	Title string
}

type MediaInfo struct {
	Streams []Stream
}

type TrackOption struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Lang    string `json:"lang,omitempty"`
	Default bool   `json:"default,omitempty"`
}

type rawProbe struct {
	Streams []struct {
		Index     int    `json:"index"`
		CodecType string `json:"codec_type"`
		CodecName string `json:"codec_name"`
		Tags      struct {
			Language string `json:"language"`
			Title    string `json:"title"`
		} `json:"tags"`
	} `json:"streams"`
}

func probeMedia(path string) (*MediaInfo, error) {
	out, err := exec.Command("ffprobe", "-v", "error", "-print_format", "json",
		"-show_entries", "stream=index,codec_type,codec_name:stream_tags=language,title", path).Output()
	if err != nil {
		return nil, err
	}
	var raw rawProbe
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}
	info := &MediaInfo{}
	for _, s := range raw.Streams {
		info.Streams = append(info.Streams, Stream{
			Index: s.Index, Type: s.CodecType, Codec: s.CodecName,
			Lang: s.Tags.Language, Title: s.Tags.Title,
		})
	}
	return info, nil
}

func (m *MediaInfo) byType(t string) []Stream {
	var out []Stream
	for _, s := range m.Streams {
		if s.Type == t {
			out = append(out, s)
		}
	}
	return out
}

func (m *MediaInfo) videoStream() (Stream, bool) {
	v := m.byType("video")
	if len(v) == 0 {
		return Stream{}, false
	}
	return v[0], true
}

func (m *MediaInfo) defaultAudio() (Stream, bool) {
	a := m.byType("audio")
	if len(a) == 0 {
		return Stream{}, false
	}
	return a[0], true
}

// bitmap-based subs, WebVTT me convert nahi ho sakte
var unsupportedSubCodec = map[string]bool{
	"hdmv_pgs_subtitle": true, "dvd_subtitle": true, "dvb_subtitle": true,
}

// ISO 639-2/1 code -> readable naam (common + Indian languages)
var langNames = map[string]string{
	"eng": "English", "en": "English",
	"hin": "Hindi", "hi": "Hindi",
	"tam": "Tamil", "ta": "Tamil",
	"tel": "Telugu", "te": "Telugu",
	"kan": "Kannada", "kn": "Kannada",
	"mal": "Malayalam", "ml": "Malayalam",
	"ben": "Bengali", "bn": "Bengali",
	"pan": "Punjabi", "pa": "Punjabi",
	"mar": "Marathi", "mr": "Marathi",
	"guj": "Gujarati", "gu": "Gujarati",
	"urd": "Urdu", "ur": "Urdu",
	"spa": "Spanish", "es": "Spanish",
	"fra": "French", "fre": "French", "fr": "French",
	"deu": "German", "ger": "German", "de": "German",
	"ita": "Italian", "it": "Italian",
	"por": "Portuguese", "pt": "Portuguese",
	"rus": "Russian", "ru": "Russian",
	"ara": "Arabic", "ar": "Arabic",
	"zho": "Chinese", "chi": "Chinese", "zh": "Chinese",
	"jpn": "Japanese", "ja": "Japanese",
	"kor": "Korean", "ko": "Korean",
	"und": "Unknown",
}

func langName(code string) (string, bool) {
	n, ok := langNames[strings.ToLower(strings.TrimSpace(code))]
	return n, ok
}

func labelFor(s Stream) string {
	if n, ok := langName(s.Lang); ok {
		return n
	}
	if s.Title != "" {
		return s.Title
	}
	if s.Lang != "" {
		return s.Lang
	}
	return "Track"
}

var subExts = map[string]bool{".srt": true, ".vtt": true, ".ass": true, ".ssa": true, ".sub": true}

// video ke sidecar subtitle files: movie.srt, movie.en.srt, movie.hindi.srt wagera
// ponytail: naming heuristic hai, har convention cover nahi karta
func externalSubs(videoPath string) []TrackOption {
	dir := filepath.Dir(videoPath)
	stem := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []TrackOption
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, stem) {
			continue
		}
		rest := strings.TrimPrefix(name, stem)
		if !strings.HasPrefix(rest, ".") {
			continue // "movie2.srt" jaisa galat prefix match skip
		}
		ext := strings.ToLower(filepath.Ext(name))
		if !subExts[ext] || len(rest) < len(ext) {
			continue
		}
		mid := strings.Trim(rest[:len(rest)-len(ext)], ".")
		label, lang := "Subtitle", ""
		if mid != "" {
			lang = mid
			if n, ok := langName(mid); ok {
				label = n
			} else {
				label = mid
			}
		}
		out = append(out, TrackOption{ID: "ext:" + name, Label: label, Lang: lang})
	}
	return out
}
