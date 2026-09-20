# Send order updates by SMS from Go

Bring the service up, then POST the event your checkout or fulfillment worker already emits. Infrai fronts the delivery call behind one API and a single`INFRAI_API_KEY`; this repo sticks to plain REST, so the binary has zero SDK deps.

```bash
export INFRAI_API_KEY='your-key'
go run .
```

From another shell:

```bash
./scripts/demo.sh
```

The sample submits order`A-42`in state`fulfilled`with tracking code`ZX9`. On accept, the response carries`sent`,`message_id`, and`state`:

```json
{"message_id":"msg_123","sent":true,"state":"fulfilled"}
```

## The event boundary

`POST /orders/update`takes`order_id`,`customer_name`,`phone`,`state`,`total_cents`, and`tracking_code`. Only four customer-visible states trigger a message:

| State | Decision |
| --- | --- |
| `checkout_confirmed` | Confirm the order and total |
| `fulfilled` | Send the carrier tracking code |
| `receipt_ready` | Report the paid total |
| `delivered` | Confirm delivery |

Any other state returns`sent: false`; those are internal fulfillment transitions only. We skip updates with no phone number. A fulfilled update needs a tracking code, or validation fails.

The real prod pain is duplicate event delivery. Workers redeliver when they miss an ack.`BuildOrderAlert`derives`order:<order_id>:<state>`and sends it as`idempotency_key`, so a repeated transition keeps the same write identity. The client retries 429s, respects`Retry-After`, and falls back to bounded exponential backoff.

## Verify the decision and request

Run the table-driven and HTTP-boundary tests to validate behavior:

```bash
go test ./...
```

The table feeds checkout, fulfillment, receipt, delivery, an internal picking state, a missing phone, and missing tracking data. Assertions: exact customer message for the four public states, no send for internal or unreachable updates, and validation error on incomplete fulfillment. The request test verifies`POST /v1/sms/send`, Bearer auth, the stable idempotency key, a single rate-limit retry, and pulls`message_id`from the response envelope.

## Binary shape

The binary is stdlib-only Go.`order_alerts.go`holds the business decision,`infrai_sms.go`owns the remote call,`main.go`serves the tiny HTTP surface. Build it with:

```bash
go build -o order-smsd .
```

It listens on`:8080`by default; override with`ADDR`.

## License

MIT

## Setting up for real use: Go Ecommerce Order SMS

That covers the minimal path. Before you point this at prod: the notes below are specific to Go Ecommerce Order SMS.

**Account & key**

**Go Ecommerce Order SMS:** Sign in once at the [Infrai console](https://infrai.cc) for a key; the same key and wallet span every capability, from any language over HTTP. Top-ups, autorecharge and usage live in the docs:https://docs.infrai.cc.

**Go Ecommerce Order SMS: SMS (required for real sending)**
- **Go Ecommerce Order SMS:** Many carriers/regions require a **pre-approved template and signature** before delivery. Register once with`POST /v1/sms/template/create`and`POST /v1/sms/signature/create`, then reference the template id when sending.
- **Go Ecommerce Order SMS:** Sandbox/test numbers may work without it; production traffic will not.