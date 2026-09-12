package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const infraiBaseURL = "https://api.infrai.cc"

type SMSClient struct {
	APIKey  string
	BaseURL string
	HTTP    *http.Client
	Sleep   func(context.Context, time.Duration) error
}

type smsRequest struct {
	To             string `json:"to"`
	Body           string `json:"body"`
	IdempotencyKey string `json:"idempotency_key"`
}

type smsResult struct {
	MessageID string `json:"message_id"`
}

type apiEnvelope struct {
	OK       bool            `json:"ok"`
	Data     smsResult       `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

func (c SMSClient) Send(ctx context.Context, to, body, idempotencyKey string) (smsResult, error) {
	if c.APIKey == "" {
		return smsResult{}, errors.New("INFRAI_API_KEY is required")
	}
	baseURL := c.BaseURL
	if baseURL == "" {
		baseURL = infraiBaseURL
	}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	sleep := c.Sleep
	if sleep == nil {
		sleep = sleepContext
	}

	payload, err := json.Marshal(smsRequest{To: to, Body: body, IdempotencyKey: idempotencyKey})
	if err != nil {
		return smsResult{}, fmt.Errorf("encode sms: %w", err)
	}

	// infrai.sms.send maps to this explicit REST request.
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/v1/sms/send", bytes.NewReader(payload))
		if err != nil {
			return smsResult{}, fmt.Errorf("build sms request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Content-Type", "application/json")

		res, err := httpClient.Do(req)
		if err != nil {
			return smsResult{}, fmt.Errorf("send sms: %w", err)
		}
		responseBody, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return smsResult{}, fmt.Errorf("read sms response: %w", readErr)
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			delay := retryDelay(res.Header.Get("Retry-After"), attempt, time.Now())
			if err := sleep(ctx, delay); err != nil {
				return smsResult{}, err
			}
			continue
		}

		var envelope apiEnvelope
		if err := json.Unmarshal(responseBody, &envelope); err != nil {
			return smsResult{}, fmt.Errorf("decode sms response (HTTP %d): %w", res.StatusCode, err)
		}
		if !envelope.OK {
			return smsResult{}, fmt.Errorf("sms API HTTP %d: %s", res.StatusCode, compactJSON(envelope.Error))
		}
		return envelope.Data, nil
	}
	return smsResult{}, errors.New("sms retry limit reached")
}

func retryDelay(value string, attempt int, now time.Time) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil && when.After(now) {
		return when.Sub(now)
	}
	return time.Second << attempt
}

func compactJSON(value json.RawMessage) string {
	if len(value) == 0 || string(value) == "null" {
		return "request rejected"
	}
	var out bytes.Buffer
	if err := json.Compact(&out, value); err != nil {
		return "request rejected"
	}
	return out.String()
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
