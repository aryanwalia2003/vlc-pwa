package main

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Library struct {
	Root string
}

type Track struct {
	ID   string `json:"id"` // root-relative path
	Name string `json:"name"`
}

var mediaExts = map[string]bool{
	".mp4": true, ".mkv": true, ".avi": true, ".mov": true, ".webm": true,
	".mp3": true, ".flac": true, ".wav": true, ".ogg": true, ".m4a": true,
}

var errInvalidPath = errors.New("invalid path")

// root ke bahar access block (path traversal guard) — sab handlers yahi use karte hain
func (l *Library) resolve(relPath string) (string, error) {
	full := filepath.Join(l.Root, relPath)
	rel, err := filepath.Rel(l.Root, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", errInvalidPath
	}
	return full, nil
}

func (l *Library) scan() ([]Track, error) {
	var tracks []Track
	err := filepath.WalkDir(l.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !mediaExts[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		rel, err := filepath.Rel(l.Root, path)
		if err != nil {
			return nil
		}
		tracks = append(tracks, Track{ID: rel, Name: filepath.Base(path)})
		return nil
	})
	sort.Slice(tracks, func(i, j int) bool { return tracks[i].Name < tracks[j].Name })
	return tracks, err
}

func (l *Library) handleList(w http.ResponseWriter, r *http.Request) {
	tracks, err := l.scan()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tracks)
}

func (l *Library) handleMedia(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/media/")
	full, err := l.resolve(id)
	if err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	if _, err := os.Stat(full); err != nil {
		http.NotFound(w, r)
		return
	}

	info, err := probeMedia(full)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	audioIdx := -1
	if a, ok := info.defaultAudio(); ok {
		audioIdx = a.Index
	}
	if q := r.URL.Query().Get("audio"); q != "" {
		if v, err := strconv.Atoi(q); err == nil {
			audioIdx = v
		}
	}

	if !needsRemux(full, info, audioIdx) {
		http.ServeFile(w, r, full)
		return
	}
	if err := remux(w, full, info, audioIdx); err != nil {
		log.Println("remux error:", err)
	}
}

// audio/subtitle options ki list, movie switch karne par frontend ye call karta hai
func (l *Library) handleInfo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/info/")
	full, err := l.resolve(id)
	if err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	info, err := probeMedia(full)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := struct {
		Audio     []TrackOption `json:"audio"`
		Subtitles []TrackOption `json:"subtitles"`
	}{}

	defaultAudio, hasDefault := info.defaultAudio()
	for _, s := range info.byType("audio") {
		resp.Audio = append(resp.Audio, TrackOption{
			ID:      strconv.Itoa(s.Index),
			Label:   labelFor(s),
			Lang:    s.Lang,
			Default: hasDefault && s.Index == defaultAudio.Index,
		})
	}
	for _, s := range info.byType("subtitle") {
		if unsupportedSubCodec[s.Codec] {
			continue // bitmap sub, convert nahi ho sakta
		}
		resp.Subtitles = append(resp.Subtitles, TrackOption{
			ID:    "embed:" + strconv.Itoa(s.Index),
			Label: labelFor(s),
			Lang:  s.Lang,
		})
	}
	resp.Subtitles = append(resp.Subtitles, externalSubs(full)...)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Files app se manually file upload karne ke liye (iOS share-target ka alternative)
// ponytail: single file overwrite protection hai, resumable/chunked upload nahi
func (l *Library) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	mr, err := r.MultipartReader()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var saved []string
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if part.FormName() != "file" || part.FileName() == "" {
			part.Close()
			continue
		}

		name := filepath.Base(part.FileName()) // path traversal guard
		ext := strings.ToLower(filepath.Ext(name))
		if !mediaExts[ext] && !subExts[ext] {
			part.Close()
			http.Error(w, "unsupported file type: "+name, http.StatusBadRequest)
			return
		}

		dest := filepath.Join(l.Root, name)
		if _, err := os.Stat(dest); err == nil {
			part.Close()
			http.Error(w, "already exists: "+name, http.StatusConflict)
			return
		}

		f, err := os.Create(dest)
		if err != nil {
			part.Close()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, copyErr := io.Copy(f, part)
		f.Close()
		part.Close()
		if copyErr != nil {
			os.Remove(dest)
			http.Error(w, copyErr.Error(), http.StatusInternalServerError)
			return
		}
		saved = append(saved, name)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]string{"saved": saved})
}

// embedded ya sidecar subtitle ko WebVTT me serve karta hai
func (l *Library) handleSubtitle(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/subs/")
	full, err := l.resolve(id)
	if err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	track := r.URL.Query().Get("track")

	switch {
	case strings.HasPrefix(track, "embed:"):
		idx := strings.TrimPrefix(track, "embed:")
		if err := subtitleToVTT(w, []string{"-i", full, "-map", "0:" + idx}); err != nil {
			log.Println("subtitle convert error:", err)
		}
	case strings.HasPrefix(track, "ext:"):
		name := strings.TrimPrefix(track, "ext:")
		if strings.ContainsAny(name, `/\`) {
			http.Error(w, "invalid subtitle", http.StatusBadRequest)
			return
		}
		subPath := filepath.Join(filepath.Dir(full), name)
		if strings.ToLower(filepath.Ext(subPath)) == ".vtt" {
			http.ServeFile(w, r, subPath)
			return
		}
		if err := subtitleToVTT(w, []string{"-i", subPath}); err != nil {
			log.Println("subtitle convert error:", err)
		}
	default:
		http.Error(w, "unknown track", http.StatusBadRequest)
	}
}
