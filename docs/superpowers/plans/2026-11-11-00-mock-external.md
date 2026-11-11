# Mock External Systems Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** service `mockext` ตัวเดียว จำลองระบบภายนอกทุกตัวที่ demo ต้องใช้ ได้แก่ KBank PG, Turnstile, SMS, Social login (LINE / Google / Apple / WeChat), FCM / Huawei Push Kit และระบบประตูเดิม พร้อมหน้า **Mock Console** ให้คนทำ demo ดู OTP, การจ่ายเงิน, Push และข้อมูลที่ส่งไประบบเดิม

**Architecture:** เป็น Go binary (`services/mockext`) ที่ใช้ `server` จาก chassis (Plan 01) แต่ไม่ใช้ DB, outbox หรือ NATS ข้อมูลเก็บใน memory ทุก mock ใช้ protocol HTTP ใกล้เคียงของจริง service ของเราจึงเรียกผ่าน base URL ใน env (เช่น `KBANK_BASE_URL`) เมื่อจะต่อระบบจริงให้เขียน adapter จริงมาแทน โดยไม่ต้องแก้ domain code ส่วนอีเมลใช้ **Mailpit** (container) แทนการเขียน mock เอง

**Tech Stack:** Go 1.23 (`net/http`, `html/template`, `crypto/hmac`, `crypto/ed25519`), `github.com/golang-jwt/jwt/v5`, Mailpit `axllent/mailpit`, chassis `pkg/chassis/server`, `pkg/chassis/config`

**Spec:** [docs/system-design.md](../../system-design.md) O1 (ระบบภายนอก), S2, S3, S4, S5, S7 · ดัชนี: [2026-11-11-phase1-index.md](2026-11-11-phase1-index.md) หัวข้อ "โหมด Demo"

## Global Constraints

- **Build และ test ใน Docker เสมอ (มติ 11 พ.ย. 2026):** ห้ามรัน `go build` / `go test` / `go run` บนเครื่องโดยตรง ให้ใช้ `make test`, `make test-int`, `make build` ที่รันใน container `golang:1.23-bookworm` คู่กับ MariaDB / NATS ใน `deploy/compose.yaml` (กำหนดใน Plan 01 Task 1)
- **มติ 11 พ.ย. 2026: งานนี้เป็น demo ระบบภายนอกทุกตัวต้อง mock ขึ้นมาเอง** ห้ามเรียกบริการภายนอกจริงจากโค้ด demo
- ทุก service ของเราเรียกระบบภายนอกผ่าน interface + base URL ที่ตั้งใน env ห้าม hardcode URL ของ mockext
- ข้อกำหนดทั้งหมดใน Global Constraints ของ Plan 01 ใช้กับ service ของเรา ส่วน mockext ได้รับยกเว้นเรื่อง DB / outbox
- Webhook ของ KBank ต้องลงลายเซ็น และทดสอบการส่งซ้ำได้ (S3: "Webhook (signed)", "eventId เคยประมวลผลแล้ว")
- WeChat ไม่ให้อีเมล / เบอร์ และผูกบัญชีด้วย UnionID (S2)
- Push: ระบบต้องลบ token ที่ใช้ไม่ได้แล้ว (S7 ขั้นสุดท้าย)

## Review Focus

1. **Webhook ส่งซ้ำและสลับลำดับ:** Console ต้องกด "ส่ง webhook ซ้ำ" ได้ เพื่อให้ demo และ QA พิสูจน์ได้ว่า Commerce ไม่ออกบัตรซ้ำ (Task 4)
2. **Webhook ไม่มาเลย:** ต้องมีตัวเลือก "จ่ายสำเร็จแต่ไม่ส่ง webhook" เพื่อทดสอบงานกระทบยอดรายชั่วโมง (S4) ซึ่ง inquiry API ต้องตอบ `paid` (Task 4)
3. **Restart mockext ระหว่าง demo:** ข้อมูลใน memory หายไป inquiry ของ payment เก่าต้องตอบ `404 not_found` ไม่ใช่ 500 เพื่อให้ Commerce จัดการต่อได้ (Task 1, 4)
4. **ภาษาไทย / จีนใน SMS และ Push:** Console ต้องแสดงข้อความ 7 ภาษาได้ถูกต้อง (`charset=utf-8`) (Task 2, 5)
5. **Token push ที่ใช้ไม่ได้แล้ว:** token ที่ขึ้นต้นด้วย `invalid-` ต้องได้ 404 `UNREGISTERED` เหมือน FCM จริง เพื่อทดสอบการลบ token (Task 5)

---

## File Structure

```
services/mockext/main.go           ประกอบ handler ทุกตัว, PORT ค่าเริ่ม :8090
services/mockext/store.go          Store ใน memory (ring buffer ต่อหมวด)
services/mockext/console.go        GET /  หน้า Mock Console (html/template, refresh ทุก 3 วิ)
services/mockext/turnstile.go      POST /turnstile/v0/siteverify
services/mockext/sms.go            POST /sms/send
services/mockext/oauth.go          OIDC: LINE / Google / Apple · OAuth: WeChat
services/mockext/kbank.go          PG: payments, hosted checkout, webhook, inquiry, refund
services/mockext/push.go           POST /push/fcm, /push/hms
services/mockext/legacy.go         POST /legacy/gate-events, /legacy/tickets
services/mockext/templates/*.html
deploy/compose.yaml                (แก้) เพิ่ม mockext + mailpit
```

---

### Task 1: Skeleton + Store + Console

**Files:**
- Create: `services/mockext/main.go`, `store.go`, `console.go`, `templates/console.html`
- Modify: `deploy/compose.yaml` (เพิ่ม `mockext` ที่ build จาก `deploy/Dockerfile.go` ด้วย `args: {SVC: mockext}` port 8090 พร้อม healthcheck `/healthz` และ `mailpit` port 8025 / SMTP 1025)
- Test: `services/mockext/store_test.go`, `console_test.go`

**Interfaces:**
- Produces:
  - `type Entry struct { At time.Time; Kind string; Title string; Body string; Meta map[string]string }`
  - `type Store struct{...}` มี `Add(kind string, e Entry)` และ `List(kind string) []Entry` (ใหม่สุดก่อน เก็บสูงสุด 200 รายการต่อ kind) และ thread-safe ด้วย `sync.Mutex`
  - kinds: `sms`, `payment`, `webhook`, `oauth`, `push`, `legacy`
  - `GET /` แสดงแท็บตาม kind และลิงก์ไป Mailpit `http://localhost:8025`
- ponytail: เก็บใน memory เท่านั้น restart แล้วหาย ถ้า demo ต้องการเก็บข้อมูลข้ามรอบ ค่อยย้ายไป SQLite

- [ ] **Step 1: เขียน test ที่ fail**: `Add` 201 ครั้งแล้ว `List` ได้ 200 รายการ ตัวแรกคือรายการล่าสุด, `Add` พร้อมกัน 50 goroutine ด้วย `-race` ต้องไม่ race, `GET /` ได้ 200 + `Content-Type: text/html; charset=utf-8` และหน้ามีข้อความ "รหัส OTP 123456" ที่ใส่ไว้
- [ ] **Step 2: รันให้ fail** — `make test PKG=./services/mockext/...`
- [ ] **Step 3: เขียน Store, Console และ main** ใช้ `server.New` จาก chassis (ส่ง DB เป็น nil ได้ เพราะ `/readyz` ของ mockext ไม่ต้องตรวจ DB) ให้ `server.New` ยอมรับ `db == nil`
- [ ] **Step 4: รันให้ผ่าน** และ `make build SVC=mockext && make up` แล้วเปิด `http://localhost:8090` ได้
- [ ] **Step 5: Commit** — `git commit -m "feat(mockext): skeleton, store, console"`

### Task 2: Turnstile + SMS + Mailpit

**Files:**
- Create: `services/mockext/turnstile.go`, `sms.go`
- Test: `services/mockext/turnstile_test.go`, `sms_test.go`

**Interfaces:**
- Produces:
  - `POST /turnstile/v0/siteverify` (form `secret`, `response`, `remoteip`) คืน `{"success":true}` ทุก token ยกเว้น `response == "mock-fail"` ซึ่งคืน `{"success":false,"error-codes":["invalid-input-response"]}` และ `response == "mock-timeout"` ซึ่งหน่วง 5 วิ (รูปแบบตอบเหมือน Cloudflare จริง)
  - `POST /sms/send` JSON `{to, text}` ได้ `202 {"messageId"}` · `to` ที่ขึ้นต้นด้วย `+999` ได้ `400 {"error":"undeliverable"}` ใช้ทดสอบ fallback ไป Email OTP
  - ฝั่งเว็บ / แอปใช้ widget จำลอง: เมื่อ `NEXT_PUBLIC_TURNSTILE_MODE=mock` ให้ส่ง token `mock-ok` (มีปุ่มในหน้า dev ให้เลือก `mock-fail`)
- Email: service ของเราส่ง SMTP ไป `mailpit:1025` แล้วดูผลที่ `http://localhost:8025` ไม่ต้องเขียนโค้ด mock

- [ ] **Step 1: เขียน test ที่ fail**: ครบ 3 กรณีของ siteverify, SMS ภาษาจีน `您的验证码 123456` เก็บใน Store แล้วอ่านกลับได้ตรง, `+999…` ได้ 400
- [ ] **Step 2: รันให้ fail**
- [ ] **Step 3: เขียน handler**
- [ ] **Step 4: รันให้ผ่าน**
- [ ] **Step 5: Commit** — `git commit -m "feat(mockext): turnstile and sms mocks"`

### Task 3: Social login IdP (LINE / Google / Apple / WeChat)

**Files:**
- Create: `services/mockext/oauth.go`, `templates/authorize.html`
- Test: `services/mockext/oauth_test.go`

**Interfaces:**
- Produces สำหรับ `{p}` ∈ `line`, `google`, `apple` (OIDC):
  - `GET /oauth/{p}/.well-known/openid-configuration`
  - `GET /oauth/{p}/authorize?client_id&redirect_uri&state&code_challenge&code_challenge_method=S256&nonce` แสดงหน้าเลือกผู้ใช้ทดสอบ (มีให้เลือก 3 คน + ช่องกรอกอีเมลเอง) แล้ว redirect `redirect_uri?code&state`
  - `POST /oauth/{p}/token` (form `code`, `code_verifier`, `redirect_uri`) ตรวจ PKCE S256 แล้วคืน `{access_token, id_token, token_type}` โดย id_token เป็น EdDSA มี `iss`, `aud`=client_id, `sub`, `email`, `nonce`, `exp` (5 นาที)
  - `GET /oauth/{p}/jwks`
- WeChat (OAuth แบบ WeChat ไม่ใช่ OIDC):
  - `GET /oauth/wechat/connect/qrconnect?appid&redirect_uri&state` (หน้าเว็บ "สแกน QR" จำลองด้วยปุ่มเลือกผู้ใช้) และ `/oauth/wechat/connect/oauth2/authorize` (Official Account) ทั้งสองแบบ redirect กลับพร้อม `code`
  - `GET /oauth/wechat/sns/oauth2/access_token?appid&secret&code&grant_type=authorization_code` คืน `{access_token, openid, unionid, expires_in}` และ**ไม่มีอีเมลหรือเบอร์**
  - ผู้ใช้คนเดียวกันได้ `openid` ต่างกันตาม `appid` แต่ `unionid` เหมือนกัน (เพื่อทดสอบการผูกบัญชีด้วย UnionID)
- code ใช้ได้ครั้งเดียว อายุ 60 วินาที

- [ ] **Step 1: เขียน test ที่ fail**: ทำ flow เต็มด้วย `httptest` → id_token ตรวจด้วย JWKS ผ่าน, `code_verifier` ผิดได้ `400 invalid_grant`, ใช้ code ซ้ำได้ `400 invalid_grant`, WeChat 2 appid ได้ openid ต่างกัน unionid เดียวกัน และ response ไม่มี key `email`
- [ ] **Step 2: รันให้ fail**
- [ ] **Step 3: เขียน handler**
- [ ] **Step 4: รันให้ผ่าน**
- [ ] **Step 5: Commit** — `git commit -m "feat(mockext): oidc and wechat oauth mocks"`

### Task 4: KBank Payment Gateway

**Files:**
- Create: `services/mockext/kbank.go`, `templates/checkout.html`
- Test: `services/mockext/kbank_test.go`

**Interfaces (contract ที่ Commerce ใช้ เมื่อได้สเปก KBank จริงให้เขียน adapter แปลงเข้า contract นี้):**
- `POST /pg/payments` header `Authorization: Bearer <KBANK_API_KEY>` body `{ref, amount, currency:"THB", method: card|promptpay|alipayplus|wechatpay|unionpay, returnUrl, webhookUrl}` คืน `201 {paymentId, status:"pending", paymentUrl, promptpayPayload?}` (amount เป็นสตางค์ แบบ int)
- `GET /pg/checkout/{paymentId}` หน้าจำลองการจ่าย มีปุ่ม: **จ่ายสำเร็จ** · **ล้มเหลว** · **ยกเลิก** · **จ่ายสำเร็จแต่ไม่ส่ง webhook** · **ยอดไม่ตรง (จ่ายขาด 1 บาท)** กดแล้วเปลี่ยนสถานะ ส่ง webhook (ยกเว้นกรณีไม่ส่ง) และ redirect ไป `returnUrl?ref=…`
- Webhook: `POST {webhookUrl}` body `{eventId, paymentId, ref, status: paid|failed|cancelled, amount, paidAt}` header `X-Mock-Signature: hex(HMAC-SHA256(KBANK_WEBHOOK_SECRET, rawBody))` ถ้าได้ non-2xx ให้ retry 3 ครั้งห่างกัน 2, 4, 8 วิ
- `POST /pg/payments/{paymentId}/webhook:resend` (ปุ่มใน Console) ส่ง webhook เดิมซ้ำด้วย `eventId` เดิม
- `GET /pg/payments?ref=…` (inquiry สำหรับกระทบยอด S4) คืน `{paymentId, ref, status, amount}` ถ้าไม่พบได้ `404 {"error":"not_found"}`
- `POST /pg/refunds` `{paymentId, amount}` ได้ `200 {refundId, status:"succeeded"}` ถ้า amount เกินยอดที่จ่ายได้ `422 {"error":"amount_exceeds_paid"}` และกรณี `paymentId` ที่ลงท้ายด้วย `-slow` ตอบ `202 {status:"pending"}` (เพื่อทดสอบ refund แบบ async, Q4)

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestPaySuccessSendsSignedWebhook(t *testing.T)   // httptest receiver ตรวจ HMAC ผ่าน, status paid, redirect ไป returnUrl
func TestWebhookRetryOnFailure(t *testing.T)          // receiver ตอบ 500 สองครั้งแล้ว 200 → ได้รับ 3 ครั้ง eventId เดียวกัน (ใช้ clock/sleep ที่ inject ได้)
func TestResendKeepsEventID(t *testing.T)
func TestPaidWithoutWebhookInquiryShowsPaid(t *testing.T)
func TestAmountMismatch(t *testing.T)                 // inquiry amount = เดิม − 100 สตางค์
func TestInquiryUnknownRef404(t *testing.T)
func TestRefundExceedsPaid422(t *testing.T)
```

- [ ] **Step 2: รันให้ fail**
- [ ] **Step 3: เขียน handler + หน้า checkout** (หน้า checkout แสดงยอดเป็นบาท และแสดง PromptPay QR จาก `promptpayPayload` ด้วยข้อความ ไม่ต้องสร้าง QR จริง)
- [ ] **Step 4: รันให้ผ่าน** `-race`
- [ ] **Step 5: Commit** — `git commit -m "feat(mockext): kbank payment gateway mock"`

### Task 5: Push (FCM / Huawei Push Kit) + ระบบประตูเดิม

**Files:**
- Create: `services/mockext/push.go`, `legacy.go`
- Test: `services/mockext/push_test.go`, `legacy_test.go`

**Interfaces:**
- `POST /push/fcm` และ `POST /push/hms` body `{token, title, body, data, lang}` ได้ `200 {"messageId"}` ถ้า token ขึ้นต้นด้วย `invalid-` ได้ `404 {"error":"UNREGISTERED"}` ทุกข้อความแสดงใน Console แท็บ Push แยก platform
- **Web Push ไม่ mock** เพราะเป็นมาตรฐานเปิด (VAPID) ไม่ต้องมีบัญชีกับใคร Notification ส่งไป push service ของเบราว์เซอร์ได้โดยตรง
- `POST /legacy/gate-events` `{ticketId, gateId, ts, result}` และ `POST /legacy/tickets` `{ticketId, status}` ได้ `202` และแสดงใน Console แท็บ Legacy (ใช้แทน Integration Spec Q1 ระหว่าง demo)

- [ ] **Step 1: เขียน test ที่ fail**: FCM ได้ 200 และเก็บใน Store, `invalid-abc` ได้ 404 `UNREGISTERED`, ข้อความภาษาเกาหลีอ่านกลับได้ตรง, legacy event ทั้ง 2 แบบได้ 202
- [ ] **Step 2: รันให้ fail**
- [ ] **Step 3: เขียน handler**
- [ ] **Step 4: รันให้ผ่าน**
- [ ] **Step 5: Commit** — `git commit -m "feat(mockext): push and legacy gate mocks"`

---

## ตาราง env ที่ service ของเราใช้ใน demo

| Service | env | ค่าใน demo |
|---|---|---|
| identity | `TURNSTILE_VERIFY_URL` | `http://mockext:8090/turnstile/v0/siteverify` |
| identity | `OIDC_LINE_ISSUER`, `OIDC_GOOGLE_ISSUER`, `OIDC_APPLE_ISSUER` | `http://localhost:8090/oauth/{p}` |
| identity | `WECHAT_BASE_URL` | `http://localhost:8090/oauth/wechat` |
| notify | `SMS_BASE_URL` | `http://mockext:8090/sms` |
| notify | `SMTP_ADDR` | `mailpit:1025` |
| notify | `FCM_URL`, `HMS_PUSH_URL` | `http://mockext:8090/push/fcm`, `/push/hms` |
| commerce | `KBANK_BASE_URL`, `KBANK_API_KEY`, `KBANK_WEBHOOK_SECRET` | `http://mockext:8090/pg`, `demo-key`, `demo-secret` |
| ticket | `LEGACY_BASE_URL` | `http://mockext:8090/legacy` |
| web / app | `NEXT_PUBLIC_TURNSTILE_MODE` / `--dart-define=TURNSTILE_MODE` | `mock` |
