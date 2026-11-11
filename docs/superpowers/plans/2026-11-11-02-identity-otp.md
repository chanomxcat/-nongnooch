# Identity: OTP + JWT Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ผู้ใช้ขอ OTP ทางเบอร์โทร (ไทยหรือต่างประเทศ รวม +86) หรืออีเมลได้ ระบบกันบอทด้วย Turnstile และ rate limit เมื่อยืนยัน OTP แล้วได้ JWT ที่ API Gateway ตรวจได้ ฝั่ง Notification ส่ง SMS / อีเมลตามภาษาของผู้ใช้ (1 ใน 7 ภาษา)

**Architecture:** service `identity` (schema `identity`) สร้างบน chassis จาก Plan 01 ส่วนการส่ง SMS / อีเมลอยู่ใน service `notify` ซึ่งรับ event `otp.requested` ผ่าน consumer ของ chassis JWT ลงลายเซ็นแบบ EdDSA (Ed25519) และเปิด public key ผ่าน JWKS ให้ Gateway ตรวจได้โดยไม่ต้องเรียก Identity

**Tech Stack:** Go 1.23 + chassis (Plan 01), `github.com/golang-jwt/jwt/v5`, `github.com/nyaruka/phonenumbers`, `crypto/ed25519`, `crypto/hmac`, `net/smtp`, Cloudflare Turnstile siteverify API

**Spec:** [docs/system-design.md](../../system-design.md) F1, S1 ขั้น 3–14, หัวข้อ 2 (Edge, SMS / Email) · ขึ้นกับ [Plan 01](2026-11-11-01-platform-chassis.md) และ [Plan 00 mockext](2026-11-11-00-mock-external.md) Task 2

## Global Constraints

- **Build และ test ใน Docker เสมอ (มติ 11 พ.ย. 2026):** ห้ามรัน `go build` / `go test` / `go run` บนเครื่องโดยตรง ให้ใช้ `make test`, `make test-int`, `make build` ที่รันใน container `golang:1.23-bookworm` คู่กับ MariaDB / NATS ใน `deploy/compose.yaml` (กำหนดใน Plan 01 Task 1)
- ข้อกำหนดทั้งหมดใน Global Constraints ของ Plan 01
- "SMS OTP ที่ส่งถึง +86 ได้ + Email OTP สำรอง"
- "ใช้ Turnstile แทน reCAPTCHA เพราะ reCAPTCHA ถูกบล็อกในจีน"
- S1: `POST /auth/otp {phone, lang, turnstileToken}` → `202`, Identity ทำ "rate limit · บันทึก OTP (hash) · outbox otp.requested"
- 7 ภาษา: `th en zh ja ko de fr` และภาษาที่ไม่มี template ให้ใช้ EN [สมมติฐาน S7]
- F1: "ส่งใหม่ · จำกัดจำนวนครั้ง"

**ค่าที่แผนนี้กำหนดเพิ่ม เพราะ spec ไม่ได้ระบุ (ให้ PO ยืนยันใน review):** OTP 6 หลัก, อายุ 5 นาที, ตอบผิดได้ 5 ครั้งต่อรหัส, ขอส่งใหม่ได้หลังผ่านไป 60 วินาที, ไม่เกิน 5 ครั้งต่อเบอร์ / อีเมลต่อชั่วโมง, ไม่เกิน 20 ครั้งต่อ IP ต่อชั่วโมง, access token 15 นาที, refresh token 30 วัน (หมุนใหม่ทุกครั้งที่ใช้)

**ต่างจาก spec:** S1 ระบุ `POST /auth/verify {otp}` แต่แผนนี้ใช้ `{requestId, otp}` เพราะ server ต้องรู้ว่าตรวจกับคำขอใด

**โหมด Demo (มติ 11 พ.ย. 2026):** ระบบภายนอกทุกตัวใช้ mock ดังนี้ Turnstile ใช้ `TURNSTILE_VERIFY_URL` ที่ชี้ไป mockext, SMS ใช้ `SMS_BASE_URL` ที่ชี้ไป mockext และอีเมลส่งไป Mailpit ทุกอย่างตั้งผ่าน env ไม่ hardcode เมื่อจะใช้ผู้ให้บริการจริงให้เพิ่ม adapter (Task 5 Step 6)

## Review Focus

1. **เบอร์ที่กรอกหลายรูปแบบ** เช่น `0812345678`, `+66 81 234 5678`, `66812345678` ต้องได้ E.164 ตัวเดียวกัน ถ้าไม่เป็นแบบนี้ rate limit และกติกา 1 เบอร์ = 1 บัญชีจะถูกเลี่ยงได้ (Task 1)
2. **เบอร์จีน +86 และเบอร์ที่ไม่มีรหัสประเทศ:** ค่า default region คือ TH และเบอร์จีนต้องกรอก `+86` เท่านั้น ถ้าเป็นเลขที่ไม่ valid ต้องได้ 400 พร้อม error code ไม่ใช่ 500 (Task 1)
3. **เดา OTP:** ตอบผิด 5 ครั้งแล้วต้องใช้รหัสนั้นไม่ได้อีก แม้จะกรอกรหัสที่ถูกก็ตาม และใช้รหัสที่ถูกซ้ำครั้งที่สองต้องไม่ผ่าน (Task 3)
4. **Turnstile ล่มหรือหมดเวลา:** ต้อง fail closed (ปฏิเสธ) พร้อม error code `turnstile_unavailable` ให้หน้าเว็บแสดงข้อความให้ลองใหม่ ไม่ปล่อยผ่านเงียบ ๆ (Task 2)
5. **OTP ในข้อความ event:** `otp.requested` ต้องมีรหัสแบบ plaintext เพื่อให้ Notification ส่งได้ จึงห้าม log payload นี้ใน consumer และให้ test ยืนยันว่าบรรทัด log ไม่มีรหัส (Task 5)

---

## File Structure

```
services/identity/main.go
services/identity/migrations/00001_identity.sql   otp_requests, refresh_tokens
services/identity/contact.go          Normalize ข้อมูลติดต่อ
services/identity/turnstile.go        Verifier
services/identity/otp.go              ขอ / ตรวจ OTP
services/identity/token.go            ออก / หมุน JWT + JWKS
services/identity/http.go             handler
api/openapi/identity.yaml
services/notify/main.go
services/notify/migrations/00001_notify.sql       (ตารางของ notify ในแผน 10 จะเพิ่มต่อ)
services/notify/otp.go                consumer otp.requested
services/notify/templates/otp.<lang>.txt          7 ไฟล์
```

---

### Task 1: Normalize ข้อมูลติดต่อ + schema

**Files:**
- Create: `services/identity/contact.go`, `services/identity/migrations/00001_identity.sql`
- Test: `services/identity/contact_test.go`

**Interfaces:**
- Produces: `type Contact struct { Kind string /* "phone" | "email" */; Value string /* E.164 หรือ email ตัวพิมพ์เล็ก */ }` และ `func NormalizeContact(phone, email string) (Contact, error)` ส่ง phone หรือ email มาอย่างใดอย่างหนึ่ง (ส่งทั้งคู่หรือไม่ส่งเลยเป็น error) error คืนเป็นค่าคงที่ `ErrInvalidPhone`, `ErrInvalidEmail`, `ErrContactRequired`

ตาราง:

```sql
CREATE TABLE otp_requests (
  id CHAR(36) PRIMARY KEY, contact VARCHAR(254) NOT NULL, kind VARCHAR(8) NOT NULL,
  code_hmac BINARY(32) NOT NULL, lang CHAR(2) NOT NULL, ip VARBINARY(16) NOT NULL,
  attempts TINYINT NOT NULL DEFAULT 0, expires_at DATETIME(6) NOT NULL,
  used_at DATETIME(6) NULL, created_at DATETIME(6) NOT NULL,
  INDEX idx_contact_time (contact, created_at), INDEX idx_ip_time (ip, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE refresh_tokens (
  id CHAR(36) PRIMARY KEY, subject VARCHAR(254) NOT NULL, token_hash BINARY(32) NOT NULL UNIQUE,
  expires_at DATETIME(6) NOT NULL, revoked_at DATETIME(6) NULL, created_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

- [ ] **Step 1: เขียน test ที่ fail** แบบ table-driven

```go
{"0812345678", "", "+66812345678", nil},
{"+66 81 234 5678", "", "+66812345678", nil},
{"66812345678", "", "+66812345678", nil},
{"+86 138 0013 8000", "", "+8613800138000", nil},
{"12345", "", "", ErrInvalidPhone},
{"", "  Foo@Example.COM ", "foo@example.com", nil},
{"", "not-an-email", "", ErrInvalidEmail},
{"0812345678", "a@b.co", "", ErrContactRequired},
{"", "", "", ErrContactRequired},
```

- [ ] **Step 2: รันให้ fail** — `make test PKG=./services/identity/ RUN=TestNormalizeContact`
- [ ] **Step 3: เขียน `NormalizeContact`**: ใช้ `phonenumbers.Parse(s, "TH")` + `IsValidNumber`, ตรวจอีเมลด้วย `net/mail.ParseAddress` แล้ว lower-case
- [ ] **Step 4: รันให้ผ่าน**
- [ ] **Step 5: Commit** — `git commit -m "feat(identity): contact normalization and schema"`

### Task 2: ขอ OTP (`POST /auth/otp`)

**Files:**
- Create: `services/identity/turnstile.go`, `services/identity/otp.go`, `services/identity/http.go`, `api/openapi/identity.yaml`
- Test: `services/identity/otp_request_test.go` (integration)

**Interfaces:**
- Consumes: `NormalizeContact`, `outbox.Write`, `events.New(events.OTPRequested, ...)`
- Produces:
  - `type Verifier interface { Verify(ctx context.Context, token, ip string) error }` และ `TurnstileVerifier{URL, Secret string; Client *http.Client}` (timeout 3 วินาที) โดย `URL` มาจาก env `TURNSTILE_VERIFY_URL` (demo ใช้ mockext, production ใช้ `https://challenges.cloudflare.com/turnstile/v0/siteverify`) ใน unit test ใช้ fake
  - `func (s *OTPService) Request(ctx, phone, email, lang, turnstileToken, ip string) (requestID string, err error)`
  - payload ของ `otp.requested`: `{"requestId","kind","contact","code","lang"}` เมื่อ lang ไม่อยู่ใน 7 ภาษาให้เปลี่ยนเป็น `en`
  - HTTP: `POST /auth/otp` body `{phone?, email?, lang, turnstileToken}` ตอบ `202 {"requestId"}` และ error `400 invalid_phone|invalid_email|contact_required`, `403 turnstile_failed`, `503 turnstile_unavailable`, `429 rate_limited` (header `Retry-After` เป็นวินาที)
- OTP สร้างด้วย `crypto/rand` 6 หลัก แล้วเก็บเฉพาะ `HMAC-SHA256(OTP_PEPPER, requestId + ":" + code)` (`OTP_PEPPER` มาจาก env / Secret manager)

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestRequestWritesRowAndOutboxInOneTx(t *testing.T) // 202, otp_requests 1 แถว, outbox 1 แถว type otp.requested, code_hmac ไม่เท่ากับรหัส plaintext
func TestTurnstileRejected(t *testing.T)                 // fake คืน error → 403 turnstile_failed, ไม่มีแถวใหม่
func TestTurnstileTimeout(t *testing.T)                  // fake หน่วง 5 วิ → 503 turnstile_unavailable
func TestResendCooldown60s(t *testing.T)                 // ขอซ้ำทันที → 429, Retry-After อยู่ระหว่าง 1 ถึง 60
func TestPerContactLimit5PerHour(t *testing.T)           // เลื่อนนาฬิกา (clock inject) ครั้งละ 61 วิ, ครั้งที่ 6 → 429
func TestPerIPLimit20PerHour(t *testing.T)               // 21 เบอร์ต่างกันจาก IP เดียว → ครั้งที่ 21 ได้ 429
func TestUnknownLangFallsBackToEN(t *testing.T)          // lang "ru" → payload.lang == "en"
```

- [ ] **Step 2: รันให้ fail** — `make test-int PKG=./services/identity/...`
- [ ] **Step 3: เขียน `Request`**: ตรวจ Turnstile → normalize → นับ rate limit จาก `otp_requests` (ใช้ index ที่มีอยู่ ไม่ต้องใช้ Redis) → `BEGIN` insert + `outbox.Write` `COMMIT` และ inject `now func() time.Time` เพื่อให้ test เลื่อนนาฬิกาได้
- [ ] **Step 4: เขียน handler + `identity.yaml`** ตาม Interfaces
- [ ] **Step 5: รันให้ผ่าน**
- [ ] **Step 6: Commit** — `git commit -m "feat(identity): request otp with turnstile and rate limits"`

### Task 3: ยืนยัน OTP (`POST /auth/verify`) + ออก JWT

**Files:**
- Create: `services/identity/token.go`
- Modify: `services/identity/otp.go`, `services/identity/http.go`, `api/openapi/identity.yaml`
- Test: `services/identity/otp_verify_test.go`, `services/identity/token_test.go`

**Interfaces:**
- Produces:
  - `func (s *OTPService) Verify(ctx, requestID, code string) (Contact, error)` error มี `ErrOTPInvalid`, `ErrOTPExpired`, `ErrOTPLocked`, `ErrOTPUsed`
  - `type Tokens struct { Access, Refresh string; ExpiresIn int }`
  - `func (t *TokenIssuer) Issue(ctx, c Contact) (Tokens, error)` claims ตาม [contracts](2026-11-11-contracts.md) หัวข้อ 3: `typ` = `user`, `sub` = E.164 หรือ email, `ctype` = `phone|email`, `mid` = "" (เติมในแผน 03), `lang`, `verified` = true, `iss` = `nongnooch-identity`, `aud` = `nongnooch-api`, `exp` = 15 นาที, header `kid`
  - HTTP: `POST /auth/verify {requestId, otp}` ได้ `200 Tokens` และ error `400 otp_invalid` (พร้อม `attemptsLeft`), `410 otp_expired`, `423 otp_locked`, `409 otp_used`
- ส่วน `memberId` จะเพิ่มลง claim ในแผน 03 หลังสร้าง Member แล้ว (S1 ขั้น 14–15)

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestVerifyCorrectCode(t *testing.T)         // 200, access token parse ได้ด้วย public key, sub == "+66812345678"
func TestVerifyWrongCodeDecrementsAttempts(t *testing.T) // attemptsLeft 4,3,2,1 แล้วครั้งที่ 5 → 423
func TestLockedEvenWithRightCode(t *testing.T)   // ผิด 5 ครั้งแล้วส่งรหัสถูก → 423
func TestExpiredAfter5Min(t *testing.T)          // เลื่อนนาฬิกา 5 นาที 1 วิ → 410
func TestCodeSingleUse(t *testing.T)             // ครั้งที่สอง → 409
func TestConcurrentVerifyOnlyOneWins(t *testing.T) // 10 goroutine ส่งรหัสถูกพร้อมกัน → 200 หนึ่งครั้ง ที่เหลือ 409 (SELECT … FOR UPDATE)
```

- [ ] **Step 2: รันให้ fail**
- [ ] **Step 3: เขียน `Verify`** (`SELECT … FOR UPDATE` → ตรวจ used / expired / attempts → เทียบด้วย `hmac.Equal`) และ `Issue` (EdDSA, ใช้ private key จาก env `JWT_ED25519_KEY` แบบ PEM)
- [ ] **Step 4: รันให้ผ่าน** `-race`
- [ ] **Step 5: Commit** — `git commit -m "feat(identity): verify otp and issue eddsa jwt"`

### Task 4: Refresh, logout และ JWKS

**Files:**
- Modify: `services/identity/token.go`, `services/identity/http.go`, `api/openapi/identity.yaml`
- Test: `services/identity/refresh_test.go`

**Interfaces:**
- Produces:
  - `POST /auth/refresh {refreshToken}` ได้ `200 Tokens` (หมุนใหม่: token เดิม revoke ทันที) ส่วน token ที่ถูก revoke แล้วถูกใช้ซ้ำให้ revoke token ทุกตัวของ subject นั้น แล้วตอบ `401 refresh_reused`
  - `POST /auth/logout {refreshToken}` ได้ `204`
  - `GET /.well-known/jwks.json` คืน OKP / Ed25519 key พร้อม `kid` ซึ่ง Gateway (Plan 12) ใช้ตรวจ token
- refresh token เป็น random 32 byte (base64url) และเก็บเฉพาะ SHA-256

- [ ] **Step 1: เขียน test ที่ fail**: หมุน token ได้ token ใหม่, ใช้ token เก่าซ้ำได้ `401 refresh_reused` และ token ใหม่ใช้ไม่ได้อีก, หมดอายุ 30 วันได้ `401`, logout แล้ว refresh ได้ `401`, JWKS มี `kid` ตรงกับ header ของ access token
- [ ] **Step 2: รันให้ fail**
- [ ] **Step 3: เขียนตาม Interfaces**
- [ ] **Step 4: รันให้ผ่าน**
- [ ] **Step 5: Commit** — `git commit -m "feat(identity): refresh rotation, logout, jwks"`

### Task 5: Notification ส่ง OTP (SMS / Email 7 ภาษา)

**Files:**
- Create: `services/notify/main.go`, `services/notify/migrations/00001_notify.sql` (ไฟล์ว่างสำหรับ goose), `services/notify/otp.go`, `services/notify/templates/otp.{th,en,zh,ja,ko,de,fr}.txt`
- Test: `services/notify/otp_test.go` (integration)

**Interfaces:**
- Consumes: `consumer.Run` (Config.Name = `notify-otp`, EventTypes = `[events.OTPRequested]`), payload จาก Task 2
- Produces:
  - `type SMSSender interface { Send(ctx, e164, text string) error }` + `HTTPSMSSender{BaseURL string; Client *http.Client}` ที่ POST `{to, text}` ไป `{SMS_BASE_URL}/send` (contract ของ mockext) โดย log เฉพาะเบอร์ที่ปิดบังเหลือ 4 ตัวท้าย ห้าม log ข้อความ
  - `type EmailSender interface { Send(ctx, to, subject, body string) error }` + `SMTPSender{Addr, From, Auth}` (`net/smtp`)
  - `func OTPHandler(sms SMSSender, mail EmailSender, tpl Templates) consumer.Handler`
- ข้อความ template ใช้ `text/template` ตัวแปร `{{.Code}}` และ `{{.Minutes}}` (= 5) ทุกภาษาต้องมีคำว่า "สวนนงนุช" หรือชื่อแบรนด์ในภาษานั้น (ให้การตลาดตรวจคำแปลเป็นงาน NN 1.18.3)

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestPhoneOTPSentOnceInLang(t *testing.T)     // publish otp.requested lang=zh 2 ครั้ง (id เดียวกัน) → fake SMS ถูกเรียก 1 ครั้ง ข้อความมาจาก otp.zh.txt
func TestEmailOTP(t *testing.T)                   // kind=email → fake Email ถูกเรียก
func TestAllSevenTemplatesRender(t *testing.T)    // render ครบ 7 ภาษา แต่ละอันมีรหัส 6 หลัก
func TestSenderErrorRetries(t *testing.T)         // fake fail ครั้งแรก → ส่งสำเร็จในครั้งที่ redeliver
func TestNoCodeInLogs(t *testing.T)               // จับ slog output ทั้งหมด → ไม่มีรหัสปรากฏ
```

- [ ] **Step 2: รันให้ fail** — `make test-int PKG=./services/notify/...`
- [ ] **Step 3: เขียน `OTPHandler`, `HTTPSMSSender`, `SMTPSender`, templates 7 ไฟล์**
- [ ] **Step 4: รันให้ผ่าน**
- [ ] **Step 5: ทดสอบปลายทางบน local**: `POST /auth/otp` ด้วย `turnstileToken: "mock-ok"` → เห็น SMS ใน Mock Console `http://localhost:8090` (เคสอีเมลดูที่ Mailpit `http://localhost:8025`) → `POST /auth/verify` ด้วยรหัสนั้นได้ 200 · ส่ง `mock-fail` ต้องได้ 403
- [ ] **Step 6: (หลัง demo ไม่อยู่ในขอบเขตตอนนี้)** เมื่อเลือกผู้ให้บริการ SMS จริงแล้ว ให้เขียน `<Provider>Sender` ที่ implement `SMSSender` และทดสอบส่งจริงไปเบอร์ +66 และ +86
- [ ] **Step 7: Commit** — `git commit -m "feat(notify): send otp via sms/email in 7 languages"`
