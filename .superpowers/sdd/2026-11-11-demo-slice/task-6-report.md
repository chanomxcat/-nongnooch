# Task 6 report: Commerce
Status: DONE

Built services/commerce (schema commerce: products, quota, orders, order_items, payments, webhook_events, idempotency, order_seq):
GET /products, GET /products/availability, POST /orders (Idempotency-Key, 15 min hold), GET /orders, GET /orders/{id},
POST /orders/{id}/payments (KBank mock, ref = our paymentId), POST /webhooks/kbank (HMAC verify, idempotent by eventId, one tx:
payment paid + order confirmed + quota held->sold + outbox order.paid), GET /admin/stats/today (typ=staff), POST /internal/seed
(3 products th/en/zh, quota 5,000 x 60 days), release job every 30 s (order.expired). compose: commerce service added.

Tests (make test-int PKG=./services/commerce/..., -race, 11 tests, run 4x green): thai price with non-thai/ad-hoc holder 422;
past date 422; sold_out 409; 20 concurrent orders on quota 5 -> exactly 5 created, held=5; Idempotency replay (sequential +
6 concurrent) -> one order, failed requests do not consume the key; webhook replay (same and different eventId) -> exactly one
order.paid, payload checked; bad/missing signature 401 and no side effects; amount mismatch -> payment.discrepancy, order stays
pending; release returns quota + order.expired; stats; seed idempotent + i18n fallback.

E2E via gateway: OTP login as seeded Thai member -> POST /orders (thai, me) 201 30000 satang -> payment -> POST paymentUrl/pay
action=success (303) -> GET order confirmed with paidAt; outbox order.paid published (published_at set); ad-hoc Thai holder 422;
availability 4999.

Decisions:
- Webhook amount mismatch -> payment status mismatch + payment.discrepancy, order not confirmed. Paid after hold expiry: re-take quota if
  available and confirm, else payment paid_unfulfilled + discrepancy (needs refund; refunds out of scope).
- Idempotency via DB row claimed in the same tx (concurrent duplicates block on it); deadlock victims are retried (up to 5).
  Payment creation calls KBank inside that tx (acceptable for demo).
- Annual tickets do not consume daily quota; orders still carry a visitDate (annual items omit visitDate in order.paid).
- Bangkok date uses a fixed UTC+7 zone (distroless has no tzdata). orderNo is NN+YYMMDD+sequential 6 digits per Bangkok day.
- Users without mid (not registered) get 403 member_required on order endpoints. Payment requires returnUrl.
- Staged only my hunk of deploy/compose.yaml; Makefile untouched. An untracked stray file `nul` exists in the repo root (not mine).
