package main

import (
	"fmt"
	"strings"
)

type OrderUpdate struct {
	OrderID      string `json:"order_id"`
	CustomerName string `json:"customer_name"`
	Phone        string `json:"phone"`
	State        string `json:"state"`
	TotalCents   int64  `json:"total_cents"`
	TrackingCode string `json:"tracking_code"`
}

type Alert struct {
	To             string
	Body           string
	IdempotencyKey string
}

func BuildOrderAlert(update OrderUpdate) (Alert, bool, error) {
	if update.OrderID == "" {
		return Alert{}, false, fmt.Errorf("order_id is required")
	}
	if update.Phone == "" {
		return Alert{}, false, nil
	}

	name := strings.TrimSpace(update.CustomerName)
	if name == "" {
		name = "Customer"
	}
	var body string
	switch update.State {
	case "checkout_confirmed":
		body = fmt.Sprintf("%s, order %s is confirmed. Total: $%.2f.", name, update.OrderID, float64(update.TotalCents)/100)
	case "fulfilled":
		if update.TrackingCode == "" {
			return Alert{}, false, fmt.Errorf("tracking_code is required for fulfilled orders")
		}
		body = fmt.Sprintf("%s, order %s shipped. Tracking: %s.", name, update.OrderID, update.TrackingCode)
	case "receipt_ready":
		body = fmt.Sprintf("%s, receipt for order %s is ready. Paid: $%.2f.", name, update.OrderID, float64(update.TotalCents)/100)
	case "delivered":
		body = fmt.Sprintf("%s, order %s was delivered.", name, update.OrderID)
	default:
		return Alert{}, false, nil
	}

	return Alert{
		To:             update.Phone,
		Body:           body,
		IdempotencyKey: "order:" + update.OrderID + ":" + update.State,
	}, true, nil
}
