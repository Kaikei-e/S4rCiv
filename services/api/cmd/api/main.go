// Command api is the read-only query side: a Connect-RPC server over the
// interpretation read models, plus health/readiness probes. It never writes
// (CQRS: the collector owns the command side).
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"connectrpc.com/connect"

	queryv1connect "s4rciv.org/api/gen/s4rciv/query/v1/queryv1connect"
	"s4rciv.org/api/internal/driver/postgres"
	"s4rciv.org/api/internal/handler/queryrpc"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe the local health endpoint and exit")
	flag.Parse()
	if *healthcheck {
		os.Exit(probe())
	}

	ctx := context.Background()
	pool, err := postgres.Connect(ctx)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer pool.Close()

	handler := queryrpc.New(postgres.NewQueryReader(pool), postgres.NewLawQueryReader(pool))
	mux := http.NewServeMux()
	// SanitizeErrors keeps raw internal/DB error detail out of RPC responses (CWE-209);
	// the BFF/browser only ever sees a generic message for Internal/Unknown failures.
	mux.Handle(queryv1connect.NewQueryServiceHandler(handler, connect.WithInterceptors(queryrpc.SanitizeErrors())))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			http.Error(w, "db unreachable", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprintln(w, "ready")
	})

	addr := ":" + envOr("API_PORT", "8080")
	// Bound every phase of a connection so a slow client cannot tie up server
	// resources indefinitely (CWE-400). All RPCs are unary and read-only, so these
	// are generous: header 5s, full request 15s, response 30s, idle keep-alive 60s.
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("api listening on %s", addr)
	log.Fatal(srv.ListenAndServe())
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
