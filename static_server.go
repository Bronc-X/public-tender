package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	distPath := "/Users/raoyi/.openclaw/workspace/hudi/bid_data_management/frontend/dist"
	backendURL, _ := url.Parse("http://127.0.0.1:8081")

	proxy := httputil.NewSingleHostReverseProxy(backendURL)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api") || strings.HasPrefix(r.URL.Path, "/files") {
			proxy.ServeHTTP(w, r)
			return
		}

		path := filepath.Join(distPath, r.URL.Path)
		if _, err := os.Stat(path); os.IsNotExist(err) || r.URL.Path == "/" {
			http.ServeFile(w, r, filepath.Join(distPath, "index.html"))
			return
		}

		http.ServeFile(w, r, path)
	})

	log.Println("Static server running on http://127.0.0.1:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
