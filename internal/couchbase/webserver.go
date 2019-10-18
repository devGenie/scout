package couchbase

import (
	"fmt"
	"net/http"
)

func RunWebServer() {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Welcome to my website!")
	})

	http.ListenAndServe(":8600", nil)
}
