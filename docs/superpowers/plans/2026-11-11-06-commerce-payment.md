# Commerce: Payment, Reconciliation, Refund, Waiting Room Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ต่อยอด service `commerce` จาก Plan 05 ให้รับเงินผ่าน KBank (mockext ใน demo) ด้วยบัตร / PromptPay / Alipay+ / WeChat Pay / UnionPay, ยืนยันด้วย Webhook ที่ลงลายเซ็นแบบ idempotent แล้วส่ง `order.paid`, กระทบยอดทุกชั่วโมง, คืนเงินโดย Finance (void บัตรก่อนคืน, เงินหรือ Voucher), Waiting room ที่ออกบัตรคิวลงลายเซ็นและ access token ให้ Gateway ตรวจ และส่งใบเสร็จ / ใบกำกับภาษีทางอีเมล

**Architecture:** แพ็กเกจใหม่ใน `services/commerce`: `payment` (สร้างรายการ + Webhook + `ConfirmPaid` ที่ Webhook และงานกระทบยอดใช้ร่วมกัน), `recon`, `refund` (รวม Voucher), `queue` (Waiting room) และ `receipt` ทุกจุดที่เปลี่ยนสถานะเงินล็อกแถว `payments` แล้ว `orders` ด้วย `SELECT … FOR UPDATE` ตามลำดับนี้เสมอ ส่วน event เขียน outbox ใน tx เดียวกัน KBank / Ticket / Member / SMTP เรียกผ่าน interface + URL ใน env ตัวตรวจ access token ของ Waiting room อยู่ใน `pkg/chassis/queuetoken` เพื่อให้ Gateway (Plan 12) import ไปใช้ได้ตรง ๆ

**Tech Stack:** Go 1.23 + chassis (Plan 01), `crypto/hmac`, `crypto/sha256`, `crypto/ed25519`, `html/template`, `net/smtp`, gRPC stub `gen/go/ticket/v1`, `gen/go/member/v1`, mockext `/pg` (Plan 00 Task 4), Mailpit

**Spec:** [docs/system-design.md](../../system-design.md) S3, S4, S6, F3, หัวข้อ 2 (Payment, Waiting room) · [ROADMAP.md](../../../roadmap/ROADMAP.md) 4.2 E3, E6 (จัดการคำสั่งซื้อ), E8 (Waiting room) · [contracts](2026-11-11-contracts.md) · ขึ้นกับ [Plan 05](2026-11-11-05-commerce-catalog-order.md) ทั้งแผน, [Plan 00](2026-11-11-00-mock-external.md) Task 4 และ proto ของ Plan 07 (ถ้ายังไม่มี Task 3 สร้าง `api/proto/ticket/v1/ticket.proto` ตาม contracts §4 ทุกตัวอักษร) · WBS 1.1.13 (mock KBank) อยู่ใน Plan 00 แล้ว

## Global Constraints

- **Build และ test ใน Docker เสมอ (มติ 11 พ.ย. 2026):** ห้ามรัน `go` / `npm` / `flutter` บนเครื่อง ใช้ `make test`, `make test-int`, `make build SVC=commerce`, `make gen`, `make seed` เท่านั้น integration test ขึ้นต้นด้วย `//go:build integration`
- **โหมด Demo:** KBank ใช้ mockext `/pg` ผ่าน `KBANK_BASE_URL`, อีเมลส่งไป Mailpit ผ่าน `SMTP_ADDR` ทุกอย่างอยู่หลัง interface ห้าม hardcode URL
- **ทำตาม [contracts](2026-11-11-contracts.md) ห้ามแก้เอง** ข้อที่ต้องเปลี่ยนอยู่ในหัวข้อ "ข้อเสนอแก้ contracts" ท้ายแผน
- ทุก service ที่มีข้อมูลต้องมี `POST /internal/seed` (`DEMO_MODE=true` เท่านั้น, รันซ้ำได้) แผนนี้ลงทะเบียนส่วน `refund_policy` และ `queue` ผ่าน `seed.Register` ของ Plan 05
- ข้อกำหนดใน Global Constraints ของ Plan 01 และ Plan 05 (เงินเป็นสตางค์ `int64`, เวลา UTC, error / list format, ห้ามเลขบัตรเต็ม)
- S3: "ตรวจโควตายังจองอยู่ · Payment(pending)" แล้ว "BEGIN · Payment=paid · Order=confirmed · ตัดโควตาจริง · outbox order.paid · COMMIT" และ "eventId เคยประมวลผลแล้ว → 200 OK (idempotent)"
- S3: "ออกบัตรล้มเหลว JetStream redeliver จนสำเร็จ ไม่ refund อัตโนมัติ"
- E3: "ไม่เก็บเลขบัตรในระบบเรา" · "ออก e-Ticket ภายใน 30 วินาทีหลัง Webhook ยืนยัน" · "Webhook ทำงานแบบ idempotent, มีงานกระทบยอดกับ Gateway ทุกชั่วโมง" · "Alipay+, WeChat Pay, UnionPay (และ UPI ถ้า Gateway รองรับ)" · "ใบเสร็จอิเล็กทรอนิกส์ส่งทางอีเมล, ขอใบกำกับภาษีเต็มรูปได้ [รอยืนยันรูปแบบจากฝ่ายบัญชี]" · "คืนเงินจาก CMS (ตามสิทธิ์), บัตรที่คืนเงินแล้วสแกนไม่ผ่าน"
- S4: "หา Payment pending เกิน 15 นาที + รายการชั่วโมงก่อน" · KBank paid → "ทำขั้นตอนเดียวกับ Webhook" · failed / expired → "Payment=failed · ปล่อยโควตาคืน" · ยอดไม่ตรง → "บันทึก discrepancy" + `payment.discrepancy`
- S4: คืนเงิน "ตรวจสิทธิ์ Finance · Refund(pending) · AuditLog" → "gRPC VoidTickets(orderId) แบบ sync ให้สแกนไม่ผ่านทันที" → ขอคืนเงิน → "Refund=done · outbox order.refunded"
- E6: "ระบบคำนวณยอดคืนและค่าธรรมเนียมตามนโยบายคืนเงินที่อนุมัติ, คืนเป็นเงินหรือ Voucher ตามนโยบาย"
- Contracts v2: Waiting room access token ตามหัวข้อ 8, ชื่อ secret ตามหัวข้อ 9 (`KBANK_API_KEY`, `KBANK_WEBHOOK_SECRET`, `QUEUE_TOKEN_KEY`), `Idempotency-Key` ของ `POST /orders/{id}/payments` ตามหัวข้อ 2, `order.refunded` ตามหัวข้อ 5, audit log ตามรูปแบบหัวข้อ 1, route สาธารณะ `/api/commerce/queue/*` และ `POST /api/commerce/webhooks/kbank` ตามหัวข้อ 2
- E8: Waiting room "ออกบัตรคิวลงลายเซ็นตามลำดับเข้า, ปล่อยเข้าตามจำนวนที่ระบบรับได้ (ตั้งค่าได้จาก Admin), หน้ารอคิวแสดงลำดับและเวลาโดยประมาณ 7 ภาษา, เปิด / ปิดได้ทันทีจาก Admin, ไม่บล็อกการสแกนประตูและหน้าสมัครสมาชิก (ยกเว้นตั้งค่าให้รวม), ผ่าน Load test ที่ 3 เท่าของยอดพีค" · S6: "บัตรคิวลงลายเซ็น {seq, issuedAt, sig}", poll "ทุก 10–20 วินาที [สมมติฐาน]", "access token อายุสั้น", "GW ตรวจ token ก่อน route"

**ค่าที่แผนนี้กำหนดเพิ่ม เพราะ spec ไม่ได้ระบุ (ให้ PO / Finance ยืนยันใน review):** Webhook body สูงสุด 64 KB, inquiry ไม่พบ 3 รอบติดกันถือว่า failed, refund policy demo (≥ 24 ชม. ก่อนวันเข้าชม 08:00 น. ไม่มีค่าธรรมเนียม, < 24 ชม. ค่าธรรมเนียมเงินคืน 20% / Voucher 0%, หลังเวลาเข้าชมคืนไม่ได้) [รอ ROADMAP #17], Voucher อายุ 365 วัน [รอ #29], บัตรคิวอายุ 2 ชม., access token อายุ 10 นาที, poll ทุก 15 วินาที, release tick ทุก 5 วินาที, VAT 7% รวมในราคา (`vat = round(total × 7 / 107)`)

**ต่างจาก spec:** (1) S3 ส่ง `ref=orderId` แต่แผนนี้ใช้ `ref = paymentId` ของเรา เพราะออเดอร์หนึ่งจ่ายได้หลายครั้ง (ลองใหม่ภายในเวลาจองตาม F3) inquiry ต้องแยกรายครั้ง (2) S6 ระบุว่า Waiting room ปิดแล้ว "เข้าได้ทันที" แผนนี้ยังออก access token ให้ทันทีเพื่อให้ Gateway ตรวจแบบเดียวทุกกรณีโดยไม่ต้องรู้สถานะเปิด / ปิด (3) S4 ใช้ path `POST /orders/{id}/refunds` ตามเดิม แต่บังคับ role `finance`

## Review Focus

1. **Webhook ซ้ำ / พร้อมกัน / ต่าง eventId ของ payment เดียว:** ต้องได้ `order.paid` 1 แถวพอดีและ `sold` เพิ่มครั้งเดียว ถึง mockext จะ retry และกด resend (Task 2 `TestConcurrentWebhooksOneOrderPaid`, `TestWebhookDuplicateEventID`)
2. **จ่ายสำเร็จหลังหมดเวลาจอง (โควตาถูกปล่อยไปแล้ว):** ต้องจองกลับ (`Reacquire`) ถ้ายังว่าง ถ้าเต็มแล้วต้องเป็น `refund_required` ไม่ขายเกิน และไม่ส่ง `order.paid` (Task 2 `TestLatePaidAfterExpiryReacquires`, `TestLatePaidSoldOutRefundRequired`)
3. **Webhook กับงานกระทบยอดยืนยัน payment เดียวกันพร้อมกัน:** ต้องยืนยันครั้งเดียว และงานกระทบยอดสองเครื่องต้องไม่ทำงานซ้อนกัน (Task 4 `TestReconAndWebhookConcurrent`, `TestReconSingleRunner`)
4. **ลายเซ็น Webhook:** ต้องตรวจ HMAC กับ raw body ก่อน decode JSON, เทียบแบบ constant-time, body ที่ถูกแก้ 1 byte หรือ JSON ที่ re-encode แล้วต้องได้ 401 และไม่มีผลใด ๆ (Task 2 `TestWebhookBadSignature`)
5. **คืนเงินเมื่อ VoidTickets ล้มเหลว หรือบัตรยังออกไม่ครบ:** ห้ามเรียก KBank คืนเงิน (ไม่งั้นลูกค้าได้เงินคืนแต่ QR ยังสแกนผ่าน) และกดคืนซ้ำต้องไม่คืนสองครั้ง (Task 5 `TestRefundVoidFailsNoMoneyMoved`, `TestRefundTicketsPending409`, `TestRefundDoubleSubmit409`)

---

## File Structure

```
services/commerce/migrations/00003_payment.sql   payments, payment_idempotency, pg_webhook_events, discrepancies
services/commerce/migrations/00004_refund.sql    refund_policy_tiers, refunds, refund_items, vouchers, voucher_redemptions, audit_log
services/commerce/migrations/00005_queue.sql     queue_state
services/commerce/migrations/00006_receipt.sql   receipts, tax_invoices
services/commerce/clients/kbank.go          PG interface + HTTPPG (contract mockext /pg) + FakePG
services/commerce/clients/ticket.go         TicketClient (gRPC) + fake
services/commerce/payment/create.go         POST /orders/{id}/payments
services/commerce/payment/confirm.go        ConfirmPaid / MarkFailed (ใช้ร่วม webhook + recon)
services/commerce/payment/webhook.go        POST /webhooks/kbank
services/commerce/payment/methods.go        GET /payments/methods
services/commerce/payment/status.go         เติม tickets ใน GET /orders/{id}
services/commerce/recon/recon.go            Reconciler + POST /internal/reconcile
services/commerce/refund/policy.go          Quote + entity refund_policy ของ price.approved
services/commerce/refund/refund.go          POST /orders/{id}/refunds, voucher
services/commerce/audit/audit.go            Write(tx, …) + GET /admin/audit
pkg/chassis/queuetoken/queuetoken.go        Sign / Verify access token (ใช้ร่วมกับ Gateway)
services/commerce/queue/queue.go            บัตรคิว, ปล่อยคิว, API
services/commerce/receipt/receipt.go        consumer order.paid → อีเมลใบเสร็จ
services/commerce/receipt/taxinvoice.go     POST/GET /orders/{id}/tax-invoice
services/commerce/receipt/templates/receipt.{th,en,zh,ja,ko,de,fr}.html, taxinvoice.th.html
api/proto/ticket/v1/ticket.proto            (สร้างเมื่อยังไม่มี ตาม contracts §4)
```

---

### Task 1: สร้างรายการชำระเงิน (`POST /orders/{id}/payments`)

**Files:**
- Create: `services/commerce/migrations/00003_payment.sql`, `services/commerce/clients/kbank.go`, `services/commerce/payment/create.go`
- Modify: `services/commerce/main.go`, `api/openapi/commerce.yaml`, `deploy/compose.yaml` (env commerce: `KBANK_BASE_URL=http://mockext:8090/pg`, `KBANK_API_KEY=demo-key`, `KBANK_WEBHOOK_SECRET=demo-secret`, `KBANK_WEBHOOK_URL=http://commerce:8080/webhooks/kbank`, `PAYMENT_RETURN_ORIGINS=http://localhost:3000`)
- Test: `services/commerce/clients/kbank_test.go`, `services/commerce/payment/create_test.go` (integration)

**Interfaces:**
- Consumes: `order.GetForUpdate`, `order.CanRead`, `order.StatusPendingPayment`, `authn.From`
- Produces:
  - `clients.CreatePaymentReq{Ref string; AmountSatang int64; Method, ReturnURL, WebhookURL string}`, `clients.CreatePaymentRes{PGPaymentID, Status, PaymentURL, PromptPayPayload string}`, `clients.Inquiry{PGPaymentID, Ref, Status string; AmountSatang int64}`, `clients.RefundRes{RefundID, Status string /* succeeded|pending */}`
  - `type clients.PG interface { CreatePayment(ctx, CreatePaymentReq) (CreatePaymentRes, error); Inquire(ctx, ref string) (Inquiry, error); Refund(ctx, pgPaymentID string, amountSatang int64) (RefundRes, error) }` · `clients.HTTPPG{BaseURL, APIKey string; Client *http.Client /* timeout 10s */}` แปลงตาม contract mockext (`POST /pg/payments`, `GET /pg/payments?ref=`, `POST /pg/refunds`, `Authorization: Bearer`) · error `clients.ErrPGNotFound` (404), `clients.ErrPGAmountExceeds` (422), `clients.ErrPGUnavailable` · `clients.FakePG` สำหรับ test
  - payment status: `payment.StatusPending = "pending"`, `StatusPaid = "paid"`, `StatusFailed = "failed"`, `StatusCancelled = "cancelled"`, `StatusSuperseded = "superseded"`, `StatusDiscrepancy = "discrepancy"`
  - HTTP: `POST /orders/{id}/payments {method, returnUrl, voucherCode?}` header `Idempotency-Key?` (เจ้าของออเดอร์หรือ Guest `?key=`) → `201 {paymentId, status, amountSatang, paymentUrl, promptpayPayload?}` · key เดิมของออเดอร์เดียวกันภายใน 24 ชม. → ตอบ payment เดิม (`200`) ไม่ supersede และไม่เรียก KBank ซ้ำ (ตาราง `payment_idempotency (order_id, idem_key, payment_id, created_at, PK(order_id, idem_key))`) · error `404 order_not_found`, `409 order_not_payable` (status ไม่ใช่ pending_payment), `409 hold_expired` (`now >= holdExpiresAt`), `422 method_unavailable`, `422 invalid_return_url` (origin ไม่อยู่ใน `PAYMENT_RETURN_ORIGINS`), `502 pg_unavailable`
- ลำดับ: tx1 ล็อกออเดอร์ ตรวจสถานะ / เวลาจอง → payment เดิมที่ `pending` เป็น `superseded` → insert payment `pending` (`id` UUIDv7 = `ref` ที่ส่ง KBank, `amount_satang` = total − voucher) → COMMIT → เรียก `PG.CreatePayment` (นอก tx) → สำเร็จ update `pg_payment_id`, `payment_url` · ล้มเหลว → `failed` + `order.SetLastPaymentStatus(failed)` แล้วตอบ 502

ตาราง:

```sql
CREATE TABLE payments (id CHAR(36) PRIMARY KEY, order_id CHAR(36) NOT NULL, method VARCHAR(16) NOT NULL,
  amount_satang BIGINT NOT NULL, voucher_code VARCHAR(16) NULL, voucher_satang BIGINT NOT NULL DEFAULT 0,
  status VARCHAR(16) NOT NULL, pg_payment_id VARCHAR(64) NULL UNIQUE, payment_url VARCHAR(512) NULL,
  promptpay_payload TEXT NULL, paid_amount_satang BIGINT NULL, paid_at DATETIME(6) NULL,
  recon_misses TINYINT NOT NULL DEFAULT 0, recon_checked_at DATETIME(6) NULL, created_at DATETIME(6) NOT NULL,
  INDEX idx_order (order_id), INDEX idx_status_created (status, created_at)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE pg_webhook_events (event_id VARCHAR(64) PRIMARY KEY, payment_id CHAR(36) NOT NULL, status VARCHAR(16) NOT NULL,
  received_at DATETIME(6) NOT NULL) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE discrepancies (id CHAR(36) PRIMARY KEY, payment_id CHAR(36) NOT NULL, order_id CHAR(36) NOT NULL,
  expected_satang BIGINT NOT NULL, actual_satang BIGINT NOT NULL, source VARCHAR(16) NOT NULL, created_at DATETIME(6) NOT NULL)
  ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestHTTPPGCreateMapsContract(t *testing.T)      // httptest ตรวจ header Bearer demo-key, body {ref, amount:60000, currency:"THB", method:"promptpay", returnUrl, webhookUrl}
func TestHTTPPGInquire404(t *testing.T)              // 404 {"error":"not_found"} → ErrPGNotFound
func TestCreatePaymentCallsPGWithOrderTotal(t *testing.T) // FakePG ได้ amount == order.totalSatang, ref == paymentId, ตอบ 201 paymentUrl
func TestCreatePaymentHoldExpired409(t *testing.T)   // เลื่อนนาฬิกา 15m → 409 hold_expired, FakePG ไม่ถูกเรียก
func TestCreatePaymentNotOwner(t *testing.T)         // สมาชิกอื่น → 404 order_not_found
func TestCreatePaymentSupersedesPrevious(t *testing.T) // จ่ายครั้งที่ 2 → payment แรกเป็น superseded
func TestReturnURLMustBeAllowed(t *testing.T)        // https://evil.example → 422 invalid_return_url
func TestPGDown502MarksFailed(t *testing.T)          // FakePG คืน ErrPGUnavailable → 502, payment failed, orders.last_payment_status = failed
func TestPaymentIdempotencyKey(t *testing.T)         // Idempotency-Key เดิม 2 ครั้ง → paymentId เดิม, FakePG ถูกเรียก 1 ครั้ง, ไม่มี superseded · หลัง 24h 1s → payment ใหม่
```

- [ ] **Step 2: รันให้ fail** — `make test PKG=./services/commerce/clients/...` และ `make test-int PKG=./services/commerce/payment/...`
- [ ] **Step 3: เขียน `clients/kbank.go`, migration, `payment/create.go`** (method ที่รับได้ใน task นี้คือ `card`, `promptpay` ส่วน Task 7 ขยายเป็นรายการจาก env)
- [ ] **Step 4: รันให้ผ่าน**
- [ ] **Step 5: Commit** — `git commit -m "feat(commerce): create kbank payment for order"`

### Task 2: Webhook ลงลายเซ็น + `ConfirmPaid` + `order.paid`

**Files:**
- Create: `services/commerce/payment/confirm.go`, `services/commerce/payment/webhook.go`
- Modify: `services/commerce/main.go`, `api/openapi/commerce.yaml`
- Test: `services/commerce/payment/webhook_test.go` (integration), `services/commerce/payment/confirm_test.go` (integration)

**Interfaces:**
- Consumes: `order.GetForUpdate`, `order.SetStatus`, `order.SetLastPaymentStatus`, `order.PaidEvent`, `inventory.Commit`, `inventory.Reacquire`, `outbox.Write`, `events.New(events.PaymentDiscrepancy, …)`
- Produces (Task 4, 5 ใช้):
  - `type ConfirmResult string` ค่า `Confirmed`, `AlreadyApplied`, `Discrepancy`, `RefundRequired`, `DuplicatePayment`
  - `payment.ConfirmPaid(ctx context.Context, tx *sql.Tx, paymentID string, paidSatang int64, paidAt time.Time, source string /* webhook|recon|voucher */) (ConfirmResult, error)`
  - `payment.MarkFailed(ctx, tx, paymentID, status string /* failed|cancelled */) error`: payment เป็น status นั้น + `order.SetLastPaymentStatus(failed)` (ออเดอร์ยัง `pending_payment` ลูกค้าลองใหม่ได้ภายในเวลาจองตาม F3)
  - HTTP: `POST /webhooks/kbank` (ไม่ต้อง JWT) header `X-Mock-Signature: hex(HMAC-SHA256(KBANK_WEBHOOK_SECRET, rawBody))` body `{eventId, paymentId, ref, status: paid|failed|cancelled, amount, paidAt}` → `200 {"ok":true}` · `401 invalid_signature` · `404 payment_not_found` (ref ไม่รู้จัก) · `413` เกิน 64 KB
  - payload `payment.discrepancy` ตาม contracts §5 `{paymentId, orderId, expectedSatang, actualSatang}`

`ConfirmPaid` (ลำดับล็อก payment → order เสมอ):

```
SELECT payment FOR UPDATE; ถ้า status ∈ {paid, discrepancy} → AlreadyApplied
SELECT order FOR UPDATE
ถ้า paidSatang != payment.amount_satang → payment=discrepancy, insert discrepancies, outbox payment.discrepancy{expected=amount, actual=paid}, order.last_payment_status=discrepancy → Discrepancy
ถ้า order.status ∈ {confirmed, refunded, refund_required} → payment=paid, outbox payment.discrepancy{expected=0, actual=paid} → DuplicatePayment
ถ้า order.status = pending_payment → inventory.Commit
ถ้า order.status = expired → inventory.Reacquire; *SoldOutError → payment=paid, order=refund_required, log ERROR alert=oncall → RefundRequired (ไม่ส่ง order.paid)
หัก voucher (ถ้ามี, Task 5) · payment=paid, paid_amount, paid_at · order=confirmed · outbox order.paid (order.PaidEvent) → Confirmed
```

Webhook handler: อ่าน body ด้วย `http.MaxBytesReader` → ตรวจ HMAC ด้วย `hmac.Equal` → decode → BEGIN → `INSERT IGNORE pg_webhook_events` (affected 0 = เคยทำแล้ว → COMMIT + 200) → หา payment ด้วย `ref` → `paid` = `ConfirmPaid(source=webhook)` · `failed|cancelled` = `MarkFailed` → COMMIT → 200

- [ ] **Step 1: เขียน test ที่ fail** (ส่ง webhook ด้วย helper ที่ลงลายเซ็นด้วย `demo-secret`)

```go
func TestWebhookBadSignature(t *testing.T)            // ลายเซ็นผิด / body แก้ 1 byte / JSON re-encode ด้วย key สลับลำดับ → 401, ไม่มีแถวใน pg_webhook_events
func TestWebhookPaidConfirmsOrder(t *testing.T)       // order confirmed, payment paid, bucket held→sold, outbox order.paid 1 แถว payload items/holder ตรง contracts
func TestWebhookDuplicateEventID(t *testing.T)        // ส่ง eventId เดิม 3 ครั้ง → 200 ทุกครั้ง, order.paid 1 แถว
func TestConcurrentWebhooksOneOrderPaid(t *testing.T) // 10 goroutine: 5 eventId เดิม + 5 eventId ใหม่ ของ payment เดียว → order.paid 1, sold +n ครั้งเดียว (-race -count=5)
func TestWebhookFailedKeepsOrderPending(t *testing.T) // status failed → payment failed, order pending_payment, last_payment_status failed
func TestWebhookAmountMismatch(t *testing.T)          // amount = expected − 100 → payment discrepancy, outbox payment.discrepancy {expectedSatang, actualSatang}, ไม่มี order.paid
func TestLatePaidAfterExpiryReacquires(t *testing.T)  // ออเดอร์ expired แต่ที่ยังว่าง → confirmed, sold +n, order.paid 1
func TestLatePaidSoldOutRefundRequired(t *testing.T)  // expired และที่เต็มแล้ว → order refund_required, ไม่มี order.paid, held+sold ≤ capacity
func TestSecondPaymentAfterConfirmed(t *testing.T)    // payment ที่ superseded จ่ายมาทีหลัง → DuplicatePayment, payment.discrepancy {expectedSatang:0}
func TestUnknownRef404(t *testing.T)
```

- [ ] **Step 2: รันให้ fail** — `make test-int PKG=./services/commerce/payment/...`
- [ ] **Step 3: เขียน `confirm.go`, `webhook.go`** ตามลำดับข้างบน
- [ ] **Step 4: รันให้ผ่าน** `-race -count=5`
- [ ] **Step 5: ทดสอบกับ mockext**: `make up && make seed` → สร้างออเดอร์ + payment → เปิด `paymentUrl` กด **จ่ายสำเร็จ** → `GET /api/commerce/orders/{id}` เป็น `confirmed` → กด **ส่ง webhook ซ้ำ** ใน Mock Console → `nats stream view EVENTS` มี `evt.order.paid` ของออเดอร์นี้ 1 ข้อความ
- [ ] **Step 6: Commit** — `git commit -m "feat(commerce): signed idempotent kbank webhook and order.paid"`

### Task 3: สถานะออเดอร์หลังจ่าย (GetTickets)

**Files:**
- Create: `services/commerce/clients/ticket.go`, `services/commerce/payment/status.go`, `api/proto/ticket/v1/ticket.proto` (ถ้ายังไม่มี)
- Modify: `services/commerce/order/http.go` (เรียก enricher), `services/commerce/main.go` (env `TICKET_GRPC_ADDR=ticket:9090`), `api/openapi/commerce.yaml`
- Test: `services/commerce/payment/status_test.go` (integration)

**Interfaces:**
- Consumes: gRPC `TicketService.GetTickets`, `VoidTickets` (contracts §4), `order.Get`, `order.CanRead`
- Produces:
  - `clients.Ticket{TicketID, Kind, VisitDate, HolderName, Status, QR string}`
  - `type clients.TicketClient interface { GetTickets(ctx, orderID string) ([]Ticket, bool /*complete*/, error); VoidTickets(ctx, orderID string, orderItemIDs []string /* ว่าง = ทั้งออเดอร์ */, reason string) (int, error) }` + `NewTicketGRPC(addr)` (timeout 3 วินาที) + `clients.FakeTicket`
  - `GET /orders/{id}` ตอบเพิ่ม `payment: {paymentId, method, status}` ล่าสุด และเมื่อ `confirmed`: `tickets: [...]`, `ticketsComplete: bool` · GetTickets error → ยังตอบ 200 ด้วย `tickets: []`, `ticketsComplete: false` (หน้าเว็บ poll ต่อ)

- [ ] **Step 1: เขียน test ที่ fail**: `TestPollPendingNoTickets` (pending → ไม่มี key `tickets`), `TestPollConfirmedWithTickets` (FakeTicket 3 ใบ complete → `ticketsComplete:true` และ `qr` ขึ้นต้น `NN1.`), `TestPollTicketServiceDown` (fake error → 200, `ticketsComplete:false`)
- [ ] **Step 2: รันให้ fail** — `make test-int PKG=./services/commerce/payment/... RUN=TestPoll`
- [ ] **Step 3: สร้าง proto ถ้ายังไม่มี → `make gen` → เขียน client + enricher**
- [ ] **Step 4: รันให้ผ่าน**
- [ ] **Step 5: Commit** — `git commit -m "feat(commerce): order status with issued tickets"`

### Task 4: กระทบยอดรายชั่วโมง + `payment.discrepancy`

**Files:**
- Create: `services/commerce/recon/recon.go`
- Modify: `services/commerce/main.go` (เริ่ม `Reconciler.Run`, route `POST /internal/reconcile` เมื่อ `DEMO_MODE=true`)
- Test: `services/commerce/recon/recon_test.go` (integration)

**Interfaces:**
- Consumes: `clients.PG.Inquire`, `payment.ConfirmPaid(source="recon")`, `payment.MarkFailed`, `order.Expire(ctx, tx, id, "payment_failed")`
- Produces:
  - `recon.Result{Checked, Confirmed, Failed, Discrepancies, Skipped int}`
  - `recon.Reconciler{DB *sql.DB; PG clients.PG; Now core.Clock; Interval time.Duration /* env RECON_INTERVAL ค่าเริ่ม 1h */; MinAge time.Duration /* env RECON_MIN_AGE ค่าเริ่ม 15m ปรับได้เฉพาะ demo / test */}` มี `Tick(ctx) (Result, error)` และ `Run(ctx) error` · `ErrLocked` เมื่ออีกเครื่องกำลังทำ
  - `POST /internal/reconcile` → `200 Result` (ให้ demo กดได้ทันทีไม่ต้องรอชั่วโมง)

Tick: `SELECT GET_LOCK('commerce_recon', 0)` (ได้ 0 → `ErrLocked`) → เลือก (ก) `status='pending' AND pg_payment_id IS NOT NULL AND created_at < now − MinAge` และ (ข) `status='paid' AND paid_at >= now − 75m AND recon_checked_at IS NULL` → ต่อรายการ `PG.Inquire(ref=payment.id)` นอก tx แล้วเปิด tx สั้นต่อรายการ:
- KBank `paid` + ระบบ `pending` → `ConfirmPaid` (ยอดไม่ตรงจะกลายเป็น discrepancy ในฟังก์ชันเดียวกัน)
- KBank `failed|cancelled` + ระบบ `pending` → `MarkFailed` แล้ว `order.Expire(…, "payment_failed")` (ข้ามถ้า `ErrNotPending`)
- ระบบ `paid` แต่ amount ของ KBank ไม่ตรง `paid_amount_satang` → payment `discrepancy` + outbox `payment.discrepancy`
- `ErrPGNotFound` → `recon_misses + 1` ครบ 3 → ทำแบบ failed
- ทุกรายการที่ตรวจแล้วตั้ง `recon_checked_at` · `RELEASE_LOCK` ตอนจบ

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestReconPaidWithoutWebhookConfirms(t *testing.T) // FakePG paid, payment pending อายุ 16m → Confirmed 1, order.paid 1
func TestReconSkipsYoungPending(t *testing.T)          // อายุ 14m → Checked 0
func TestReconFailedReleasesQuota(t *testing.T)        // FakePG failed → payment failed, order expired, outbox order.expired {reason:"payment_failed"}, held คืน
func TestReconAmountMismatch(t *testing.T)             // pending แต่ KBank amount ขาด 100 → Discrepancies 1, outbox payment.discrepancy
func TestReconPaidAmountRecheck(t *testing.T)          // paid ชั่วโมงก่อน แต่ inquiry amount ต่าง → discrepancy
func TestRecon404ThreeMissesFails(t *testing.T)        // Tick 3 รอบได้ ErrPGNotFound → รอบที่ 3 payment failed
func TestReconAndWebhookConcurrent(t *testing.T)       // Tick กับ webhook paid ของ payment เดียวพร้อมกัน → order.paid 1 แถว
func TestReconSingleRunner(t *testing.T)               // 2 Tick พร้อมกัน → หนึ่งตัวได้ ErrLocked
```

- [ ] **Step 2: รันให้ fail** — `make test-int PKG=./services/commerce/recon/...`
- [ ] **Step 3: เขียน `recon.go`** (`GET_LOCK` ต้องใช้ `*sql.Conn` ตัวเดียวตลอด Tick)
- [ ] **Step 4: รันให้ผ่าน** `-race`
- [ ] **Step 5: ทดสอบกับ mockext**: กด **จ่ายสำเร็จแต่ไม่ส่ง webhook** → รอ 15 นาที (หรือ `RECON_MIN_AGE=1m` ใน compose override) → `POST http://commerce:8080/internal/reconcile` (ผ่าน `docker compose exec`) → ออเดอร์เป็น `confirmed` · กด **ยอดไม่ตรง** → ได้ `evt.payment.discrepancy`
- [ ] **Step 6: Commit** — `git commit -m "feat(commerce): hourly reconciliation with kbank inquiry"`

### Task 5: คืนเงิน (Finance) + Voucher + Audit log

**Files:**
- Create: `services/commerce/migrations/00004_refund.sql`, `services/commerce/refund/policy.go`, `services/commerce/refund/refund.go`, `services/commerce/audit/audit.go`
- Modify: `services/commerce/payment/create.go` (รับ `voucherCode`), `services/commerce/payment/confirm.go` (หัก voucher), `services/commerce/seed/data.go` (ลงทะเบียน `refund_policy`), `api/openapi/commerce.yaml`
- Test: `services/commerce/refund/policy_test.go`, `services/commerce/refund/refund_test.go` (integration), `services/commerce/payment/voucher_test.go` (integration)

**Interfaces:**
- Consumes: `authn.RequireRole("finance")`, `clients.TicketClient`, `clients.PG.Refund`, `order.GetForUpdate`, `order.SetStatus`, `outbox.Write`, `events.New(events.OrderRefunded, …)`
- Produces:
  - `audit.Write(ctx, tx *sql.Tx, p authn.Principal, action, entity, entityID string, before, after any, reason string) error` (ตาราง `audit_log` คอลัมน์ `id, at, actor_id, actor_roles, action, entity, entity_id, before_json, after_json, reason`, Task 6 ใช้ด้วย) และ `GET /admin/audit?entity=&entityId=&cursor=` (role `admin|finance|viewer`) คืน `{items: [{id, at, actorId, actorRoles, action, entity, entityId, before, after, reason?}], nextCursor}` ตาม contracts หัวข้อ 1
  - `refund.Tier{MinHoursBefore int; MoneyFeePercent, VoucherFeePercent int; AllowMoney, AllowVoucher bool}` (เรียง `MinHoursBefore` มากไปน้อย)
  - **นโยบายคืนเงินมาจาก `price.approved`** (contracts หัวข้อ 5 entity `refund_policy`): `refund.RegisterEntity()` เรียก `catalog.RegisterEntity("refund_policy", …)` ของ Plan 05 · data `{tiers: [{minHoursBefore, moneyFeePercent, voucherFeePercent, allowMoney, allowVoucher}]}` · `upsert` แทนที่ tier ทั้งชุด (ตาราง `refund_policy_tiers` + คอลัมน์ `approval_id` กันการอนุมัติเก่ามาทับ) · `delete` = ไม่มี tier (คืนเงินไม่ได้จนกว่าจะอนุมัติชุดใหม่) · percent นอก 0–100 หรือ `minHoursBefore` ติดลบ = `ErrInvalidData` · `GET /admin/refund-policy` (role `admin|finance|editor|approver|viewer`) คืน `{tiers, approvalId, updatedAt}` ปัจจุบัน ให้ Plan 14 แสดงก่อนส่งแก้ผ่าน approval (test `TestAdminRefundPolicyRead`)
  - `refund.Quote(tiers []Tier, o order.Order, itemIDs []string /* ว่าง = ทั้งออเดอร์ */, mode string, now time.Time) (QuoteResult, error)` → `QuoteResult{RefundableSatang, FeeSatang, AmountSatang int64}` (`RefundableSatang` = ผลรวม `priceSatang` ของ item ที่เลือก) · ชั่วโมงก่อนเข้าชมนับถึง `visitDate` 08:00 น. เวลาไทย, ออเดอร์ annual อย่างเดียวนับเป็นไม่จำกัด, `refund_required` = ค่าธรรมเนียม 0 เสมอ · `ErrNotRefundable`, `ErrModeNotAllowed` · fee = `floor(total × percent / 100)`
  - HTTP (role `finance`):
    - `GET /orders/{id}/refunds/quote?mode=money|voucher&orderItemIds=` → `QuoteResult`
    - `POST /orders/{id}/refunds {mode, reason: customer_request|event_cancelled|duplicate_payment|refund_required|other, note?, amountSatang?, orderItemIds?}` (`orderItemIds` ว่าง = ทั้งออเดอร์) → `201 {refundId, status, amountSatang, feeSatang, voucherCode?}` · error `403`, `409 order_not_refundable` (สถานะไม่ใช่ confirmed / refund_required), `409 refund_exists` (item ซ้ำกับ refund อื่นที่ยังไม่ `failed`), `422 unknown_order_item`, `409 tickets_pending` (GetTickets ยังไม่ complete), `409 ticket_used`, `422 amount_exceeds_refundable`, `422 mode_not_allowed`, `502 void_failed`, `502 pg_refund_failed`
    - `POST /refunds/{id}:retry` (role `finance`) ทำต่อจากขั้นที่ค้าง (`pending` → void, `voided|processing` → PG)
  - refund status: `pending` → `voided` → `succeeded` | `processing` (KBank ตอบ 202) | `failed`
  - Voucher: code 12 ตัว (A–Z 2–9 ไม่มี O/0/I/1 จาก `crypto/rand`), `balance_satang`, เจ้าของ = memberId หรือ guestContact ของออเดอร์, หมดอายุ 365 วัน
  - `POST /orders/{id}/payments` รับ `voucherCode?`: ตรวจเจ้าของ / อายุ / ยอดคงเหลือ แล้วใช้ `min(balance, total)` ถ้าครอบคลุมทั้งออเดอร์ให้สร้าง payment `method=voucher` และเรียก `ConfirmPaid(source="voucher")` ทันทีไม่ผ่าน KBank · `ConfirmPaid` หัก `balance_satang` ด้วย `SELECT … FOR UPDATE` ถ้าไม่พอแล้วให้ถือเป็น Discrepancy · error `422 voucher_invalid|voucher_expired`

ลำดับคืนเงิน: tx1 ล็อกออเดอร์ → ตรวจสถานะ / refund ที่ยังไม่ `failed` ที่ item ซ้ำกัน (มี = `refund_exists`) → `GetTickets` (ไม่ complete = `tickets_pending`, บัตรของ item ที่เลือกเป็น `used` = `ticket_used`) → `Quote` → insert refund `pending` + `refund_items` + `audit.Write("refund.requested")` → COMMIT → `VoidTickets(orderId, orderItemIds, reason)` (ล้มเหลว = 502 `void_failed`, refund ค้าง `pending` ไม่เรียก PG) → refund `voided` → money: `PG.Refund(pg_payment_id, amount)` (`succeeded` / `pending` → `processing`) · voucher: สร้าง voucher → tx สุดท้าย: refund `succeeded`, order `refunded` (เมื่อทุก item ถูกคืนแล้ว ไม่งั้นยัง `confirmed`), outbox `order.refunded {orderId, refundId, memberId?, guestContact?, lang, amountSatang, mode, orderItemIds, reason}` (ตาม contracts หัวข้อ 5 · `orderItemIds` = ที่ส่งมา ว่าง = ทั้งออเดอร์ · Notification Plan 10 แจ้งลูกค้าจาก event นี้โดยตรง), `audit.Write("refund.succeeded")`

seed `refund_policy`: สร้าง `PriceApproved` ที่มี change `refund_policy` (approvalId คงที่จาก `deploy/seed/fixtures.json`) แล้วเรียก `catalog.Apply` แบบเดียวกับ catalog ของ Plan 05 · tier `{24, 0, 0, true, true}`, `{0, 20, 0, true, true}` (หลังเวลาเข้าชมไม่มี tier = คืนไม่ได้)

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestQuoteTiers(t *testing.T)                // total 60000 visit 2027-03-10: now 2027-03-08T00:00Z (≥24h) money fee 0 · now 2027-03-09T20:00Z (<24h) money fee 12000 amount 48000, voucher fee 0 · now 2027-03-10T02:00Z → ErrNotRefundable
func TestRefundRequiresFinanceRole(t *testing.T) // role admin → 403
func TestRefundVoidsBeforePG(t *testing.T)       // บันทึกลำดับเรียก fake: VoidTickets ก่อน PG.Refund
func TestRefundVoidFailsNoMoneyMoved(t *testing.T) // VoidTickets error → 502 void_failed, PG ไม่ถูกเรียก, refund pending, :retry หลัง fake หายแล้ว → succeeded
func TestRefundTicketsPending409(t *testing.T)   // GetTickets complete=false → 409 tickets_pending, ไม่มีแถว refunds
func TestRefundTicketUsed409(t *testing.T)
func TestRefundPolicyFromPriceApproved(t *testing.T) // price.approved refund_policy tier {48, 50, 0} → Quote ใช้ชุดใหม่ · approval เก่ามาทีหลังไม่ทับ · percent 101 → event rollback
func TestRefundMoneySuccess(t *testing.T)        // order refunded, outbox order.refunded {memberId, lang, mode:"money", orderItemIds:[]} ตรง contracts, audit_log 2 แถว actorId = X-User-Sub, actorRoles ["finance"]
func TestPartialRefundVoidsOnlyItems(t *testing.T) // ออเดอร์ 3 ใบ คืน 1 item → VoidTickets ได้ order_item_ids 1 ตัว, amount = ราคาใบนั้น, order ยัง confirmed, order.refunded.orderItemIds = [id] · คืน item เดิมซ้ำ → 409 refund_exists
func TestAdminAuditFormat(t *testing.T)          // GET /admin/audit?entity=refund&entityId= → item มี key ตาม contracts หัวข้อ 1
func TestRefundVoucherIssuesCode(t *testing.T)   // mode voucher → voucherCode 12 ตัว balance = amount, ไม่เรียก PG
func TestRefundDoubleSubmit409(t *testing.T)     // 2 คำขอพร้อมกัน → 201 หนึ่ง, 409 refund_exists หนึ่ง
func TestRefundAmountExceeds422(t *testing.T)
func TestRefundPGPendingProcessing(t *testing.T) // pg payment id ลงท้าย -slow → status processing, ยังไม่มี order.refunded
func TestVoucherPartial(t *testing.T)            // voucher 20000 กับออเดอร์ 60000 → PG amount 40000, ยืนยันแล้ว balance 0
func TestVoucherCoversFullOrder(t *testing.T)    // voucher 100000 → confirmed ทันที ไม่เรียก PG, balance 40000, order.paid paymentId = payment ของ voucher
```

- [ ] **Step 2: รันให้ fail** — `make test PKG=./services/commerce/refund/... RUN=TestQuote` และ `make test-int PKG=./services/commerce/refund/... ./services/commerce/payment/...`
- [ ] **Step 3: เขียน migration, `audit`, `refund`, ส่วน voucher ใน payment**
- [ ] **Step 4: รันให้ผ่าน** `-race`
- [ ] **Step 5: Commit** — `git commit -m "feat(commerce): finance refunds with void-first, vouchers, audit log"`

### Task 6: Waiting room

**Files:**
- Create: `pkg/chassis/queuetoken/queuetoken.go`, `services/commerce/migrations/00005_queue.sql`, `services/commerce/queue/queue.go`
- Modify: `services/commerce/main.go` (เริ่ม `Releaser.Run`), `services/commerce/seed/data.go` (ลงทะเบียน `queue`: ปิดอยู่, capacity 300 / นาที), `deploy/compose.yaml` (env commerce `QUEUE_TOKEN_KEY`, `QUEUE_TICKET_SECRET` และ gateway `QUEUE_ACCESS_PUBLIC_KEY` ค่า demo), `api/openapi/commerce.yaml`
- Test: `pkg/chassis/queuetoken/queuetoken_test.go`, `pkg/chassis/queuetoken/testdata/vectors.json`, `services/commerce/queue/queue_test.go` (integration)

**Interfaces (สัญญากับ Plan 12 Gateway และ Plan 13 / 15 หน้ารอคิว ตาม contracts หัวข้อ 8):**
- Produces:
  - **Access token:** `WR1.<b64url(payloadJSON)>.<b64url(ed25519 signature ของ "WR1." + b64url(payloadJSON))>` (b64url ไม่มี padding) payload `{"s":<seq int64>,"exp":<unix วินาที>,"dev":"<X-Device-Id หรือ ''>"}`
  - `queuetoken.Claims{Seq int64; Exp int64; Device string}` · `queuetoken.Sign(priv ed25519.PrivateKey, c Claims) (string, error)` · `queuetoken.Verify(token string, pub ed25519.PublicKey, now time.Time) (Claims, error)` (signature ตาม contracts หัวข้อ 8 ทุกตัวอักษร) ยอมเวลาคลาดเคลื่อน 30 วินาที · `func (c Claims) CheckDevice(deviceID string) error` ถ้า `dev` ไม่ว่างต้องตรงกับ `deviceID` · error `ErrMalformed`, `ErrBadSignature`, `ErrExpired`, `ErrDeviceMismatch` · `queuetoken.ParsePrivateKeyPEM`, `ParsePublicKeyPEM` (PKCS8 / SPKI)
  - env: commerce `QUEUE_TOKEN_KEY` (Ed25519 private PEM, contracts หัวข้อ 9), `QUEUE_ACCESS_TTL` (ค่าเริ่ม `10m`), `QUEUE_TICKET_SECRET`, `QUEUE_TICK` (`5s`) · **gateway** `QUEUE_ACCESS_PUBLIC_KEY` (Ed25519 public PEM) และ `QUEUE_PROTECTED_ROUTES` (ค่าเริ่ม `POST /api/commerce/orders`, คั่นด้วย `,` ใส่หน้าสมัครหรือ route อื่นได้เมื่อ "ตั้งค่าให้รวม")
  - **Gateway ทำ (Plan 12):** route ที่อยู่ใน `QUEUE_PROTECTED_ROUTES` ต้องมี header `X-Queue-Token` → `queuetoken.Verify(token, pub, now)` แล้ว `claims.CheckDevice(X-Device-Id)` ไม่ผ่านตอบ `403 {"error":"queue_token_required"|"queue_token_invalid"|"queue_token_expired"}` ก่อน route (`ErrDeviceMismatch` / `ErrMalformed` / `ErrBadSignature` = `queue_token_invalid`) · route อื่นไม่ตรวจ (สแกนประตูและสมัครสมาชิกไม่ถูกบล็อก)
  - **บัตรคิว (ตรวจเฉพาะใน commerce):** `QT1.<b64url({"s":seq,"e":epoch,"i":issuedAtUnix})>.<b64url(HMAC-SHA256(QUEUE_TICKET_SECRET, "QT1." + payloadB64))>` อายุ 2 ชม.
  - HTTP สาธารณะ (header `X-Device-Id` ถ้ามี):
    - `GET /queue/status` header `X-Queue-Ticket?` → ปิด: `{enabled:false, accessToken, expiresAt}` · เปิด: `{enabled:true, queueTicket, position, etaSeconds, pollAfterSeconds:15}` และถ้าถูกปล่อยแล้วเพิ่ม `accessToken, expiresAt` (ส่งบัตรคิวที่ใช้ได้มา = ไม่ออกเลขใหม่)
    - `GET /queue/position` header `X-Queue-Ticket` → `{position, etaSeconds, pollAfterSeconds, accessToken?, expiresAt?}` · `401 queue_ticket_invalid` · `410 queue_ticket_stale` (epoch เก่าหรืออายุเกิน 2 ชม. ให้เรียก status ใหม่)
  - Admin (role `admin`): `GET /admin/queue` → `{enabled, capacityPerMinute, issued, releasedUpTo, waiting}` · `PUT /admin/queue {enabled, capacityPerMinute}` มีผลทันที, เปิดจากปิด = `epoch + 1`, `audit.Write("queue.updated")`
  - `queue.Releaser{DB; Now; Tick time.Duration}` มี `Tick(ctx) error`: ล็อก `queue_state` → `add = floor(capacity × elapsed / 60s)` → `released_up_to = min(released_up_to + add, next_seq − 1)` → เลื่อน `last_release_at` เท่าเวลาที่ใช้ไปจริง (`add × 60s / capacity`) เพื่อไม่ทิ้งเศษ
  - `position = max(0, seq − released_up_to)`, `etaSeconds = ceil(position / capacity × 60)` · ออกเลขคิว: `UPDATE queue_state SET next_seq = LAST_INSERT_ID(next_seq + 1) WHERE id = 1`

```sql
CREATE TABLE queue_state (id TINYINT PRIMARY KEY, enabled BOOL NOT NULL, capacity_per_minute INT NOT NULL, epoch INT NOT NULL,
  next_seq BIGINT NOT NULL, released_up_to BIGINT NOT NULL, last_release_at DATETIME(6) NOT NULL,
  updated_by VARCHAR(64) NULL, updated_at DATETIME(6) NOT NULL) ENGINE=InnoDB;
```

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestSignVerifyRoundTrip(t *testing.T)          // queuetoken
func TestTamperedPayloadRejected(t *testing.T)      // เปลี่ยน s ใน payload → ErrBadSignature
func TestExpiredBeyondLeeway(t *testing.T)          // now = exp + 31s → ErrExpired, exp + 29s → ผ่าน
func TestDeviceMismatch(t *testing.T)               // claims.CheckDevice("other") → ErrDeviceMismatch, dev ว่าง → ผ่าน
func TestStatusDisabledGivesTokenImmediately(t *testing.T)
func TestEnabledIssuesUniqueSequentialSeq(t *testing.T) // 1000 goroutine เรียก status → seq 1..1000 ไม่ซ้ำ ไม่ขาด
func TestReleaseByCapacity(t *testing.T)            // capacity 60 / นาที, issued 100, Tick หลัง 10s → releasedUpTo 10, หลังอีก 0.5s → ยัง 10, อีก 0.5s → 11
func TestReleaseCappedAtIssued(t *testing.T)        // issued 5 รอ 10 นาที → releasedUpTo 5 และคนที่ 6 ไม่ได้ผ่านทันที
func TestPositionReleasedReturnsAccessToken(t *testing.T) // token ที่ได้ Verify ผ่านด้วย public key คู่กัน
func TestForgedQueueTicket401(t *testing.T)
func TestEpochChangeStale410(t *testing.T)          // ปิดแล้วเปิดใหม่ → บัตรเดิม 410
func TestAdminToggleRequiresAdminAndAudits(t *testing.T) // role finance → 403, admin → 200 + audit_log 1 แถว
func TestQueueTokenVectors(t *testing.T)            // pkg/chassis/queuetoken/testdata/vectors.json (key seed คงที่, schema ตาม Plan 12 Task 2 ถ้าไฟล์มีแล้วใช้ต่อไม่สร้างใหม่) ผ่านทุก case ให้ Plan 12 ใช้ไฟล์เดียวกัน
```

- [ ] **Step 2: รันให้ fail** — `make test PKG=./pkg/chassis/queuetoken/...` และ `make test-int PKG=./services/commerce/queue/...`
- [ ] **Step 3: เขียน `queuetoken` แล้ว `queue`** ตาม Interfaces
- [ ] **Step 4: รันให้ผ่าน** `-race`
- [ ] **Step 5: Commit** — `git commit -m "feat(commerce): waiting room with signed queue tickets and gateway access token"`

### Task 7: ช่องทางจ่ายต่างชาติ (Alipay+ / WeChat Pay / UnionPay)

**Files:**
- Create: `services/commerce/payment/methods.go`
- Modify: `services/commerce/payment/create.go`, `deploy/compose.yaml` (env commerce `PAYMENT_METHODS=card,promptpay,alipayplus,wechatpay,unionpay` และ service `gotest` เพิ่ม `TEST_KBANK_BASE_URL=http://mockext:8090/pg` + `depends_on: mockext`), `api/openapi/commerce.yaml`
- Test: `services/commerce/payment/methods_test.go`, `services/commerce/payment/methods_mockext_test.go` (integration)

**Interfaces:**
- Consumes: `clients.HTTPPG` (Task 1)
- Produces:
  - `payment.Method{Method, Kind string /* card|qr|wallet */}` · `payment.MethodsFor(lang string, enabled []string) []Method` ลำดับ: `zh` → `wechatpay, alipayplus, unionpay, card, promptpay` · `th` → `promptpay, card, …` · ภาษาอื่น → `card, alipayplus, wechatpay, unionpay, promptpay` ตัดตัวที่ไม่อยู่ใน `enabled` ออก (UPI เพิ่มได้ทาง env เมื่อ Gateway รองรับ ไม่ต้องแก้โค้ด)
  - `GET /payments/methods?lang=` → `{"items":[Method]}` (ชื่อแสดงผลแปลฝั่ง frontend จาก `method`)
  - `POST /orders/{id}/payments` ตรวจ `method` กับ `PAYMENT_METHODS` (`422 method_unavailable`) และส่งไป KBank ตามชื่อเดิม สกุลเงินเป็น `THB` เสมอ

- [ ] **Step 1: เขียน test ที่ fail**: `TestMethodsOrderByLang` (zh ตัวแรก `wechatpay`, th ตัวแรก `promptpay`), `TestDisabledMethod422` (`PAYMENT_METHODS=card` แล้วส่ง `alipayplus` → 422), `TestForeignMethodsAgainstMockext` (integration ใช้ `TEST_KBANK_BASE_URL`: `alipayplus`, `wechatpay`, `unionpay` ได้ 201 และ `paymentUrl` ไม่ว่าง · ไม่มี env = `t.Fatal` ห้าม skip)
- [ ] **Step 2: รันให้ fail** — `make test PKG=./services/commerce/payment/... RUN=TestMethods` และ `make test-int PKG=./services/commerce/payment/... RUN=TestForeignMethods`
- [ ] **Step 3: เขียน `methods.go` และตรวจใน `create.go`**
- [ ] **Step 4: รันให้ผ่าน**
- [ ] **Step 5: Commit** — `git commit -m "feat(commerce): foreign payment methods via kbank"`

### Task 8: ใบเสร็จอิเล็กทรอนิกส์ + ขอใบกำกับภาษี

**Files:**
- Create: `services/commerce/migrations/00006_receipt.sql`, `services/commerce/receipt/receipt.go`, `services/commerce/receipt/taxinvoice.go`, `services/commerce/receipt/templates/receipt.{th,en,zh,ja,ko,de,fr}.html`, `services/commerce/receipt/templates/taxinvoice.th.html`
- Modify: `services/commerce/main.go` (consumer `commerce-receipt`, env `SMTP_ADDR=mailpit:1025`, `MAIL_FROM=no-reply@nongnooch.local`), `api/openapi/commerce.yaml`
- Test: `services/commerce/receipt/receipt_test.go` (integration), `services/commerce/receipt/taxinvoice_test.go`

**Interfaces:**
- Consumes: `consumer.Run` (Config.Name = `commerce-receipt`, EventTypes = `[events.OrderPaid]`), `docno.Next(ctx, tx, "RC"|"TI", now)`, `order.Get`, `clients.MemberClient` + เพิ่มเมธอด `GetContactPrefs(ctx, memberID) (ContactPrefs{Lang, Email, Phone string}, error)` (contracts §4)
- Produces:
  - `type receipt.Mailer interface { Send(ctx, to, subject, htmlBody string) error }` + `SMTPMailer{Addr, From string}` · `receipt.FakeMailer`
  - `receipt.VATSatang(totalSatang int64) int64` = `round(total × 7 / 107)` (half-up)
  - `receipt.Handler(m Mailer, members clients.MemberClient) consumer.Handler`: insert `receipts (order_id UNIQUE, receipt_no, sent_to, created_at)` → render template ตาม `lang` ของ event (ไม่มีใช้ `en`) → ส่งถึงอีเมลของสมาชิก (`GetContactPrefs`) หรือ `guestContact` ที่เป็นอีเมล · ไม่มีอีเมล = ไม่ส่งแต่ยังบันทึก receipt · ส่งไม่สำเร็จคืน error ให้ redeliver
  - `receipt.ValidTaxID(id string) bool`: 13 หลัก, check digit = `(11 − (Σ dᵢ × (13 − i), i = 0..11) mod 11) mod 10`
  - HTTP: `GET /orders/{id}/receipt` (สิทธิ์ตาม `order.CanRead`) → HTML · `POST /orders/{id}/tax-invoice {buyerName, taxId, branchCode /* "00000" = สำนักงานใหญ่ */, address, email}` → `201 {taxInvoiceNo}` และส่งอีเมล · `GET /orders/{id}/tax-invoice` → HTML · error `409 order_not_confirmed`, `409 tax_invoice_exists`, `422 invalid_tax_id`
  - ใบกำกับภาษีมีรายการ, ราคารวม VAT, `vatSatang`, ผู้ขาย (ค่าจาก env `SELLER_NAME`, `SELLER_TAX_ID`, `SELLER_ADDRESS`) และหมายเหตุ "[รูปแบบรอยืนยันจากฝ่ายบัญชี]" ใน footer ของ demo

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestVAT(t *testing.T)                        // 60000 → 3925, 10700 → 700
func TestValidTaxID(t *testing.T)                 // "1234567890121" true, "1234567890122" false, "12345" false
func TestReceiptSentOnce(t *testing.T)            // publish order.paid id เดียวกัน 2 ครั้ง → FakeMailer 1 ครั้ง, receipt_no "RC" + 12 หลัก
func TestReceiptLangFallbackEN(t *testing.T)      // lang ko ใช้ receipt.ko.html · lang ที่ไม่มีไฟล์ → en
func TestReceiptGuestPhoneOnlyNoSend(t *testing.T) // guestContact "+66…" → ไม่ส่ง, แถว receipts มี sent_to ว่าง
func TestTaxInvoiceOncePerOrder(t *testing.T)     // ครั้งที่ 2 → 409 tax_invoice_exists
func TestTaxInvoiceOnlyConfirmed(t *testing.T)    // pending_payment → 409 order_not_confirmed
func TestAllReceiptTemplatesRender(t *testing.T)  // 7 ภาษา render ได้ มี orderNo และยอดเป็นบาท "600.00"
```

- [ ] **Step 2: รันให้ fail** — `make test PKG=./services/commerce/receipt/... RUN='TestVAT|TestValidTaxID'` และ `make test-int PKG=./services/commerce/receipt/...`
- [ ] **Step 3: เขียน migration, `receipt`, `taxinvoice`, templates**
- [ ] **Step 4: รันให้ผ่าน** แล้ว `make test && make test-int PKG=./services/commerce/... && make build SVC=commerce`
- [ ] **Step 5: ทดสอบปลายทางบน local**: ซื้อแบบ Guest ด้วยอีเมล → จ่ายด้วย `alipayplus` ใน mockext → เห็นใบเสร็จที่ Mailpit `http://localhost:8025` → ขอใบกำกับภาษีด้วย `1234567890121` → เห็นอีเมลที่สอง
- [ ] **Step 6: Commit** — `git commit -m "feat(commerce): e-receipt and tax invoice by email"`

---

## ข้อเสนอแก้ contracts

รวมเข้า contracts v2 แล้ว (หัวข้อ 8 waiting room token, หัวข้อ 5 entity `refund_policy`, หัวข้อ 2 route สาธารณะของ webhook / queue)

ที่ยังไม่ได้รับเข้า: env `QUEUE_TICKET_SECRET` (HMAC ของบัตรคิว `QT1` ใช้ภายใน commerce เท่านั้น) ยังไม่อยู่ใน contracts หัวข้อ 9 (Plan 16 Task 1 สร้างให้แล้ว)

## คำถามเปิด

- KBank refund แบบ async (`202 pending`): ของจริงแจ้งผลทาง webhook หรือต้อง poll (Q4) แผนนี้ให้ Finance กด `:retry` ไปก่อน
- นโยบายค่าธรรมเนียมคืนเงิน (#17) และอายุ Voucher (#29) ยังเป็นค่า demo
- รูปแบบใบกำกับภาษีเต็มรูปจากฝ่ายบัญชี และเลขที่เอกสารต้องเรียงต่อเนื่องรายปี / รายสาขาหรือไม่
- ค่าตั้งต้นของ Waiting room (capacity ต่อนาที, ช่วง poll) และยอดพีคที่ใช้คำนวณ Load test 3 เท่า (Q9) ส่วน load test จริงอยู่ในงาน QA 1.17
