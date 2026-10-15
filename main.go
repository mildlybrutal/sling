package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

type Routes struct {
	health health
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "heathy",
		})
	}).Methods(http.MethodGet)
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from %s!\n")
	}).Methods(http.MethodGet)

	fmt.Println("Server runnin on port 8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
