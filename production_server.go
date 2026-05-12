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
		// Log incoming requests for debugging
		log.Printf("[%s] %s %s", r.RemoteAddr, r.Method, r.URL.Path)

		// 1. Proxy API/Files to Backend
		if strings.HasPrefix(r.URL.Path, "/api") || strings.HasPrefix(r.URL.Path, "/files") {
			proxy.ServeHTTP(w, r)
			return
		}

		// 2. Static File Serving with SPA Fallback
		fullPath := filepath.Join(distPath, r.URL.Path)
		info, err := os.Stat(fullPath)

		// Logic for SPA routing:
		// If path doesn't exist, is a directory, or is the root, or has no dot (subroute), serve index.html
		if err != nil || info.IsDir() || r.URL.Path == "/" || !strings.Contains(r.URL.Path, ".") {
			log.Printf("  -> Serving SPA entry: index.html")
			http.ServeFile(w, r, filepath.Join(distPath, "index.html"))
			return
		}

		// Otherwise serve the physical file (JS, CSS, Images)
		http.ServeFile(w, r, fullPath)
	})

	port := "5173"
	log.Printf("V3 Production Static Server starting on 0.0.0.0:%s (Forwarding /api to 8081)", port)
	log.Printf("Serving dist from: %s", distPath)
	
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
