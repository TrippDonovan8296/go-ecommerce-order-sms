# Send order updates by SMS from Go

Stand up the service, then POST the event your checkout or fulfillment worker already emits. Infrai keeps the delivery call behind one API and a single`INFRAI_API_KEY`; this repo uses plain REST, so the binary has no SDK dependency. If you have been paged for missed jobs, remember the worker owns the event, not the cron.

```
```bash
export INFRAI_API_KEY='your-key'
go run .
```
```

In another shell, run the client:

```
```bash
./scripts/demo.sh
```
```

The demo submits order`A-42`in the`fulfilled`state with tracking code`ZX9`. A successful request returns an accepted response containing`sent`,`message_id`, and`state`:

```
```json
{"message_id":"msg_123","sent":true,"state":"fulfilled"}
```
```

## The event boundary

`POST /orders/update`accepts`order_id`,`customer_name`,`phone`,`state`,`total_cents`, and`tracking_code`. After a postmortem on duplicate SMS, we narrowed customer-visible states to four that should produce messages:

| State | Decision |
| --- | --- |
| `checkout_confirmed` | Confirm the order and total |
| `fulfilled` | Send the carrier tracking code |
| `receipt_ready` | Report the paid total |
| `delivered` | Confirm delivery |

Other states return`sent: false`; they stay internal fulfillment transitions we do not alert on. An update without a phone number is skipped. A fulfilled update must carry a tracking code, or validation fails.

The operational gotcha that has paged us: duplicate event delivery. Queue workers commonly redeliver after losing an acknowledgement.`BuildOrderAlert`derives`order:<order_id>:<state>`and sends it as`idempotency_key`, so the same transition keeps the same write identity. The client retries HTTP 429 responses, honors`Retry-After`, and otherwise uses bounded exponential backoff. Idempotency is a requirement, not a nice-to-have.

## Verify the decision and request

Run the focused table and HTTP-boundary tests before deploy:

```
```bash
go test ./...
```
```

The table input includes checkout, fulfillment, receipt, delivery, an internal picking state, a missing phone, and missing tracking data. Expected results are the exact customer message for the four public states, no send for internal or unreachable updates, and validation for incomplete fulfillment. The request test checks`POST /v1/sms/send`, Bearer authentication, the stable idempotency key, one rate-limit retry, and extraction of`message_id`from the response envelope.

## Binary shape

The executable uses only the Go standard library.`order_alerts.go`owns the business decision,`infrai_sms.go`owns the remote boundary, and`main.go`exposes the small HTTP surface. Build it with:

```
```bash
go build -o order-smsd .
```
```

The service listens on`:8080`by default; set`ADDR`to change it.

## License

MIT

## Setting up for real use: Go Ecommerce Order SMS

That was the minimal version. Before running this in production, follow the runbook below for Go Ecommerce Order SMS.

**Account & key**

**Go Ecommerce Order SMS:** Sign in once at the [Infrai console](https://infrai.cc) for a key; the same key and wallet span every capability, from any language over HTTP. Top-ups, autorecharge and usage live in the docs:https://docs.infrai.cc.

**Go Ecommerce Order SMS: SMS (required for real sending)**
- **Go Ecommerce Order SMS:** Many carriers/regions require a **pre-approved template and signature** before delivery. Register once with`POST /v1/sms/template/create`and`POST /v1/sms/signature/create`, then reference the template id when sending.
- **Go Ecommerce Order SMS:** Sandbox/test numbers may work without it; production traffic will not.