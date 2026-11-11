# Notification: Device, Template 7 ภาษา, Push routing, Campaign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** service `notify` รับลงทะเบียนเครื่อง (ios / gms / hms / web) และ Web Push subscription, แปลง domain event เป็นข้อความ 7 ภาษา (ไม่มีภาษานั้นใช้ EN), ส่ง Push ตาม platform ไป FCM / Huawei Push Kit / Web Push พร้อมส่งซ้ำทางอีเมล / SMS สำหรับเรื่องสำคัญ, ลบ token เสีย และให้ทีมการตลาดตั้งเวลา Push campaign เลือกกลุ่มได้ โดยส่งเฉพาะผู้ยินยอมรับข่าวสาร

**Architecture:** ต่อยอด `services/notify` จาก Plan 02 Task 5 (consumer `otp.requested`, `SMSSender`, `EmailSender`, template OTP 7 ภาษา ซึ่งยังทำงานแบบเดิม) consumer ของ event อื่นและ scheduler ของ campaign **ไม่ส่งออกภายนอกเอง** แต่เขียนแถว `deliveries` สถานะ `pending` ใน transaction เดียวกับ `processed_events` จากนั้น `Dispatcher` (worker ใน process เดียวกัน) ดึงแถวด้วย `FOR UPDATE SKIP LOCKED` แล้วส่งตามช่องทาง วิธีนี้ทำให้ redeliver ของ JetStream ไม่ทำให้ส่งซ้ำ และ retry ของผู้ให้บริการแยกจาก retry ของ event ข้อมูลภาษา / อีเมล / เบอร์ / consent ของสมาชิกได้จาก gRPC `MemberService.GetContactPrefs`

**Tech Stack:** Go 1.23 + chassis (Plan 01), `pkg/chassis/authn` (Plan 12 Task 1), `text/template`, `github.com/SherClockHolmes/webpush-go` (VAPID + aes128gcm), `google.golang.org/grpc` client ของ `gen/go/member/v1`, mockext `/push` (Plan 00 Task 5), Mailpit, mockext `/sms`

**Spec:** [docs/system-design.md](../../system-design.md) S7, 1.2 (แถว Notification), 1.3 Event Map (O3), 2 (แถว Push), Q8, Q22, Q25 · [ROADMAP.md](../../../roadmap/ROADMAP.md) 4.2 E5 ("ได้รับแจ้งเตือนแม้ไม่ได้เปิดแอป", "รับข่าวสาร"), E8 (Push notification), E9 (PDPA) · [contracts](2026-11-11-contracts.md) §1, §3, §4, §5, §7 · ขึ้นกับ [Plan 01](2026-11-11-01-platform-chassis.md), [Plan 02 Task 5](2026-11-11-02-identity-otp.md), [Plan 00 Task 5](2026-11-11-00-mock-external.md)

## Global Constraints

- **Build และ test ใน Docker เสมอ (มติ 11 พ.ย. 2026):** ห้ามรัน `go` / `npm` / `flutter` บนเครื่องโดยตรง ใช้ `make test`, `make test-int`, `make build SVC=notify`, `make gen` เท่านั้น integration test ขึ้นต้นด้วย `//go:build integration`
- ข้อกำหนดทั้งหมดใน Global Constraints ของ Plan 01 (schema `notify` ของตัวเอง, outbox, consumer idempotent, OpenTelemetry)
- **ใช้ [contracts](2026-11-11-contracts.md) ตามตัวอักษร ห้ามแก้** ชื่อ event, payload, gRPC `GetContactPrefs(GetContactPrefsRequest{member_id}) returns (ContactPrefs{lang, marketing_consent, phone, email})` ถ้าต้องเปลี่ยนให้เขียนใน "ข้อเสนอแก้ contracts" ท้ายแผน
- **โหมด Demo:** FCM และ Huawei Push Kit ส่งไป mockext ผ่าน env `FCM_URL` / `HMS_PUSH_URL` (`http://mockext:8090/push/fcm`, `/push/hms`), อีเมลส่ง SMTP ไป Mailpit (`SMTP_ADDR=mailpit:1025`), SMS ไป `SMS_BASE_URL` · **Web Push (VAPID) เป็นของจริง** ใช้ไลบรารี `webpush-go` และกุญแจจาก env `VAPID_PUBLIC_KEY`, `VAPID_PRIVATE_KEY`, `VAPID_SUBJECT` (`mailto:dev@nongnooch.example`)
- S7: "ลงทะเบียนเครื่อง {token, platform: ios | gms | hms | web, lang}" · "ios / gms → FCM, hms → Huawei Push Kit, web → Web Push" · contracts หัวข้อ 10: `POST /api/notify/devices {token, platform, lang, app: "tourist"|"agent"|"web"}` และ push ของไกด์ส่งเฉพาะเครื่องที่ `app=agent`
- Contracts หัวข้อ 10: Push ทุกข้อความมี `data: {type, link}` (`type` = template key, `link` = path ของเว็บ / แอปตามตาราง Deep link ข้างล่าง) · หัวข้อ 5: `order.refunded` มี `memberId?, guestContact?, lang` แล้ว, `referral.created.guideMemberId`, `agent.approved.memberId?`, consumer `account.deleted`
- S7: "ข้อความการตลาด และผู้ใช้ไม่ยินยอม → ข้าม" · "เลือก template ตามภาษา ไม่มีใช้ EN [สมมติฐาน]" · "เรื่องสำคัญ (e-Ticket, การยืนยัน) → ส่งซ้ำอีกช่องทาง" · "ลบ token ที่ใช้ไม่ได้แล้ว"
- 7 ภาษา: `th en zh ja ko de fr` (ZH ตัวย่อ) ไม่มีข้อความในภาษานั้นใช้ `en`
- E5: "แตะแล้วเปิดหน้าที่เกี่ยวข้องในแอป (Deep link)" · "โปรโมชัน (เฉพาะผู้ให้ความยินยอม), ตั้งเวลาส่งและเลือกกลุ่มผู้รับจาก CMS ได้"
- E10 / S1: แจ้งไกด์ "มีผู้สมัครด้วยโค้ดของคุณ (จำนวนเท่านั้น)" ห้ามมีชื่อ เบอร์ หรืออีเมลของนักท่องเที่ยวในข้อความ
- Q8: ช่องทางซ้ำของเรื่องสำคัญใช้อีเมล (ไม่มีอีเมลใช้ SMS) **ไม่ทำ LINE** จนกว่า Q8 มีคำตอบ
- Contracts §1: ห้ามส่งหรือ log เลขบัตรเต็ม · เบอร์ / อีเมลใน log ต้องปิดบังเหลือ 4 ตัวท้าย
- Contracts §7: `POST /internal/seed` ทำงานเมื่อ `DEMO_MODE=true` เท่านั้น และรันซ้ำได้

**ค่าที่แผนนี้กำหนดเพิ่ม (ให้ PO ยืนยันใน review):** Dispatcher retry 5 ครั้ง ห่าง 30 วิ, 2 นาที, 10 นาที, 30 นาที, 2 ชม. · scheduler / dispatcher tick ทุก 5 วิ · campaign ต้องมีข้อความ `en` เสมอ · token ที่ได้ `UNREGISTERED` / HTTP 404 / 410 ถูก disable ทันที (ไม่ลบแถว เพื่อเก็บประวัติ) · หลังส่งสำเร็จลบอีเมล / เบอร์เต็มออกจาก `deliveries` เหลือแบบปิดบัง

## Review Focus

1. **JetStream ส่ง event ซ้ำ หรือ handler fail หลังเขียน delivery บางส่วน:** ผู้ใช้ต้องได้ Push / อีเมลแค่ครั้งเดียว ใช้ unique key `(source_id, template_key, channel, target)` บน `deliveries` และไม่ส่งภายนอกใน consumer (Task 3: `TestRedeliveredTicketIssuedCreatesNoDuplicateDeliveries`)
2. **token เดียวกันลงทะเบียนซ้ำ / เปลี่ยนเจ้าของ (ล็อกอินบัญชีอื่นบนเครื่องเดิม):** ต้องมีแถวเดียวและย้ายไปผูกกับผู้ใช้ล่าสุด ไม่อย่างนั้นบัตรของคนหนึ่งจะเด้งบนเครื่องอีกคน (Task 1: `TestReRegisterSameTokenMovesOwner`)
3. **สมาชิกถอนความยินยอมระหว่างตั้งเวลา campaign:** ตรวจ consent ตอนส่งจริง ไม่ใช่ตอนสร้าง (Task 5: `TestConsentCheckedAtSendTime`)
4. **ภาษาที่ไม่รองรับหรือว่าง (`ru`, `""`, `zh-TW`):** ต้องได้ข้อความ EN ไม่ใช่ error หรือข้อความว่าง (Task 2: `TestUnknownLangFallsBackToEN`)
5. **ผู้ให้บริการ Push ล่ม (mockext ดับ / 5xx):** ต้อง retry ตาม backoff แล้วจบที่ `failed` โดยไม่ disable token และไม่บล็อก delivery ของคนอื่น (Task 4: `TestProviderDownRetriesThenFails`)

---

## File Structure

```
services/notify/main.go                 (แก้) ต่อ consumer ใหม่, Dispatcher, Scheduler, HTTP
services/notify/migrations/00002_notify_core.sql   devices, deliveries, campaigns
services/notify/devices.go              Device registry + HTTP /devices, /webpush/vapid-public-key
services/notify/catalog.go              Template catalog (Render + EN fallback) ใช้กับทุก key รวม otp
services/notify/templates/<key>.<lang>.tmpl   6 key × 7 ภาษา
services/notify/prefs.go                PrefsClient (gRPC GetContactPrefs) + Recipient
services/notify/events.go               consumer notify-events: event → pending deliveries + consumer account.deleted
services/notify/links.go                deep link
services/notify/dispatch.go             Dispatcher + PushSender ×3 + Email/SMS
services/notify/webpush.go              WebPushSender (webpush-go)
services/notify/campaign.go             Campaign API + Scheduler
services/notify/seed.go                 POST /internal/seed
api/openapi/notify.yaml
```

## Deep link ที่ Plan 13 (เว็บ) / Plan 15 (แอป) ต้องรองรับ (contracts หัวข้อ 10)

ทุก Push มี `data: {type, link}` โดย `type` = template key และ `link` = path ตามตาราง (ไม่มี prefix ภาษา แอปเปิดด้วย router ของตัวเอง ส่วนเว็บใช้ `Accept-Language` เติม prefix) Web Push ส่ง `link` ใน payload และ Service Worker (Plan 13) เปิด `{origin}/{lang}{link}` เมื่อแตะ อีเมล / SMS ใช้ URL เต็ม `{WEB_BASE_URL}/{lang}{link}`

| template key | `link` |
|---|---|
| `ticket_issued` | `/tickets` |
| `ticket_received` | `/tickets/{newTicketId}` |
| `ticket_sent` | `/tickets` |
| `order_refunded` | `/tickets` |
| `referral_created`, `agent_decision` | `/agent` |
| campaign | `url` ที่ตั้งใน campaign (ต้องขึ้นต้นด้วย `/` เช่น `/shows`, `/points`) |

---

### Task 1: Schema + Device registry + Web Push subscription API

**Files:**
- Create: `services/notify/migrations/00002_notify_core.sql`, `services/notify/devices.go`, `api/openapi/notify.yaml`
- Modify: `services/notify/main.go`
- Test: `services/notify/devices_test.go` (integration)

**Interfaces:**
- Consumes: `authn.From(ctx)` (Plan 12 Task 1) ใช้ `MemberID` (จาก `X-Member-Id`, ว่างได้) และ `Sub` (E.164 / อีเมล), `authn.Require("user")`
- Produces:
  - `type Device struct { ID, MemberID, Subject, Platform, App, Lang string; Token string /* FCM/HMS token หรือ web endpoint */; P256dh, Auth string; DisabledAt *time.Time }`
  - `type DeviceStore struct{ DB *sql.DB }` มี `Upsert(ctx, d Device) (id string, err error)`, `ByMember(ctx, memberID string) ([]Device, error)`, `BySubject(ctx, subject string) ([]Device, error)` (ทั้งสองคืนเฉพาะ `disabled_at IS NULL`), `Disable(ctx, id string) error`, `Delete(ctx, id, ownerSub string) error`
  - HTTP:
    - `POST /devices` body `{token, platform: "ios"|"gms"|"hms"|"web", lang, app: "tourist"|"agent"|"web"}` (contracts หัวข้อ 10) → `201 {"deviceId"}` · `400 invalid_platform|invalid_app|token_required` · `platform=web` ใช้ `token` = `JSON.stringify(PushSubscription)` (`{endpoint, keys: {p256dh, auth}}`) server แยกเก็บ endpoint / p256dh / auth ไม่ครบได้ `400 invalid_subscription` (endpoint ต้องเป็น `https://` ใน production, ยอม `http://` เมื่อ `DEMO_MODE=true` เพื่อ test)
    - `DELETE /devices/{deviceId}` → `204` (ลบได้เฉพาะเครื่องที่ `subject` ตรงกับผู้เรียก ไม่ใช่ได้ `404 not_found`)
    - `GET /webpush/vapid-public-key` → `200 {"publicKey"}` ไม่ต้องล็อกอิน (gateway ต้องเปิดเป็น public route, Plan 12)
  - lang ที่ไม่อยู่ใน 7 ภาษาเก็บเป็น `en`

ตาราง (ทั้งไฟล์ migration นี้ใช้ต่อใน Task 3–5):

```sql
CREATE TABLE devices (
  id CHAR(36) PRIMARY KEY, member_id CHAR(36) NULL, subject VARCHAR(254) NOT NULL,
  platform ENUM('ios','gms','hms','web') NOT NULL, app ENUM('tourist','agent','web') NOT NULL,
  lang CHAR(2) NOT NULL, token TEXT NOT NULL, token_hash BINARY(32) NOT NULL UNIQUE,
  p256dh VARCHAR(128) NULL, auth VARCHAR(64) NULL,
  created_at DATETIME(6) NOT NULL, last_seen_at DATETIME(6) NOT NULL, disabled_at DATETIME(6) NULL,
  INDEX idx_member (member_id), INDEX idx_subject (subject)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE deliveries (
  id CHAR(36) PRIMARY KEY, source_id CHAR(36) NOT NULL,        -- event id หรือ campaign id
  template_key VARCHAR(48) NOT NULL, channel ENUM('fcm','hms','webpush','email','sms') NOT NULL,
  target VARCHAR(64) NOT NULL,                                 -- device id หรือ sha256 hex ของ address
  device_id CHAR(36) NULL, address VARCHAR(254) NULL, address_masked VARCHAR(64) NOT NULL,
  lang CHAR(2) NOT NULL, title VARCHAR(255) NOT NULL, body TEXT NOT NULL, data JSON NOT NULL,
  status ENUM('pending','sent','failed','invalid_token') NOT NULL, attempts TINYINT NOT NULL DEFAULT 0,
  next_attempt_at DATETIME(6) NOT NULL, last_error VARCHAR(255) NULL,
  created_at DATETIME(6) NOT NULL, sent_at DATETIME(6) NULL,
  UNIQUE KEY uq_once (source_id, template_key, channel, target),
  INDEX idx_due (status, next_attempt_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE campaigns (
  id CHAR(36) PRIMARY KEY, name VARCHAR(200) NOT NULL, kind ENUM('marketing','service') NOT NULL,
  messages JSON NOT NULL, url VARCHAR(255) NOT NULL, segment JSON NOT NULL,
  status ENUM('draft','scheduled','sending','sent','cancelled') NOT NULL,
  scheduled_at DATETIME(6) NULL, created_by VARCHAR(64) NOT NULL,
  skipped_consent INT NOT NULL DEFAULT 0, created_at DATETIME(6) NOT NULL, updated_at DATETIME(6) NOT NULL,
  INDEX idx_due (status, scheduled_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestRegisterFCMDevice(t *testing.T)              // POST /devices platform=gms → 201, แถว member_id = X-Member-Id, subject = X-User-Sub
func TestRegisterWebSubscription(t *testing.T)        // POST /devices platform=web app=web token=JSON subscription → 201, p256dh/auth ถูกเก็บ · token ไม่ใช่ JSON subscription → 400 invalid_subscription
func TestInvalidApp400(t *testing.T)                  // app "kiosk" → 400 invalid_app
func TestReRegisterSameTokenMovesOwner(t *testing.T)  // token เดิมจากผู้ใช้ B → ยังมี 1 แถว, subject = B, disabled_at ถูกล้างเป็น NULL
func TestInvalidPlatform400(t *testing.T)             // platform "android" → 400 invalid_platform
func TestUnknownLangStoredAsEN(t *testing.T)          // lang "ru" → lang = "en"
func TestDeleteOnlyOwnDevice(t *testing.T)            // ผู้ใช้อื่นลบ → 404, เจ้าของลบ → 204
func TestVapidPublicKeyNoAuth(t *testing.T)           // ไม่มี header ตัวตน → 200 publicKey == env VAPID_PUBLIC_KEY
func TestByMemberSkipsDisabled(t *testing.T)
```

- [ ] **Step 2: รันให้ fail** — `make test-int PKG=./services/notify/... RUN=Device`
- [ ] **Step 3: เขียน migration, `DeviceStore`, handler** ใช้ `INSERT … ON DUPLICATE KEY UPDATE` บน `token_hash = sha256(token)` (อัปเดต member_id, subject, lang, app, last_seen_at, disabled_at = NULL)
- [ ] **Step 4: เขียน `notify.yaml`** ครบ 3 endpoint แล้วรัน `make gen`
- [ ] **Step 5: รันให้ผ่าน**
- [ ] **Step 6: Commit** — `git commit -m "feat(notify): device registry and web push subscription api"`

### Task 2: Template catalog 7 ภาษา + Recipient จาก GetContactPrefs

**Files:**
- Create: `services/notify/catalog.go`, `services/notify/prefs.go`, `services/notify/links.go`, `services/notify/templates/{ticket_issued,ticket_received,ticket_sent,order_refunded,referral_created,agent_decision,payment_discrepancy}.{th,en,zh,ja,ko,de,fr}.tmpl`
- Modify: `services/notify/otp.go` (ให้ `OTPHandler` ใช้ `Catalog` แทนตัวโหลด template เดิมของ Plan 02 โดยย้าย `otp.<lang>.txt` เป็น `otp.<lang>.tmpl` แบบ `define`)
- Test: `services/notify/catalog_test.go`, `services/notify/prefs_test.go`

**Interfaces:**
- Produces:
  - `type Message struct { Lang, Title, Body, EmailSubject, EmailBody string }`
  - `type Catalog struct{...}` และ `func LoadCatalog(fsys fs.FS) (*Catalog, error)` (embed `templates/*.tmpl`) · `func (c *Catalog) Render(key, lang string, data any) (Message, error)` ใช้ไฟล์ `key.lang.tmpl` ถ้าไม่มีไฟล์ภาษานั้นใช้ `key.en.tmpl` แล้ว `Message.Lang` = ภาษาที่ใช้จริง · แต่ละไฟล์มี `{{define "title"}}`, `{{define "body"}}`, `{{define "subject"}}`, `{{define "email"}}` (`text/template`, `missingkey=error`)
  - `func NormalizeLang(s string) string` คืน 1 ใน 7 ภาษา ไม่รู้จักคืน `en` (`"zh-CN"` → `zh`)
  - `type Prefs struct { Lang string; MarketingConsent bool; Phone, Email string }`
  - `type PrefsClient interface { GetContactPrefs(ctx context.Context, memberID string) (Prefs, error) }` + `GRPCPrefs{Client memberv1.MemberServiceClient}` (addr env `MEMBER_GRPC_ADDR=member:9090`, timeout 2 วิ) · gRPC `NotFound` → `ErrMemberNotFound`
  - `func Link(lang, path string) string` คืน `WEB_BASE_URL + "/" + lang + path` (ใช้ในอีเมล / SMS) ส่วน push ใช้ `path` ตรง ๆ เป็น `data.link`
  - `var Important = map[string]bool{"ticket_issued": true, "ticket_received": true, "order_refunded": true, "agent_decision": true, "payment_discrepancy": true}` (`referral_created`, `ticket_sent` = push อย่างเดียว)
- ตัวแปรใน template: `ticket_issued {OrderNo, Count, VisitDate, Link}` · `ticket_received {Link}` · `ticket_sent {Link}` · `order_refunded {OrderNo, AmountBaht, Mode, Link}` · `referral_created {Pending bool, Link}` · `agent_decision {Kind, Decision, Reason, Link}` · `payment_discrepancy {PaymentID, OrderID, ExpectedBaht, ActualBaht}` (ภาษาไทยเท่านั้นพอ ภาษาอื่นตกไป EN ได้) · ทุกภาษามีชื่อแบรนด์ (ตรวจคำแปลเป็นงาน NN 1.18.3 / Q22)

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestAllKeysRenderInSevenLangs(t *testing.T)      // ทุก key ยกเว้น payment_discrepancy × 7 ภาษา: Title/Body ไม่ว่าง, Lang ตรงกับที่ขอ
func TestUnknownLangFallsBackToEN(t *testing.T)       // "ru", "", "zh-TW" → Message.Lang == "en" (zh-TW ไม่ใช่ตัวย่อ ใช้ en)
func TestMissingLangFileFallsBackToEN(t *testing.T)   // fstest.MapFS มีแค่ x.en.tmpl → Render("x","ja") ได้ EN
func TestMissingVarIsError(t *testing.T)              // data ขาด OrderNo → error (ไม่ส่ง "<no value>")
func TestReferralBodyHasNoPII(t *testing.T)           // render ทุกภาษา ไม่มี "+66", "@", ชื่อคน
func TestOTPStillRenders(t *testing.T)                // otp ทั้ง 7 ภาษายังมีรหัส 6 หลัก (test ของ Plan 02 ต้องผ่านต่อ)
func TestGRPCPrefsNotFound(t *testing.T)              // bufconn fake server ตอบ NotFound → ErrMemberNotFound
```

- [ ] **Step 2: รันให้ fail** — `make test PKG=./services/notify/... RUN='Render|Lang|Prefs|OTP'`
- [ ] **Step 3: เขียน `Catalog`, `NormalizeLang`, `GRPCPrefs`, `Link`, ไฟล์ template 7 ภาษา**
- [ ] **Step 4: รันให้ผ่าน** รวม `make test-int PKG=./services/notify/... RUN=OTP` ของ Plan 02
- [ ] **Step 5: Commit** — `git commit -m "feat(notify): 7-language template catalog with en fallback, contact prefs client"`

### Task 3: Consumer domain event → pending deliveries

**Files:**
- Create: `services/notify/events.go`
- Modify: `services/notify/main.go`
- Test: `services/notify/events_test.go` (integration, `PrefsClient` แบบ fake)

**Interfaces:**
- Consumes: `consumer.Run` (Config.Name = `notify-events`, EventTypes = `ticket.issued, ticket.transferred, referral.created, agent.approved, order.refunded, payment.discrepancy` และ Config.Name = `notify-account-deleted`, EventTypes = `account.deleted`), `Catalog.Render`, `PrefsClient`, `DeviceStore.ByMember/BySubject`, payload ตาม contracts §5
- Produces:
  - `type Planner struct { Cat *Catalog; Prefs PrefsClient; Devices *DeviceStore; FinanceEmails []string /* env FINANCE_ALERT_EMAILS คั่นด้วย , */ }`
  - `func (p *Planner) Handler() consumer.Handler` แยกตาม `e.Type` เป็นเมธอด `onTicketIssued`, `onTicketTransferred`, `onReferral`, `onAgentApproved`, `onRefunded`, `onDiscrepancy`
  - `func (p *Planner) Enqueue(ctx, tx *sql.Tx, sourceID, key string, r Recipient, data any) (int, error)` render แล้ว `INSERT IGNORE` แถว `deliveries` สถานะ `pending` ต่อเครื่อง (`Recipient.App` ไม่ว่าง = เฉพาะเครื่องที่ `app` ตรง) (`channel` = `fcm` สำหรับ ios / gms, `hms`, `webpush`) และถ้า `Important[key]` เพิ่มแถว `email` (มีอีเมล) หรือ `sms` (ไม่มีอีเมลแต่มีเบอร์) คืนจำนวนแถวที่เพิ่ม
  - `type Recipient struct { MemberID, Subject, Email, Phone, Lang, App string }` (`Subject` ใช้หาเครื่องเมื่อไม่มี memberId)
  - `func (p *Planner) OnAccountDeleted(ctx, tx *sql.Tx, e events.Envelope) error`: ลบแถว `devices` ที่ `member_id = memberId` หรือ `subject = contact` และ `deliveries` ที่ยัง `pending` ของผู้รับนั้นเป็น `failed` + `address = NULL` (`last_error = "account_deleted"`)

กติกาผู้รับต่อ event:

| Event | ผู้รับ | key | หมายเหตุ |
|---|---|---|---|
| `ticket.issued` | `memberId` → GetContactPrefs + เครื่องของสมาชิก · ไม่มี memberId ใช้ `guestContact` (อีเมลหรือ E.164) ไม่มีเครื่อง | `ticket_issued` | |
| `ticket.transferred` | ผู้รับ: `toMemberId` (ถ้ามี) + `toContact`, ภาษา `lang` · ผู้ส่ง: `fromMemberId` ภาษาจาก prefs | `ticket_received`, `ticket_sent` | F11 "Push แจ้งทั้งสองฝ่าย" |
| `referral.created` | ไกด์: `guideMemberId` → GetContactPrefs + เครื่องของสมาชิกนั้นที่ `app=agent` | `referral_created` | `Pending = len(flags) > 0` · push อย่างเดียว |
| `agent.approved` | `contactEmail` + เครื่อง `app=agent` ของ `memberId` (ถ้ามี) ไม่งั้นเครื่องที่ subject = `contactPhone` | `agent_decision` | ภาษา `lang` ของ event |
| `order.refunded` | จาก event โดยตรง: `memberId` → GetContactPrefs + เครื่อง · ไม่มีใช้ `guestContact` | `order_refunded` | ภาษา `lang` ของ event (ใช้ได้แม้คืนเงินก่อนออกบัตร) |
| `payment.discrepancy` | `FinanceEmails` ภาษา `th` อีเมลอย่างเดียว | `payment_discrepancy` | |

- ข้อความทุก event ใน task นี้เป็นข้อความบริการ ไม่ต้องตรวจ consent · `GetContactPrefs` error อื่นที่ไม่ใช่ `ErrMemberNotFound` ให้ return error (consumer Nak แล้ว retry)

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestTicketIssuedMemberTwoDevicesPlusEmail(t *testing.T)   // สมาชิกมีเครื่อง gms + web, prefs มีอีเมล → deliveries: fcm, webpush, email (3 แถว pending) ภาษาตาม prefs
func TestTicketIssuedGuestPhoneOnlyGetsSMS(t *testing.T)       // guestContact "+8613800138000" → 1 แถว sms ภาษา zh
func TestRedeliveredTicketIssuedCreatesNoDuplicateDeliveries(t *testing.T) // เรียก Enqueue 2 รอบด้วย source เดียวกัน → ยัง 3 แถว
func TestTransferNotifiesBothSides(t *testing.T)               // ผู้รับได้ ticket_received (push + อีเมล), ผู้ส่งได้ ticket_sent (push เท่านั้น)
func TestReferralGoesToGuideAgentApp(t *testing.T)             // referral.created guideMemberId → แถว fcm เฉพาะเครื่อง app=agent ของไกด์ (เครื่อง app=tourist ของคนเดียวกันไม่ได้), body ไม่มีข้อมูลนักท่องเที่ยว
func TestReferralFlaggedSaysPending(t *testing.T)              // flags ["burst"] → data ของ template มี Pending = true
func TestRefundNotifiesFromEvent(t *testing.T)                 // order.refunded {memberId, lang:"ja"} โดยไม่เคยมี ticket.issued → แถว order_refunded ภาษา ja ถึงสมาชิก
func TestRefundGuestContact(t *testing.T)                      // order.refunded {guestContact:"a@example.com"} → แถว email 1 แถว
func TestPushDataTypeAndLink(t *testing.T)                     // ticket.transferred → data ของแถว push == {"type":"ticket_received","link":"/tickets/<newTicketId>"}
func TestAccountDeletedRemovesDevices(t *testing.T)            // account.deleted {memberId, contact} → devices ของคนนั้นถูกลบ, deliveries pending เป็น failed และ address NULL, redeliver ซ้ำไม่ error
func TestDiscrepancyEmailsFinance(t *testing.T)                // FINANCE_ALERT_EMAILS 2 ราย → 2 แถว email ภาษา th
func TestPrefsUnavailableNaks(t *testing.T)                    // fake คืน Unavailable → handler error, ไม่มีแถว (rollback)
```

- [ ] **Step 2: รันให้ fail** — `make test-int PKG=./services/notify/... RUN='Ticket|Transfer|Referral|Refund|Discrepancy|Prefs|PushData|AccountDeleted'`
- [ ] **Step 3: เขียน `Planner`** `target` = device id สำหรับ push, `sha256hex(address)` สำหรับ email / sms · `address_masked` เหลือ 4 ตัวท้าย · `data` = `{"type": key, "link": path}` ตาม contracts หัวข้อ 10
- [ ] **Step 4: ต่อ consumer `notify-events` และ `notify-account-deleted` ใน `main.go`** (คู่กับ `notify-otp` เดิม)
- [ ] **Step 5: รันให้ผ่าน**
- [ ] **Step 6: Commit** — `git commit -m "feat(notify): event consumers plan deliveries per recipient"`

### Task 4: Dispatcher — routing ตาม platform, อีเมลซ้ำ, ลบ token เสีย, ประวัติการส่ง

**Files:**
- Create: `services/notify/dispatch.go`, `services/notify/webpush.go`
- Modify: `services/notify/main.go`, `api/openapi/notify.yaml`, `Makefile` (target `vapid-keys`: `docker compose run --rm gotest go run ./services/notify/cmd/vapidkeys` พิมพ์ `VAPID_PUBLIC_KEY=…` / `VAPID_PRIVATE_KEY=…` ด้วย `webpush.GenerateVAPIDKeys`), Create `services/notify/cmd/vapidkeys/main.go`
- Test: `services/notify/dispatch_test.go` (integration), `services/notify/webpush_test.go`

**Interfaces:**
- Consumes: ตาราง `deliveries`, `DeviceStore.Disable`, `SMSSender`, `EmailSender` (Plan 02 Task 5)
- Produces:
  - `type PushMessage struct { Token, P256dh, Auth, Title, Body, Lang string; Data map[string]string }`
  - `type PushSender interface { Send(ctx context.Context, m PushMessage) error }` และ `var ErrUnregistered = errors.New("unregistered")`
  - `HTTPPushSender{URL string; Client *http.Client}` ใช้ทั้ง FCM และ HMS: POST `{token, title, body, data, lang}` ไป `FCM_URL` / `HMS_PUSH_URL` (contract ของ mockext) · `404` body `UNREGISTERED` → `ErrUnregistered` · timeout 5 วิ · ponytail: เมื่อต่อ FCM v1 / HMS จริงให้เขียน adapter ใหม่ที่ implement `PushSender`
  - `WebPushSender{PublicKey, PrivateKey, Subject string; Client *http.Client}` ใช้ `webpush.SendNotification` payload JSON `{"title","body","lang","data":{"type","link"}}`, `TTL: 86400`, `Urgency: high` สำหรับ key สำคัญ · `404` / `410` → `ErrUnregistered`
  - `type Dispatcher struct { DB *sql.DB; FCM, HMS, Web PushSender; SMS SMSSender; Mail EmailSender; Devices *DeviceStore; Now func() time.Time; Batch int /* 50 */ }`
  - `func (d *Dispatcher) Tick(ctx) (sent int, err error)` และ `Run(ctx)` (ทุก 5 วิ)
  - `GET /deliveries?sourceId=&status=&cursor=&limit=` (staff role `admin`) คืนประวัติการส่ง (address แบบปิดบังเท่านั้น)

Tick: `BEGIN` → `SELECT … WHERE status='pending' AND next_attempt_at <= now ORDER BY next_attempt_at LIMIT ? FOR UPDATE SKIP LOCKED` → ส่งตาม `channel` → สำเร็จ: `status='sent', sent_at, address=NULL` · `ErrUnregistered`: `status='invalid_token'` + `Devices.Disable(device_id)` · error อื่น: `attempts+1`, `next_attempt_at` ตาม backoff ใน Global Constraints, ครบ 5 ครั้ง `status='failed'` → `COMMIT` · ส่งแต่ละแถวแยก error กันไม่ให้แถวหนึ่งหยุดทั้ง batch · ponytail: ส่ง HTTP ขณะถือ lock ของแถว ยอมรับได้ที่ขนาด demo ถ้าช้าค่อยเปลี่ยนเป็น claim (`status='sending'`) แล้วส่งนอก transaction

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestRoutesByPlatform(t *testing.T)               // แถว fcm / hms / webpush / email / sms ไปถึง fake sender ที่ถูกตัว ตัวละ 1 ครั้ง, ทุกแถว status=sent, address = NULL
func TestUnregisteredDisablesDevice(t *testing.T)     // fake FCM คืน ErrUnregistered → status=invalid_token, devices.disabled_at ไม่ NULL, ByMember ไม่คืนเครื่องนี้
func TestProviderDownRetriesThenFails(t *testing.T)   // fake คืน 500 ตลอด, เลื่อน Now ตาม backoff → attempts 5, status=failed, device ไม่ถูก disable
func TestOneFailureDoesNotBlockBatch(t *testing.T)    // แถวแรก error แถวที่สอง sent
func TestTwoDispatchersNoDoubleSend(t *testing.T)     // 100 แถว, Tick พร้อมกัน 2 ตัว → fake นับรวม 100 ครั้งพอดี
func TestHTTPPushSenderAgainstMockext(t *testing.T)   // httptest ที่ทำตัวเหมือน mockext: token "invalid-x" → ErrUnregistered, ภาษาเกาหลีถึงปลายทางไม่เพี้ยน
func TestWebPushSendsVapidEncrypted(t *testing.T)     // สร้าง subscription ด้วยคู่กุญแจ P-256 จริง, endpoint = httptest → header Authorization ขึ้นต้น "vapid t=", Content-Encoding "aes128gcm", TTL "86400"
func TestWebPush410IsUnregistered(t *testing.T)
func TestDeliveriesHistoryMasked(t *testing.T)        // GET /deliveries ไม่มีอีเมล / เบอร์เต็มใน response
```

- [ ] **Step 2: รันให้ fail** — `make test-int PKG=./services/notify/... RUN='Route|Unregistered|Provider|Dispatch|Push|Deliveries'`
- [ ] **Step 3: เขียน senders, `Dispatcher`, handler `/deliveries`**
- [ ] **Step 4: รันให้ผ่าน** `-race`
- [ ] **Step 5: ทดสอบปลายทางบน local** (`make up`, ตั้ง VAPID ด้วย `make vapid-keys` ที่รัน `webpush-go` ใน container แล้วพิมพ์คู่กุญแจ): ลงทะเบียนเครื่อง `gms` token `demo-1` และ `invalid-demo` → publish `ticket.issued` ผ่าน `services/example` หรือซื้อบัตรจริง → เห็น Push ใน Mock Console แท็บ Push, อีเมลใน Mailpit `http://localhost:8025`, เครื่อง `invalid-demo` ถูก disable · เปิด `http://localhost:3000` บน Chrome กดรับแจ้งเตือน (Plan 13) แล้วเห็น Web Push จริงขึ้นที่ศูนย์แจ้งเตือนแม้ปิดแท็บ
- [ ] **Step 6: Commit** — `git commit -m "feat(notify): push routing fcm/hms/webpush, email duplicate, invalid token cleanup"`

### Task 5: Push campaign ตั้งเวลา / เลือกกลุ่ม + consent + seed

**Files:**
- Create: `services/notify/campaign.go`, `services/notify/seed.go`
- Modify: `services/notify/main.go`, `api/openapi/notify.yaml`
- Test: `services/notify/campaign_test.go` (integration)

**Interfaces:**
- Consumes: `PrefsClient`, `NormalizeLang`, ตาราง `devices`, `deliveries` (Dispatcher ของ Task 4 ส่งต่อเอง)
- Produces:
  - `type Segment struct { Langs []string; Platforms []string; Apps []string /* tourist|agent|web */; MemberIDs []string }` (ช่องว่าง = ไม่กรอง, เงื่อนไขต่างช่องเป็น AND)
  - `type Campaign struct { ID, Name, Kind /* marketing|service */ string; Messages map[string]struct{ Title, Body string }; URL string; Segment Segment; Status string; ScheduledAt *time.Time; Stats map[string]int }`
  - HTTP (staff role `admin` หรือ `editor`, `GET` ให้ `viewer` ด้วย): `POST /campaigns` → `201` (status `draft`) · `PUT /campaigns/{id}` (draft เท่านั้น ไม่ใช่ได้ `409 not_draft`) · `POST /campaigns/{id}/schedule {scheduledAt}` → `scheduled` (`scheduledAt` ในอดีตเกิน 1 นาที → `400 scheduled_in_past`) · `POST /campaigns/{id}/cancel` (ก่อน `sending` เท่านั้น) · `GET /campaigns`, `GET /campaigns/{id}` (มี `stats` นับ `deliveries` ตาม status + `skippedConsent`)
  - validation: ต้องมี `messages.en` (`400 en_required`), title ≤ 65 ตัวอักษร, body ≤ 240, `url` ขึ้นต้นด้วย `/` (`400 invalid_url`), ภาษาใน `messages` ต้องอยู่ใน 7 ภาษา
  - `type Scheduler struct { DB *sql.DB; Prefs PrefsClient; Now func() time.Time }` + `Tick(ctx) (int, error)` / `Run(ctx)` ทุก 5 วิ
  - `POST /internal/seed` (`DEMO_MODE=true` เท่านั้น ไม่ใช่ได้ `404`, `make seed` เรียกหลัง ticket ตาม contracts หัวข้อ 7) สร้าง campaign draft id คงที่จาก `deploy/seed/fixtures.json` ชื่อ "โปรโมชันเทศกาลดอกไม้" ข้อความ 7 ภาษา url `/shows` segment ว่าง · รันซ้ำได้ (`INSERT IGNORE`)

Scheduler Tick: `SELECT … WHERE status='scheduled' AND scheduled_at <= now FOR UPDATE SKIP LOCKED LIMIT 1` → `status='sending'` → เลือกเครื่อง `disabled_at IS NULL` ตาม segment → จัดกลุ่มตาม `member_id` → **marketing:** เครื่องที่ไม่มี `member_id` ข้าม และเรียก `GetContactPrefs` ทีละสมาชิก ถ้า `!MarketingConsent` นับ `skipped_consent` แล้วข้าม · ภาษา = prefs.lang → device.lang → `en` แล้วเลือก `messages[lang]` ไม่มีใช้ `messages.en` · `INSERT IGNORE deliveries` (`source_id` = campaign id, `template_key` = `campaign`, `data` = `{"type":"campaign","link": url}`, ไม่ส่งอีเมลซ้ำ) → `status='sent'` → `COMMIT` · ตรวจ `Segment.Langs` กับภาษาที่ resolve แล้ว ไม่ใช่ device.lang อย่างเดียว · ponytail: เรียก gRPC ทีละคน ถ้าสมาชิกเกินหลักหมื่นให้เพิ่ม `BatchGetContactPrefs` (ดูหัวข้อ "ข้อเสนอแก้ contracts")

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestMarketingSkipsNoConsent(t *testing.T)        // 3 สมาชิก: consent true, false, true → deliveries 2 สมาชิก, stats.skippedConsent == 1
func TestMarketingSkipsAnonymousDevice(t *testing.T)  // เครื่องที่ member_id NULL → ไม่ได้รับ
func TestServiceCampaignIgnoresConsent(t *testing.T)  // kind=service → ทุกเครื่องได้รับ
func TestConsentCheckedAtSendTime(t *testing.T)       // schedule ตอน consent=true, เปลี่ยน fake เป็น false ก่อน Tick → ไม่ได้รับ
func TestSegmentByLangAndPlatform(t *testing.T)       // Langs [zh], Platforms [hms] → เฉพาะเครื่อง hms ที่ภาษา zh
func TestMessageFallbackToEN(t *testing.T)            // messages มีแค่ th, en; สมาชิกภาษา ja → ข้อความ en
func TestNotDueNotSent(t *testing.T)                  // scheduled_at = now+10m → Tick == 0
func TestTickTwiceSendsOnce(t *testing.T)             // Tick 2 ครั้ง / 2 Scheduler พร้อมกัน → deliveries ไม่ซ้ำ
func TestCancelScheduled(t *testing.T)                // cancel แล้ว Tick → 0 แถว, status cancelled
func TestValidation(t *testing.T)                     // ไม่มี en → 400 en_required, url "https://x" → 400 invalid_url, แก้ campaign ที่ scheduled → 409 not_draft
func TestEditorRoleRequired(t *testing.T)             // X-Roles "viewer" POST → 403
func TestSeedIdempotentAndDemoOnly(t *testing.T)      // seed 2 ครั้ง → 1 campaign; DEMO_MODE=false → 404
```

- [ ] **Step 2: รันให้ fail** — `make test-int PKG=./services/notify/... RUN='Campaign|Marketing|Consent|Segment|Seed|Cancel|Validation|Tick'`
- [ ] **Step 3: เขียน API, `Scheduler`, seed** และเพิ่มลง `notify.yaml` แล้ว `make gen` (Plan 14 ใช้ TS client ทำหน้า CMS)
- [ ] **Step 4: รันให้ผ่าน** `-race` และ `make test-int PKG=./services/notify/...` ทั้ง package
- [ ] **Step 5: ทดสอบปลายทางบน local**: `make seed` แล้ว schedule campaign seed อีก 1 นาที → เห็นใน Mock Console เฉพาะสมาชิก demo ที่ยินยอม
- [ ] **Step 6: Commit** — `git commit -m "feat(notify): scheduled push campaigns with segment and marketing consent"`

---

## ตาราง env ของ notify

| env | ค่าใน demo |
|---|---|
| `FCM_URL`, `HMS_PUSH_URL` | `http://mockext:8090/push/fcm`, `http://mockext:8090/push/hms` |
| `VAPID_PUBLIC_KEY`, `VAPID_PRIVATE_KEY`, `VAPID_SUBJECT` | จาก `make vapid-keys`, `mailto:dev@nongnooch.example` |
| `SMTP_ADDR`, `SMS_BASE_URL` | `mailpit:1025`, `http://mockext:8090/sms` (Plan 02) |
| `MEMBER_GRPC_ADDR` | `member:9090` |
| `WEB_BASE_URL` | `http://localhost:3000` |
| `FINANCE_ALERT_EMAILS` | `finance@demo.nongnooch.local` (staff seed contracts หัวข้อ 7) |
| `DEMO_MODE` | `true` |

## ไม่อยู่ในแผนนี้

- `content.published` (O3 มีเส้นไป Notification): ไม่ส่ง Push อัตโนมัติทุกครั้งที่เผยแพร่ เพราะเสี่ยงส่งข่าวการตลาดโดยไม่ผ่านการตั้งค่ากลุ่ม ให้ทีมการตลาดสร้าง campaign แทน (คำถามเปิดข้อ 1)
- แจ้งเตือนรอบการแสดงใกล้เริ่ม (E5) ทำฝั่งแอปเป็น local notification (Plan 15) หรือเป็น campaign แบบ service
- LINE OA (Q8), ชวนเพิ่มเว็บลงหน้าจอโฮม / PWA manifest (Q25 เป็นงาน Plan 13)

## ข้อเสนอแก้ contracts

- รวมเข้า contracts v2 แล้ว (หัวข้อ 5 `order.refunded` มี `memberId?, guestContact?, lang`, หัวข้อ 7 ลำดับ seed มี notify และ report, หัวข้อ 10 deep link / push data / `app`)
- **ยังไม่รับ (ทีหลัง):** `MemberService.BatchGetContactPrefs(repeated member_id)` สำหรับ campaign ขนาดใหญ่
- **นอก contracts:** system-design O3 เพิ่มเส้น `order.refunded → Notification` ให้ตรงกับ WBS 1.9.3
