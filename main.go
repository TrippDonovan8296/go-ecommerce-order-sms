package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"
)

type orderHandler struct {
	sms SMSClient
}

func (h orderHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	var update OrderUpdate
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&update); err != nil {
		http.Error(w, "invalid order update", http.StatusBadRequest)
		return
	}
	alert, send, err := BuildOrderAlert(update)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !send {
		writeJSON(w, http.StatusOK, map[string]any{"sent": false, "state": update.State})
		return
	}
	result, err := h.sms.Send(r.Context(), alert.To, alert.Body, alert.IdempotencyKey)
	if err != nil {
		if errors.Is(err, r.Context().Err()) {
			http.Error(w, "request canceled", http.StatusRequestTimeout)
			return
		}
		log.Printf("send order alert: %v", err)
		http.Error(w, "could not send order alert", http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"sent": true, "message_id": result.MessageID, "state": update.State})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func main() {
	client := SMSClient{
		APIKey: os.Getenv("INFRAI_API_KEY"),
		HTTP:   &http.Client{Timeout: 10 * time.Second},
	}
	if client.APIKey == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	mux := http.NewServeMux()
	mux.Handle("POST /orders/update", orderHandler{sms: client})
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("order-smsd listening on %s", addr)
	log.Fatal(server.ListenAndServe())
}
