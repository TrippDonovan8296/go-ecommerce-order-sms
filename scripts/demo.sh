#!/bin/sh
set -eu

curl --fail-with-body \
  --request POST \
  --header 'Content-Type: application/json' \
  --data '{"order_id":"A-42","customer_name":"Mina","phone":"+15550100","state":"fulfilled","total_cents":2599,"tracking_code":"ZX9"}' \
  "${ORDER_SMS_URL:-http://localhost:8080}/orders/update"
