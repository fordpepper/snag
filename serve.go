package main

import (
	"archive/zip"
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed docs/index.html
var indexHTML []byte

// The hosted page may talk to a snag server running on the user's machine.
// Forks can override with SNAG_ORIGIN.
func pagesOrigin() string {
	if o := os.Getenv("SNAG_ORIGIN"); o != "" {
		return o
	}
	return "https://fordpepper.github.io"
}

type job struct {
	mu       sync.Mutex
	State    string `json:"state"` // running | done | error
	Progress string `json:"progress"`
	Err      string `json:"error,omitempty"`
	Name     string `json:"name,omitempty"`
	files    []string
	dir      string
	created  time.Time
	cancel   context.CancelFunc
}

type server struct {
	mu       sync.Mutex
	jobs     map[string]*job
	loopback bool
}

type request struct {
	URLs    string `json:"urls"`
	Mode    string `json:"mode"`
	Res     string `json:"res"`
	AQ      string `json:"aq"`
	Cookies string `json:"cookies"`
}

func newID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func serve(addr string) error {
	if !strings.Contains(addr, ":") {
		addr = "127.0.0.1:" + addr
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	s := &server{jobs: map[string]*job{}}
	ip := net.ParseIP(host)
	s.loopback = host == "localhost" || (ip != nil && ip.IsLoopback())

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.guard(s.index))
	mux.HandleFunc("/api/health", s.guard(s.health))
	mux.HandleFunc("/api/jobs", s.guard(s.create))
	mux.HandleFunc("/api/jobs/", s.guard(s.jobRoute))

	go s.reap()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	fmt.Println("snag is running at http://" + displayAddr(ln.Addr().String()))
	fmt.Println("Open that in a browser, or use " + pagesOrigin() + "/snag. Ctrl-C to stop.")
	return (&http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}).Serve(ln)
}

func displayAddr(a string) string {
	return strings.Replace(strings.Replace(a, "127.0.0.1", "localhost", 1), "[::]", "localhost", 1)
}

// guard blocks other websites from driving the local server, and answers
// preflight requests for the hosted page.
func (s *server) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.loopback {
			h, _, err := net.SplitHostPort(r.Host)
			if err != nil {
				h = r.Host
			}
			if !in(h, "localhost", "127.0.0.1", "[::1]", "::1") {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}
		if o := r.Header.Get("Origin"); o != "" {
			if o != "http://"+r.Host && o != pagesOrigin() {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", o)
			h.Set("Vary", "Origin")
			h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Content-Type")
			h.Set("Access-Control-Allow-Private-Network", "true")
			h.Set("Access-Control-Expose-Headers", "Content-Disposition")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func (s *server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(indexHTML)
}

func (s *server) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	type b struct {
		Label string `json:"label"`
		Value string `json:"value"`
	}
	list := []b{}
	for _, o := range browsers()[1:] {
		list = append(list, b{o.label, o.val})
	}
	json.NewEncoder(w).Encode(map[string]any{"snag": true, "browsers": list})
}

func (s *server) create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var q request
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&q); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	fields := strings.Fields(q.URLs)
	if len(fields) == 0 || len(fields) > 25 {
		http.Error(w, "send between 1 and 25 links", http.StatusBadRequest)
		return
	}
	urls := normalize(fields)
	for _, u := range urls {
		if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			http.Error(w, "only http(s) links", http.StatusBadRequest)
			return
		}
	}
	s.mu.Lock()
	running := 0
	for _, j := range s.jobs {
		if j.State == "running" {
			running++
		}
	}
	if running >= 4 {
		s.mu.Unlock()
		http.Error(w, "busy, try again shortly", http.StatusTooManyRequests)
		return
	}
	dir, err := os.MkdirTemp("", "snag-")
	if err != nil {
		s.mu.Unlock()
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	j := &job{State: "running", Progress: "Starting", dir: dir, created: time.Now(), cancel: cancel}
	id := newID()
	s.jobs[id] = j
	s.mu.Unlock()

	cfg := Config{Mode: q.Mode, Res: q.Res, AQ: q.AQ, Cookies: q.Cookies}
	cfg.sanitize()
	go func() {
		defer cancel()
		saved, err := download(ctx, cfg, dir, urls, func(p string) {
			j.mu.Lock()
			j.Progress = p
			j.mu.Unlock()
		})
		j.mu.Lock()
		defer j.mu.Unlock()
		if err != nil {
			j.State, j.Err = "error", err.Error()
			return
		}
		j.State, j.files = "done", saved
		j.Name = filepath.Base(saved[0])
		if len(saved) > 1 {
			j.Name = "snag.zip"
		}
	}()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

func (s *server) jobRoute(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/jobs/"), "/")
	s.mu.Lock()
	j := s.jobs[parts[0]]
	s.mu.Unlock()
	if j == nil {
		http.NotFound(w, r)
		return
	}
	j.mu.Lock()
	state, name, files := j.State, j.Name, append([]string(nil), j.files...)
	snapshot, _ := json.Marshal(j)
	j.mu.Unlock()
	if len(parts) == 2 && parts[1] == "file" {
		if state != "done" {
			http.Error(w, "not ready", http.StatusConflict)
			return
		}
		sendFiles(w, r, name, files)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(snapshot)
}

func sendFiles(w http.ResponseWriter, r *http.Request, name string, files []string) {
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	if len(files) == 1 {
		http.ServeFile(w, r, files[0])
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	zw := zip.NewWriter(w)
	defer zw.Close()
	for _, p := range files {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		fw, err := zw.Create(filepath.Base(p))
		if err == nil {
			io.Copy(fw, f)
		}
		f.Close()
	}
}

// reap removes finished jobs and their files after 30 minutes.
func (s *server) reap() {
	for range time.Tick(time.Minute) {
		s.mu.Lock()
		for id, j := range s.jobs {
			if time.Since(j.created) > 30*time.Minute {
				j.cancel()
				os.RemoveAll(j.dir)
				delete(s.jobs, id)
			}
		}
		s.mu.Unlock()
	}
}
