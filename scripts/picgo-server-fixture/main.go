// Local-only PicGo protocol fixture. It never sends images to another service.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

func main() {
	listener, err := net.Listen("tcp", "127.0.0.1:36677")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	server := &http.Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			List []string `json:"list"`
		}
		if r.Method != "POST" || json.NewDecoder(r.Body).Decode(&input) != nil || len(input.List) == 0 {
			http.Error(w, "invalid fixture request", 400)
			return
		}
		result := []string{}
		for _, path := range input.List {
			if _, err := os.Stat(path); err != nil {
				http.Error(w, "fixture path missing", 400)
				return
			}
			result = append(result, (&url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(path)}).String())
		}
		fmt.Printf("PICGO_OFFLINE_FIXTURE_REQUEST %d\n", len(result))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"success": true, "result": result})
	})
	mux.HandleFunc("/shutdown", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Header.Get("X-MarkdownReader-Fixture") != "offline-test" {
			http.Error(w, "forbidden", 403)
			return
		}
		fmt.Fprintln(w, "fixture stopping")
		go server.Shutdown(context.Background())
	})
	server.Handler = mux
	fmt.Println("PICGO_OFFLINE_FIXTURE_READY 127.0.0.1:36677")
	if err = server.Serve(listener); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
	fmt.Println("PICGO_OFFLINE_FIXTURE_STOPPED")
}
