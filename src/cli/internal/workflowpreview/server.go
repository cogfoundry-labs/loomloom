// Package workflowpreview serves a single draft and the shared static viewer on loopback.
package workflowpreview

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strings"
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
type Options struct{ File, Title string }
type Resolver func(context.Context, Bindings) (json.RawMessage, error)
type Snapshot struct {
	Protocol string          `json:"protocol"`
	Sequence string          `json:"sequence"`
	Title    string          `json:"title"`
	Content  *string         `json:"content"`
	Context  json.RawMessage `json:"context"`
	Problem  string          `json:"problem"`
}
type state struct {
	sync.RWMutex
	snapshot Snapshot
	revision uint64
}

func (s *state) change(fn func(*Snapshot)) { s.Lock(); defer s.Unlock(); fn(&s.snapshot); s.revision++ }

// Run never uploads the document. Resolver receives a typed projection containing bindings only.
// File updates and resolver work are independent so a slow authority cannot block local viewing.
func Run(ctx context.Context, assets fs.FS, resolve Resolver, opts Options, emit func(Event) error) error {
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
	server := &http.Server{Handler: handler(listener.Addr().String(), token, assets, current), ReadHeaderTimeout: 3 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 15 * time.Second, MaxHeaderBytes: 8192}
	serving := make(chan error, 1)
	go func() { serving <- server.Serve(listener) }()
	workCtx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		_ = server.Close()
		<-serving
		current.change(func(s *Snapshot) { s.Content = nil; s.Context = nil })
	}()
	jobs := make(chan resolveJob, 1)
	done := make(chan struct{})
	go func() { defer close(done); resolveLoop(workCtx, resolve, jobs, current) }()
	defer func() { cancel(); <-done }()
	if err = emit(Event{Type: "ready", ViewURL: "http://" + listener.Addr().String() + "/#token=" + token, Sequence: "0"}); err != nil {
		return err
	}
	err = watch(workCtx, opts.File, current, jobs, emit)
	if err != nil {
		return err
	}
	return emit(Event{Type: "ended"})
}

func handler(host, token string, assets fs.FS, current *state) http.Handler {
	static := http.FileServer(http.FS(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; worker-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
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
		// Only compiled assets, never directory listings or arbitrary filesystem paths.
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "viewer.html"
		}
		if !fs.ValidPath(path) {
			http.NotFound(w, r)
			return
		}
		entry, err := fs.Stat(assets, path)
		if err != nil || entry.IsDir() {
			http.NotFound(w, r)
			return
		}
		r.URL.Path = "/" + path
		static.ServeHTTP(w, r)
	})
}
