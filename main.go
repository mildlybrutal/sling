package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
)

// ---- behavior under test: each test commit edits this block ----
const (
	message      = "hello from v1"
	startupDelay = 0 * time.Second
	crashOnBoot  = false
	healthStatus = http.StatusOK
)

// ----------------------------------------------------------------

type item struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

var (
	mu    sync.Mutex
	items = []item{{ID: 1, Name: "first"}}
)

func main() {
	// Sling sets APP_VERSION to the commit SHA so you can see which
	// version answered after a blue/green switch.
	version := os.Getenv("APP_VERSION")
	if version == "" {
		version = "dev"
	}
	// The container port Sling publishes is 3000.
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	if crashOnBoot {
		log.Println("fatal: simulated crash on boot")
		os.Exit(1)
	}
	if startupDelay > 0 {
		log.Printf("slow start: sleeping %v before listening", startupDelay)
		time.Sleep(startupDelay)
	}

	r := mux.NewRouter()

	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		status := "healthy"
		if healthStatus != http.StatusOK {
			status = "unhealthy"
		}
		writeJSON(w, healthStatus, map[string]string{"status": status})
	}).Methods(http.MethodGet)

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from %s! %s\n", version, message)
	}).Methods(http.MethodGet)

	r.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		writeJSON(w, http.StatusOK, items)
	}).Methods(http.MethodGet)

	r.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil || in.Name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": `body must be {"name": "..."}`})
			return
		}
		mu.Lock()
		defer mu.Unlock()
		it := item{ID: len(items) + 1, Name: in.Name}
		items = append(items, it)
		writeJSON(w, http.StatusCreated, it)
	}).Methods(http.MethodPost)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Graceful shutdown: on SIGTERM (docker stop) finish in-flight requests
	// instead of dropping them. This is what makes retiring the old
	// container during a blue/green switch safe.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()

	go func() {
		log.Printf("listening on %s (version %s)", srv.Addr, version)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
