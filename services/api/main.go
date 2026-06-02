package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe the local health endpoint and exit")
	flag.Parse()
	if *healthcheck {
		os.Exit(probe())
	}

	// Fail fast if the DB password secret is not mounted. Never log its value.
	if path := os.Getenv("DB_PASSWORD_FILE"); path != "" {
		b, err := os.ReadFile(path)
		if err != nil || len(strings.TrimSpace(string(b))) == 0 {
			log.Fatalf("db password secret missing or empty at %s", path)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if err := dialDB(); err != nil {
			http.Error(w, "db unreachable", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprintln(w, "ready")
	})

	addr := ":" + envOr("API_PORT", "8080")
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("api listening on %s", addr)
	log.Fatal(srv.ListenAndServe())
}

func dialDB() error {
	addr := net.JoinHostPort(envOr("POSTGRES_HOST", "db"), envOr("POSTGRES_PORT", "5432"))
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return err
	}
	return conn.Close()
}

func probe() int {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + envOr("API_PORT", "8080") + "/healthz")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
