package main

import (
	"net/http"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

var webSafeContainers = map[string]bool{".mp4": true, ".webm": true, ".ogg": true}
var webSafeVideoCodecs = map[string]bool{"h264": true, "vp8": true, "vp9": true, "av1": true}
var webSafeAudioCodecs = map[string]bool{"aac": true, "opus": true, "vorbis": true, "mp3": true}

// browser isse direct play kar sakta hai kya
func isWebSafe(ext, videoCodec, audioCodec string) bool {
	if !webSafeContainers[strings.ToLower(ext)] {
		return false
	}
	if videoCodec != "" && !webSafeVideoCodecs[videoCodec] {
		return false
	}
	if audioCodec != "" && !webSafeAudioCodecs[audioCodec] {
		return false
	}
	return true
}

// file seedhi serve ho sakti hai, ya remux chahiye (non-default audio bhi trigger karta hai)
func needsRemux(path string, info *MediaInfo, audioIdx int) bool {
	video, hasVideo := info.videoStream()
	audio, hasAudio := info.defaultAudio()
	if hasAudio && audioIdx != audio.Index {
		return true // user ne non-default audio track maanga
	}
	vCodec, aCodec := "", ""
	if hasVideo {
		vCodec = video.Codec
	}
	if hasAudio {
		aCodec = audio.Codec
	}
	return !isWebSafe(filepath.Ext(path), vCodec, aCodec)
}

// chuni hui audio track + video ko mux karke fragmented mp4 stream karo
// ponytail: disk cache nahi, har request re-transcode; zaroorat pade to cache add karo
func remux(w http.ResponseWriter, path string, info *MediaInfo, audioIdx int) error {
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "no-store") // dynamic output, cache mat karo
	args := []string{"-i", path}

	if video, ok := info.videoStream(); ok {
		args = append(args, "-map", "0:"+strconv.Itoa(video.Index))
		if webSafeVideoCodecs[video.Codec] {
			args = append(args, "-c:v", "copy")
		} else {
			args = append(args, "-c:v", "libx264", "-preset", "veryfast")
		}
	}

	for _, s := range info.byType("audio") {
		if s.Index != audioIdx {
			continue
		}
		args = append(args, "-map", "0:"+strconv.Itoa(s.Index))
		if webSafeAudioCodecs[s.Codec] {
			args = append(args, "-c:a", "copy")
		} else {
			args = append(args, "-c:a", "aac")
		}
		break
	}

	// faststart pipe output ke saath contradict karta hai (moov relocate seek maangta hai), isliye nahi
	args = append(args, "-movflags", "frag_keyframe+empty_moov", "-f", "mp4", "-")
	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = w
	return cmd.Run()
}

// subtitle stream/file ko WebVTT me convert karke stream karo
func subtitleToVTT(w http.ResponseWriter, inputArgs []string) error {
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store") // dynamic output, cache mat karo
	args := append(inputArgs, "-c:s", "webvtt", "-f", "webvtt", "-")
	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = w
	return cmd.Run()
}
