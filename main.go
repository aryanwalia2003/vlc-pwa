package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"
)

//go:embed web
var webFS embed.FS

func main() {
	dir := flag.String("dir", ".", "media library root")
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	lib := &Library{Root: *dir}

	static, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(static)))
	mux.HandleFunc("/api/library", lib.handleList)
	mux.HandleFunc("/api/info/", lib.handleInfo)
	mux.HandleFunc("/media/", lib.handleMedia)
	mux.HandleFunc("/subs/", lib.handleSubtitle)
	mux.HandleFunc("/api/upload", lib.handleUpload)

	log.Printf("vlc-pwa http://localhost%s (library: %s)", *addr, *dir)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
