// Serves a self-authored test PNG on loopback only.
package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"os"
)

func main() {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	data, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAACAAAAAYCAYAAACbU/80AAAAPElEQVR4AezQQQoAMAgDwdWX9+e2b1Chlw3kGAiTBWdSqDfvN/kcDyiggAIKLAhEQL8LBxjFAwoooIACFwAA//9lZw9LAAAABklEQVQDAPAbOCF7gZ39AAAAAElFTkSuQmCC")
	server := &http.Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("/photo.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(data)
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
	fmt.Println("IMAGE_HTTP_FIXTURE_READY http://" + listener.Addr().String())
	if err = server.Serve(listener); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
	fmt.Println("IMAGE_HTTP_FIXTURE_STOPPED")
}
