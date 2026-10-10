package main

import (
	"flag"
	"github.com/yingziseo/claudeFF/internal/licensing"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:8787", "listen behind HTTPS reverse proxy")
	db := flag.String("db", "license.db", "SQLite database path")
	web := flag.String("web", "frontend/dist", "static frontend directory")
	flag.Parse()
	s, e := licensing.Open(*db, os.Getenv("CLAUDEFF_ADMIN_TOKEN"))
	if e != nil {
		log.Fatal(e)
	}
	defer s.DB.Close()
	mux := http.NewServeMux()
	mux.Handle("/v1/", limited(s.Handler()))
	mux.Handle("/claudeFF/", http.StripPrefix("/claudeFF/", http.FileServer(http.Dir(*web))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/claudeFF/#admin", 302)
	})
	server := &http.Server{Addr: *listen, Handler: headers(mux), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	log.Print("授权服务就绪：仅监听 ", *listen, "，公网发布须配置 HTTPS。")
	log.Fatal(server.ListenAndServe())
}
func headers(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		if strings.HasPrefix(r.URL.Path, "/v1/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

// Bound expensive password-hashing work; overload receives a retryable response.
func limited(next http.Handler) http.Handler {
	slots := make(chan struct{}, 2)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
			next.ServeHTTP(w, r)
		default:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"服务忙，请稍后重试"}`))
		}
	})
}
