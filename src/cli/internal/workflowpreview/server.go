// Package workflowpreview serves a designated draft and a tiny HTML startup page on loopback.
package workflowpreview

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
	"unicode/utf8"
)

const Protocol = "local-workflow-preview/v1"

type Event struct {
	Type     string `json:"type"`
	ViewURL  string `json:"viewUrl,omitempty"`
	Sequence string `json:"sequence,omitempty"`
	Code     string `json:"code,omitempty"`
}
type Options struct{ File, Title, ViewerURL string }
type Snapshot struct {
	Protocol string  `json:"protocol"`
	Sequence string  `json:"sequence"`
	Title    string  `json:"title"`
	Content  *string `json:"content"`
	// Null compatibility field for older v1 readers; no remote contract lookup.
	Context json.RawMessage `json:"context"`
	Problem string          `json:"problem"`
}
type state struct {
	sync.RWMutex
	snapshot Snapshot
	revision uint64
}

func (s *state) change(fn func(*Snapshot)) { s.Lock(); defer s.Unlock(); fn(&s.snapshot); s.revision++ }

// Run serves only a tiny startup page and the designated file, entirely on loopback.
func Run(ctx context.Context, opts Options, emit func(Event) error) error {
	viewer, err := validateViewerURL(opts.ViewerURL)
	if err != nil {
		return err
	}
	if opts.File == "" || opts.File == "-" {
		return fmt.Errorf("preview requires a regular file path")
	}
	if !utf8.ValidString(opts.Title) || len(opts.Title) > 256 {
		return fmt.Errorf("preview title must be UTF-8 and at most 256 bytes")
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("start local viewer: %w", err)
	}
	defer listener.Close()
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(secret)
	current := &state{snapshot: Snapshot{Protocol: Protocol, Sequence: "0", Title: opts.Title}}
	server := &http.Server{Handler: handler(listener.Addr().String(), token, viewer, current), ReadHeaderTimeout: 3 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 15 * time.Second, MaxHeaderBytes: 8192}
	serving := make(chan error, 1)
	go func() { serving <- server.Serve(listener) }()
	workCtx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		_ = server.Close()
		<-serving
		current.change(func(s *Snapshot) { s.Content = nil; s.Context = nil })
	}()
	if err = emit(Event{Type: "ready", ViewURL: "http://" + listener.Addr().String() + "/#token=" + token, Sequence: "0"}); err != nil {
		return err
	}
	err = watch(workCtx, opts.File, current, emit)
	if err != nil {
		return err
	}
	return emit(Event{Type: "ended"})
}

func handler(host, token string, viewer *url.URL, current *state) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		if r.Host != host || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "http://"+host) || (r.Header.Get("Sec-Fetch-Site") == "cross-site" && (r.URL.Path == "/api/snapshot" || r.Header.Get("Sec-Fetch-Mode") != "navigate" || r.Header.Get("Sec-Fetch-Dest") != "document")) {
			http.Error(w, "forbidden", 403)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", 405)
			return
		}
		if r.URL.Path == "/api/snapshot" {
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
				http.Error(w, "unauthorized", 401)
				return
			}
			current.RLock()
			defer current.RUnlock()
			etag := fmt.Sprintf("\"%d\"", current.revision)
			w.Header().Set("ETag", etag)
			if r.Header.Get("If-None-Match") == etag {
				w.WriteHeader(304)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			if r.Method == "GET" {
				_ = json.NewEncoder(w).Encode(current.snapshot)
			}
			return
		}
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		body, policy, err := bootstrap(viewer)
		if err != nil {
			http.Error(w, "startup_unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Security-Policy", policy)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.Method == "GET" {
			_, _ = w.Write(body)
		}
	})
}
