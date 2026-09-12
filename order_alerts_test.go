package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBuildOrderAlert(t *testing.T) {
	tests := []struct {
		name     string
		update   OrderUpdate
		wantBody string
		wantSend bool
		wantErr  bool
	}{
		{"checkout", OrderUpdate{OrderID: "A-42", CustomerName: "Mina", Phone: "+15550100", State: "checkout_confirmed", TotalCents: 2599}, "Mina, order A-42 is confirmed. Total: $25.99.", true, false},
		{"fulfillment", OrderUpdate{OrderID: "A-42", CustomerName: "Mina", Phone: "+15550100", State: "fulfilled", TrackingCode: "ZX9"}, "Mina, order A-42 shipped. Tracking: ZX9.", true, false},
		{"receipt", OrderUpdate{OrderID: "A-42", CustomerName: "Mina", Phone: "+15550100", State: "receipt_ready", TotalCents: 2599}, "Mina, receipt for order A-42 is ready. Paid: $25.99.", true, false},
		{"delivery", OrderUpdate{OrderID: "A-42", CustomerName: "Mina", Phone: "+15550100", State: "delivered"}, "Mina, order A-42 was delivered.", true, false},
		{"internal state", OrderUpdate{OrderID: "A-42", Phone: "+15550100", State: "picking"}, "", false, false},
		{"no phone", OrderUpdate{OrderID: "A-42", State: "delivered"}, "", false, false},
		{"missing tracking", OrderUpdate{OrderID: "A-42", Phone: "+15550100", State: "fulfilled"}, "", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alert, send, err := BuildOrderAlert(tt.update)
			if (err != nil) != tt.wantErr || send != tt.wantSend || alert.Body != tt.wantBody {
				t.Fatalf("BuildOrderAlert() = (%q, %v, %v)", alert.Body, send, err)
			}
			if send && alert.IdempotencyKey != "order:A-42:"+tt.update.State {
				t.Fatalf("idempotency key = %q", alert.IdempotencyKey)
			}
		})
	}
}

func TestSMSClientRetriesRateLimitAndReadsEnvelope(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPost || r.URL.Path != "/v1/sms/send" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		var body smsRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.IdempotencyKey != "order:A-42:delivered" {
			t.Fatalf("idempotency_key = %q", body.IdempotencyKey)
		}
		if requests == 1 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"ok":false,"error":{"message":"rate limited"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":{"message_id":"msg_123"},"metadata":{}}`))
	}))
	defer server.Close()

	var slept time.Duration
	client := SMSClient{
		APIKey:  "test-key",
		BaseURL: server.URL,
		HTTP:    server.Client(),
		Sleep: func(_ context.Context, delay time.Duration) error {
			slept = delay
			return nil
		},
	}
	result, err := client.Send(context.Background(), "+15550100", "Order delivered.", "order:A-42:delivered")
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || slept != 2*time.Second || result.MessageID != "msg_123" {
		t.Fatalf("requests=%d slept=%s result=%+v", requests, slept, result)
	}
}
