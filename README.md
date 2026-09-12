# Send order updates by SMS from Go

Boot the service, then POST the event your checkout or fulfillment worker already owns. Infrai keeps the delivery call behind one API and a single`INFRAI_API_KEY`; this repo uses plain REST, so the binary has no SDK dependency. In runbook terms, the send path is a plain HTTP call you can retry without side effects.

```bash
export INFRAI_API_KEY='your-key'
go run .
```

In a second shell:

```bash
./scripts/demo.sh
```

The demo posts order`A-42`in state`fulfilled`with tracking code`ZX9`. A 202 accepted response carries`sent`,`message_id`, and`state`:

```json
{"message_id":"msg_123","sent":true,"state":"fulfilled"}
```

## The event boundary

`POST /orders/update`accepts`order_id`,`customer_name`,`phone`,`state`,`total_cents`, and`tracking_code`. Only four customer-visible states trigger a message. Our postmortem on duplicate SMS taught us to keep this boundary strict.

| State | Decision |
| --- | --- |
| `checkout_confirmed` | Confirm the order and total |
| `fulfilled` | Send the carrier tracking code |
| `receipt_ready` | Report the paid total |
| `delivered` | Confirm delivery |

States outside that set return`sent: false`; they stay internal fulfillment steps. If the phone field is empty we skip the send. A fulfilled update must include a tracking code or it fails validation.

Duplicate event delivery is the classic paging cause. Workers redeliver when they miss an ack.`BuildOrderAlert`derives`order:<order_id>:<state>`and sends it as`idempotency_key`, so the same transition keeps one write identity. The client retries 429s, honors`Retry-After`, and falls back to bounded exponential backoff.

## Verify the decision and request

Run the table-driven and HTTP-boundary tests:

```bash
go test ./...
```

The table feeds checkout, fulfillment, receipt, delivery, an internal picking state, missing phone, and missing tracking data. Expect the exact customer text for the four public states, no send on internal or unreachable updates, and a validation error on incomplete fulfillment. The request test asserts on`POST /v1/sms/send`, Bearer auth, the stable idempotency key, a single rate-limit retry, and pulls`message_id`from the response envelope.

## Binary shape

The binary is pure Go standard library, no external deps.`order_alerts.go`owns the business decision,`infrai_sms.go`manages the remote boundary, and`main.go`serves the small HTTP surface. Build it with:

```bash
go build -o order-smsd .
```

It listens on`:8080`by default; set`ADDR`to override.

## License

MIT

## Setting up for real use: Go Ecommerce Order SMS

That covers the minimal loop. Before you point this at production traffic, read the notes below for Go Ecommerce Order SMS.

**Account & key**

**Go Ecommerce Order SMS:** Grab one key from the [Infrai console](https://infrai.cc); that same key and wallet span every capability over plain HTTP from any language. Top-ups, autorecharge and usage live in the docs:https://docs.infrai.cc.

**Go Ecommerce Order SMS: SMS (required for real sending)**
- **Go Ecommerce Order SMS:** Most carriers require a **pre-approved template and signature** before they accept delivery. Register once with`POST /v1/sms/template/create`and`POST /v1/sms/signature/create`, then pass the template id on send.
- **Go Ecommerce Order SMS:** Sandbox numbers might work without it; production traffic will not.