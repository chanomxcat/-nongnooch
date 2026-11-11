# Member & Point Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** นักท่องเที่ยวที่ยืนยัน OTP แล้วสมัครสมาชิกได้ด้วยชื่อ + สัญชาติ + เลขบัตรประชาชน (ตรวจ check digit) หรือพาสปอร์ต ระบบเก็บเลขแบบเข้ารหัส + HMAC กันบัญชีซ้ำ ให้ Welcome / Invite point ลง PointLedger แบบ append-only ให้พ้อยไกด์สถานะ "รอตรวจ" จาก `referral.created` เก็บ Consent, ผู้ติดตาม, ประวัติซื้อ / เข้าชม และเปิด gRPC ให้ Identity, Commerce, Notification ใช้

**Architecture:** service `member` (schema `member`) บน chassis ของ Plan 01 · HTTP ผ่าน gateway ที่ `/api/member/...` อ่านตัวตนจาก `authn.From(ctx)` · ตอนสมัครเรียก gRPC `AgentService.ValidateInvite` แล้วเขียน Member + Consent + PointLedger + outbox ใน transaction เดียว · consumer รับ `referral.created` (ใช้ `guideMemberId`), `price.approved` (entity `point_rule`), `order.paid`, `ticket.scanned`, `account.deleted` · gRPC `MemberService` ที่ `member:9090` · Identity เรียก `GetMemberByContact` เพื่อใส่ `mid` ใน JWT

**Tech Stack:** Go 1.23 + chassis (Plan 01), `crypto/aes` + `crypto/cipher` (AES-256-GCM), `crypto/hmac`, `google.golang.org/grpc`, stub จาก `make gen` (`gen/go/member/v1`, `gen/go/agent/v1`)

**Spec:** [docs/system-design.md](../../system-design.md) F1, F3, S1 ขั้น 21–35, S7 (GetContactPrefs), หัวข้อ 1.2 (Member & Point), หัวข้อ 2 (ข้อมูลตัวตน) · [ROADMAP](../../../roadmap/ROADMAP.md) E9, หัวข้อ 12 PDPA · [contracts](2026-11-11-contracts.md) · ขึ้นกับ [Plan 01](2026-11-11-01-platform-chassis.md), [Plan 02](2026-11-11-02-identity-otp.md), Plan 12 Task 1 (`pkg/chassis/authn`)

## Global Constraints

- **Build และ test ใน Docker เท่านั้น:** `make test`, `make test-int`, `make build`, `make gen`, `make seed` ห้ามรัน `go` / `npm` / `flutter` บนเครื่องโดยตรง integration test ขึ้นต้น `//go:build integration` และรันด้วย `make test-int`
- **โหมด Demo:** ระบบภายนอกทุกตัวใช้ mockext / Mailpit (Plan 00) เรียกผ่าน interface + base URL ใน env (แผนนี้ไม่เรียกระบบภายนอกโดยตรง)
- **ใช้ชื่อ / type ตาม [contracts](2026-11-11-contracts.md) ห้ามแก้** ถ้าต้องแก้ให้เสนอในหัวข้อ "ข้อเสนอแก้ contracts" ท้ายแผน
- **Seed:** `POST /internal/seed` ทำงานเมื่อ `DEMO_MODE=true` เท่านั้น และรันซ้ำได้ (idempotent) (Task 8)
- ข้อกำหนดทั้งหมดใน Global Constraints ของ Plan 01 (1 schema ต่อ service, outbox ใน tx เดียว, consumer idempotent, OTel)
- contracts §1: "เลขบัตรประชาชน / พาสปอร์ต: ส่งข้าม service ได้แค่ `idLast4` และ `nationality` ห้ามส่งเลขเต็มใน event, log หรือ response (ยกเว้น API ถอดรหัสของ Member ที่ต้องมีสิทธิ์)"
- "เลขบัตรประชาชน / พาสปอร์ต เข้ารหัสระดับฟิลด์ (key ใน Secret manager) + HMAC hash สำหรับตรวจซ้ำ" · "แสดงแบบปิดบัง 4 หลักท้ายทุกหน้าจอ, ถอดรหัสได้เฉพาะบทบาทที่ได้สิทธิ์"
- E9: "คนไทยกรอก **เลขบัตรประชาชน 13 หลัก** (ตรวจ Check digit), ต่างชาติกรอก **เลขพาสปอร์ต + ประเทศที่ออก** (ตรวจรูปแบบ), 1 เลขบัตร / พาสปอร์ต = 1 บัญชี" · "1 เบอร์ / 1 อีเมล / 1 เลขบัตร ต่อ 1 บัญชี" · "แก้เลขเองไม่ได้หลังยืนยัน ต้องแจ้งเจ้าหน้าที่"
- E9: "บัญชีที่ยืนยันด้วยเลขบัตรประชาชนเห็นและซื้อราคาคนไทยได้, บัญชีพาสปอร์ตเห็นราคาต่างชาติ"
- E9: "ไม่มีหน้าหรือ API สำหรับแลกพ้อยในเฟส 1, บันทึกทุกรายการในบัญชีพ้อย (Ledger) แบบแก้ย้อนหลังไม่ได้"
- S1: "มีบัญชีอยู่แล้ว → ใช้บัญชีเดิม (ไม่ให้พ้อยซ้ำ)" · "PointLedger(ไกด์, สถานะ=รอตรวจ) idempotent ด้วย referralId" · มติ 9 พ.ย.: ไกด์ได้พ้อย "**ไม่ต้องรอให้นักท่องเที่ยวสแกนเข้าประตู**"
- F1: "ข่าวสารการตลาดแยกช่อง ไม่ติ๊กไว้ก่อน" · PDPA: "เก็บ Consent log ทุกครั้งที่ให้หรือถอนความยินยอม"
- E9: Invite code "กรอกได้ครั้งเดียวต่อบัญชี ตอนสมัครหรือภายใน [__ วัน]" · E9: "บัตรที่ซื้อแบบ Guest ผูกภายหลังได้"
- เงินเป็น `int64` สตางค์ (`…Satang`), เวลา UTC RFC3339, `visitDate` เป็น `YYYY-MM-DD` เวลา Asia/Bangkok, ID เป็น UUIDv7, JSON camelCase, error `{"error","message"}`, list `?cursor=&limit=` (20 / สูงสุด 100)

- **ลบบัญชี (contracts §1, มติ 11 พ.ย. 2026):** ลบชื่อ, ช่องทางติดต่อ และเลขบัตรที่เข้ารหัส แต่**เก็บ `id_hmac`** เพื่อกันลบแล้วสมัครใหม่รับ Welcome point ซ้ำ (Task 6)

**ค่าที่แผนนี้กำหนดเพิ่ม เพราะ spec ยังเป็น [__] (ให้ PO ยืนยัน, ตั้งผ่าน env เป็นค่าเริ่ม แล้วแก้ผ่าน `price.approved` entity `point_rule` ได้ ตาม Task 3):** `POINTS_WELCOME=100`, `POINTS_INVITE=50`, `POINTS_GUIDE_REFERRAL=20`, `INVITE_LATE_DAYS=30` (Q6), ผู้ติดตามสูงสุด 20 คนต่อบัญชี, รูปแบบพาสปอร์ต `^[A-Z0-9]{6,9}$`, สัญชาติ / ประเทศเป็น ISO 3166-1 alpha-2 ตัวใหญ่ (`TH`, `CN`), ถอดรหัสเลขเต็มได้เฉพาะ role `admin` (Q26 ยังเปิด)

**ต่างจาก spec:** S1 ส่ง `POST /members {consents, inviteCode, lang}` แผนนี้เพิ่ม `firstName, lastName, nationality, idNumber, passportCountry?, channel` เพราะ F1 ให้กรอกข้อมูลตัวตนในขั้นเดียวกัน

## Review Focus

1. **กดสมัครซ้ำ / 2 แท็บพร้อมกัน** ด้วย contact เดียวกัน ต้องได้ Member 1 ราย Welcome point 1 รายการ คำขอที่สองได้บัญชีเดิม `existing: true` ไม่ใช่ 500 (Task 2, 3)
2. **Agent ล่มหรือโค้ดใช้ไม่ได้ตอนสมัคร:** ต้องสมัครสำเร็จโดยไม่มี Invite point และตอบ `inviteStatus: "unavailable" | "invalid"` เพื่อให้กรอกภายหลังได้ (Task 2, 8)
3. **เลขเดียวกันหลายรูปแบบ:** `1-1017-00230-70-8`, `1101700230708`, พาสปอร์ต `e1234567` กับ `E1234567` ต้องได้ HMAC เดียวกัน และบัญชีที่สองได้ `409 id_number_taken` (Task 1, 2)
4. **`referral.created` ส่งซ้ำ:** ต้องได้ ledger ไกด์ 1 รายการ ลงบัญชี `guideMemberId` ตรง ๆ (ไม่ต้องรอ `agent.approved`) (Task 4)
5. **เลขเต็มรั่ว:** outbox payload, log และ response ของ `/members/me` ต้องไม่มีเลขเต็ม (test สแกน string) (Task 2)

---

## File Structure

```
pkg/thaiid/thaiid.go                     Normalize + check digit 13 หลัก (agent ใช้ตรวจเลขนิติบุคคลด้วย)
api/proto/member/v1/member.proto         MemberService ตาม contracts §4
api/proto/agent/v1/agent.proto           AgentService ตาม contracts §4 (สร้างถ้า Plan 09 ยังไม่สร้าง, server อยู่ Plan 09)
api/openapi/member.yaml
services/member/main.go
services/member/migrations/00001_member.sql      members, consent_log, audit_log
services/member/migrations/00002_points.sql      point_ledger (+ trigger กันแก้), point_rules
services/member/migrations/00003_companions.sql  companions
services/member/migrations/00004_history.sql     purchases, visits
services/member/idcrypto.go              IDCipher (AES-GCM) + HMAC
services/member/identityno.go            ParseIdentity: thai_id / passport + รายการประเทศ
services/member/register.go              POST /members, GET /members/me
services/member/invite.go                InviteValidator (gRPC client ไป agent)
services/member/ledger.go                PointLedger + points API + consumer price.approved (point_rule)
services/member/guidepoints.go           consumer referral.created + ตัดพ้อย
services/member/consent.go               consent + PATCH /members/me
services/member/grpc.go                  MemberService server
services/member/companion.go
services/member/accountdeleted.go        consumer account.deleted (ลบข้อมูลส่วนบุคคล เก็บ id_hmac)
services/member/history.go               consumer order.paid, ticket.scanned + ผูก Guest
services/member/lateinvite.go            POST /members/me/invite
services/member/seed.go                  POST /internal/seed
services/member/http.go                  router + error mapping
services/identity/memberlookup.go        (แผนนี้แก้ identity) ใส่ mid ใน JWT
```

---

### Task 1: Skeleton + schema + เข้ารหัสเลขบัตร + API ถอดรหัส

**Files:**
- Create: `pkg/thaiid/thaiid.go`, `services/member/main.go`, `services/member/migrations/00001_member.sql`, `services/member/idcrypto.go`, `services/member/identityno.go`, `services/member/http.go`, `api/openapi/member.yaml`
- Modify: `deploy/compose.yaml` (เพิ่ม service `member` + env ด้านล่าง)
- Test: `pkg/thaiid/thaiid_test.go`, `services/member/idcrypto_test.go`, `services/member/identityno_test.go`, `services/member/admin_reveal_test.go` (integration)

**Interfaces:**
- Produces:
  - `thaiid.Normalize(s string) string` (ตัดทุกอักขระที่ไม่ใช่ตัวเลข) และ `thaiid.Valid(s string) bool` (13 หลักหลัง normalize + check digit: `sum(d[i]*(13-i)) i=0..11`, `check = (11 - sum%11) % 10`)
  - `type Identity struct { Kind string /* "thai_id" | "passport" */; Number string /* normalize แล้ว */; Country string /* ประเทศที่ออก, thai_id = "TH" */; Last4 string; ThaiVerified bool }`
  - `func ParseIdentity(nationality, idNumber, passportCountry string) (Identity, error)` — `nationality == "TH"` บังคับ thai_id, อื่น ๆ บังคับ passport (upper-case, ตัดช่องว่าง / ขีด) · error ค่าคงที่ `ErrInvalidNationality`, `ErrInvalidThaiID`, `ErrInvalidPassport`, `ErrInvalidCountry`
  - `type IDCipher struct{...}`, `func NewIDCipher(encKeyB64 string, keyVersion byte, hmacKeyB64 string) (*IDCipher, error)` อ่านจาก env `MEMBER_ID_ENC_KEY` (32 byte base64), `MEMBER_ID_KEY_VERSION` (ค่าเริ่ม 1), `MEMBER_ID_HMAC_KEY`
  - `(*IDCipher).Encrypt(plain, aad string) ([]byte, error)` รูปแบบ `version(1) || nonce(12) || ciphertext` · `Decrypt(blob []byte, aad string) (string, error)` · `Hash(id Identity) [32]byte` = HMAC-SHA256 ของ `"thai_id:"+Number` หรือ `"passport:"+Country+":"+Number` · aad = `"member:"+memberID` หรือ `"companion:"+companionID`
  - `func MaskID(last4 string) string` คืน `"****" + last4` (ใช้ใน log / admin)
  - HTTP: `GET /admin/members/{id}/id-number` (`authn.RequireRole("admin")`) ได้ `200 {"idNumber","idKind","country"}` และเขียน `audit_log` action `reveal_id` ทุกครั้ง · `404 member_not_found`
- ฟิลด์ `authn.Principal` ที่แผนนี้ใช้ (contracts §3): `Type, Sub, MemberID, Roles, Lang, ClientIP, DeviceID` (`RequestID` ใช้ใน log)

ตาราง:

```sql
CREATE TABLE members (
  id CHAR(36) PRIMARY KEY, phone VARCHAR(20) NULL UNIQUE, email VARCHAR(254) NULL UNIQUE,
  first_name VARCHAR(100) NOT NULL, last_name VARCHAR(100) NOT NULL, nationality CHAR(2) NOT NULL,
  id_kind VARCHAR(10) NOT NULL, id_country CHAR(2) NOT NULL, id_enc VARBINARY(128) NULL,
  id_hmac BINARY(32) NULL UNIQUE, id_last4 CHAR(4) NOT NULL, thai_verified BOOLEAN NOT NULL,
  lang CHAR(2) NOT NULL, channel VARCHAR(8) NOT NULL,
  invite_code VARCHAR(32) NULL, guide_id CHAR(36) NULL, company_id CHAR(36) NULL, invite_bound_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL, deleted_at DATETIME(6) NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE consent_log (
  id CHAR(36) PRIMARY KEY, member_id CHAR(36) NOT NULL, kind VARCHAR(10) NOT NULL, -- terms|pdpa|marketing
  granted BOOLEAN NOT NULL, version VARCHAR(16) NOT NULL, channel VARCHAR(8) NOT NULL,
  ip VARBINARY(16) NULL, created_at DATETIME(6) NOT NULL, INDEX idx_member_kind (member_id, kind, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE audit_log (
  id CHAR(36) PRIMARY KEY, actor VARCHAR(64) NOT NULL, action VARCHAR(32) NOT NULL,
  target VARCHAR(64) NOT NULL, detail JSON NULL, created_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

- [ ] **Step 1: เขียน test ที่ fail**

```go
// pkg/thaiid
{"1101700230708", true}, {"1-1017-00230-70-8", true}, {"1101700230707", false}, {"110170023070", false}, {"abc", false}
// identityno_test.go
func TestParseThai(t *testing.T)            // ("TH","1-1017-00230-70-8","") → Kind thai_id, Number "1101700230708", Last4 "0708", ThaiVerified true
func TestParseThaiBadCheckDigit(t *testing.T) // → ErrInvalidThaiID
func TestParsePassport(t *testing.T)        // ("CN"," e 1234567 ","CN") → Number "E1234567", Country "CN", ThaiVerified false
func TestThaiMustUseThaiID(t *testing.T)    // ("TH","E1234567","TH") → ErrInvalidThaiID
func TestUnknownCountry(t *testing.T)       // ("XX",...) → ErrInvalidNationality; passportCountry "ZZ" → ErrInvalidCountry
// idcrypto_test.go
func TestEncryptRoundTrip(t *testing.T)     // Decrypt(Encrypt(x, aad), aad) == x; blob[0] == version
func TestWrongAADFails(t *testing.T)        // aad ต่าง → error
func TestHashStableAcrossFormats(t *testing.T) // Hash(Parse("1-1017-…")) == Hash(Parse("1101700230708"))
func TestCiphertextNotDeterministic(t *testing.T) // Encrypt 2 ครั้งได้ blob ต่างกัน
// admin_reveal_test.go (integration)
func TestRevealRequiresAdmin(t *testing.T)  // X-Roles: viewer → 403; admin → 200 idNumber เต็ม + audit_log 1 แถว actor = X-User-Sub
```

- [ ] **Step 2: รันให้ fail** — `make test PKG=./pkg/thaiid/... ./services/member/...`
- [ ] **Step 3: เขียน `thaiid`, `ParseIdentity`, `IDCipher`, `MaskID`** รายการประเทศเป็น `var isoCountries = map[string]bool{...}` ฝังในไฟล์ (249 รหัส)
- [ ] **Step 4: เขียน `main.go`** ตามแบบ `services/example` ของ Plan 01 (config.Load → db.Open → db.Migrate → server.New → outbox Relay → consumers) + handler reveal และเพิ่ม `member` ใน compose พร้อม env `MEMBER_ID_ENC_KEY`, `MEMBER_ID_HMAC_KEY` (ค่า demo ใน `deploy/.env.demo`), `AGENT_GRPC_ADDR=agent:9090`, `DEMO_MODE=true`
- [ ] **Step 5: รันให้ผ่าน** — `make test PKG=./pkg/thaiid/... ./services/member/...` และ `make test-int PKG=./services/member/...`
- [ ] **Step 6: Commit** — `git commit -m "feat(member): schema, field encryption, id parsing, admin reveal"`

### Task 2: สมัครสมาชิก (`POST /members`) + สิทธิ์ราคาไทย / ต่างชาติ

**Files:**
- Create: `services/member/register.go`, `services/member/invite.go`, `api/proto/agent/v1/agent.proto` (ถ้ายังไม่มี ให้เขียนตาม contracts §4 ตรงตัว)
- Modify: `services/member/http.go`, `api/openapi/member.yaml`
- Test: `services/member/register_test.go` (integration)

**Interfaces:**
- Consumes: `ParseIdentity`, `IDCipher`, `outbox.Write`, `events.New(events.MemberRegistered, …)`, `authn.From`
- Produces:
  - `type InviteResult struct { Valid bool; GuideID, CompanyID, GuideMemberID string }` (`guide_member_id` จาก `ValidateInvite` ตาม contracts §4) · `type InviteValidator interface { ValidateInvite(ctx context.Context, code string) (InviteResult, error) }` · `GRPCInviteValidator` ใช้ `agentv1.AgentServiceClient` timeout 2 วินาที
  - `type RegisterInput struct { Contact, FirstName, LastName, Nationality, IDNumber, PassportCountry, InviteCode, Lang, Channel, IP, DeviceID string; Consents ConsentInput }` · `type ConsentInput struct { Terms, PDPA, Marketing bool }`
  - `type RegisterResult struct { Member MemberView; Existing bool; InviteStatus string /* "none"|"applied"|"invalid"|"unavailable" */ }`
  - `func (s *MemberService) Register(ctx context.Context, in RegisterInput) (RegisterResult, error)` error: `ErrConsentRequired`, `ErrNameRequired`, `ErrIDNumberTaken`, error ของ `ParseIdentity`
  - `func (s *MemberService) RegisterTx(ctx, tx *sql.Tx, m *Member, in RegisterInput) error` — hook ที่ Task 3 (พ้อย) และ Task 7 (ผูก Guest) เพิ่มงานลงใน tx เดียวกัน (slice `s.onRegister []func(ctx, tx, *Member, RegisterInput) error`)
  - `MemberView` (JSON): `{memberId, firstName, lastName, nationality, idKind, idLast4, thaiVerified, lang, contact, inviteCode?, inviteDeadline?, createdAt}`
  - HTTP: `POST /members` (`authn.Require("user")`, contact = `X-User-Sub`: ขึ้นต้น `+` = phone, มี `@` = email) body `{firstName, lastName, nationality, idNumber, passportCountry?, consents:{terms,pdpa,marketing}, inviteCode?, lang, channel: "web"|"app"|"qr"}` ได้ `201 {member, inviteStatus}` หรือ `200 {member, existing:true}` · error `400 consent_required|name_required|invalid_nationality|invalid_thai_id|invalid_passport|invalid_country`, `409 id_number_taken` · `GET /members/me` ได้ `200 MemberView` หรือ `404 member_not_found`
  - payload `member.registered` ตาม contracts §5: `{memberId, nationality, thaiVerified, lang, channel, marketingConsent, inviteCode?, guideId?, companyId?, ip, deviceId, registeredAt}` (ใส่ inviteCode / guideId / companyId เฉพาะเมื่อ `InviteStatus == "applied"`, `marketingConsent` = `consents.marketing`)
- ลำดับ: contact มีบัญชีแล้ว → คืนบัญชีเดิม (ไม่ตรวจข้อมูลอื่น) → ตรวจ consent (`terms` และ `pdpa` ต้อง true) + ชื่อ + `ParseIdentity` → ถ้ามี inviteCode เรียก `ValidateInvite` **นอก tx** (error → `unavailable`, valid=false → `invalid`) → `BEGIN` insert member (`id_enc`, `id_hmac`) + consent_log 3 แถว + hooks + outbox `COMMIT` · duplicate key ที่ `phone`/`email` → อ่านบัญชีเดิมคืน `existing:true` · duplicate ที่ `id_hmac` → `ErrIDNumberTaken`
- lang ที่ไม่อยู่ใน `th en zh ja ko de fr` ให้เก็บเป็น `en`

- [ ] **Step 1: เขียน test ที่ fail** (fake `InviteValidator`)

```go
func TestRegisterThaiCreatesMemberAndEvent(t *testing.T)    // 201, thaiVerified true, outbox 1 แถว member.registered (marketingConsent false), consent_log 3 แถว (marketing granted=false)
func TestRegisterForeignPassport(t *testing.T)              // CN + passport → thaiVerified false, idLast4 "4567"
func TestExistingContactReturnsSameMember(t *testing.T)     // ครั้งที่สอง → 200 existing:true, members 1 แถว, outbox ยัง 1 แถว
func TestConcurrentRegisterSameContact(t *testing.T)        // 10 goroutine → 201 หนึ่งครั้ง ที่เหลือ 200 existing, members 1 แถว
func TestSameIDDifferentContact409(t *testing.T)            // เลขเดิมคนละรูปแบบ เบอร์ใหม่ → 409 id_number_taken
func TestConsentRequired(t *testing.T)                      // pdpa=false → 400 consent_required
func TestInviteApplied(t *testing.T)                        // fake valid → inviteStatus applied, payload มี guideId / companyId
func TestInviteInvalidStillRegisters(t *testing.T)          // valid=false → 201 inviteStatus invalid, payload ไม่มี inviteCode
func TestInviteUnavailableStillRegisters(t *testing.T)      // fake คืน error → 201 inviteStatus unavailable
func TestNoFullIDAnywhere(t *testing.T)                     // จับ slog + อ่าน outbox.payload + body ของ GET /members/me → ไม่มี "1101700230708"
func TestUnauthenticated401(t *testing.T)                   // ไม่มี X-User-Type → 401
```

- [ ] **Step 2: รันให้ fail** — `make test-int PKG=./services/member/... RUN=TestRegister`
- [ ] **Step 3: รัน `make gen`** ให้ได้ `gen/go/agent/v1`
- [ ] **Step 4: เขียน `Register`, `GRPCInviteValidator`, handler + `member.yaml`** ตาม Interfaces
- [ ] **Step 5: รันให้ผ่าน** `-race`
- [ ] **Step 6: Commit** — `git commit -m "feat(member): registration with thai/foreign eligibility and member.registered"`

### Task 3: PointLedger append-only + Welcome / Invite point

**Files:**
- Create: `services/member/migrations/00002_points.sql`, `services/member/ledger.go`
- Modify: `services/member/register.go` (ลง hook), `services/member/http.go`, `api/openapi/member.yaml`
- Test: `services/member/ledger_test.go` (integration)

**Interfaces:**
- Consumes: `RegisterTx` hooks จาก Task 2
- Produces:
  - `type Entry struct { ID, MemberID string; DeltaPoints int; Reason, Status, RefKind, RefID, GuideID, Note, Actor string; CreatedAt time.Time }` reason ตาม contracts: `welcome|invite|guide_referral|adjust|cut` · status: `available|pending` · `GuideID` ใส่เฉพาะ `guide_referral` / `cut`
  - `func (l *Ledger) Append(ctx context.Context, tx *sql.Tx, e Entry) (Entry, bool /*created*/, error)` — `INSERT`; ชน `uq_ref` → คืนแถวเดิม `created=false` · ถ้า created เขียน outbox `points.changed` `{memberId, entryId, deltaPoints, reason, status, balance, guideId?}` โดย `balance` = ผลรวม `available` ของ member หลังรายการนี้
  - `func (s *MemberService) Points(ctx, q Querier, key string) int` — อ่าน `point_rules.value` ของ key (`welcome|invite|guide_referral`) ไม่มีแถวใช้ค่า env
  - `func PointRuleHandler(s *MemberService) consumer.Handler` (Name `member-point-rules`, EventTypes `[events.PriceApproved]`) — สนใจเฉพาะ `changes[].entity == "point_rule"` · data `{key: "welcome"|"invite"|"guide_referral", value: int}` (key อื่น เช่น `guide_daily_cap` ข้าม) · `upsert` → upsert `point_rules(key, value, approval_id, updated_at)` · `delete` → ลบแถว (กลับไปใช้ env) · contracts §5 กำหนดให้ data ของ `point_rule` เป็นไปตามแผนนี้ Plan 04 / 14 ต้องส่งรูปแบบนี้
  - `func (l *Ledger) Balance(ctx, q Querier, memberID string) (available, pending int, err error)`
  - Welcome: `{reason: welcome, status: available, refKind: "member", refID: memberId, delta: Points("welcome")}` · Invite (เมื่อ `InviteStatus == "applied"`): `{reason: invite, status: available, refKind: "member", refID: memberId, delta: Points("invite")}`
  - HTTP: `GET /members/me/points?cursor&limit` ได้ `200 {available, pending, redeemEnabled:false, items:[Entry], nextCursor}` (ไม่มี endpoint แลกพ้อย)

```sql
CREATE TABLE point_ledger (
  id CHAR(36) PRIMARY KEY, member_id CHAR(36) NOT NULL, delta_points INT NOT NULL,
  reason VARCHAR(16) NOT NULL, status VARCHAR(10) NOT NULL,
  ref_kind VARCHAR(16) NOT NULL, ref_id CHAR(36) NOT NULL, guide_id CHAR(36) NULL, note VARCHAR(500) NULL, actor VARCHAR(64) NULL,
  created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_ref (ref_kind, ref_id, reason), INDEX idx_member_time (member_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
-- +goose StatementBegin
CREATE TRIGGER point_ledger_no_update BEFORE UPDATE ON point_ledger FOR EACH ROW
  SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'point_ledger is append-only';
-- +goose StatementEnd
-- (trigger point_ledger_no_delete แบบเดียวกันสำหรับ DELETE)
CREATE TABLE point_rules (
  `key` VARCHAR(16) PRIMARY KEY, value INT NOT NULL, approval_id CHAR(36) NOT NULL, updated_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestRegisterGivesWelcomePoint(t *testing.T)        // สมัคร → ledger 1 แถว welcome 100 available, outbox points.changed balance 100
func TestRegisterWithInviteGivesBoth(t *testing.T)       // applied → welcome + invite, balance 150, points.changed 2 ตัว
func TestExistingMemberNoSecondWelcome(t *testing.T)     // สมัครซ้ำ → ledger ยัง 1 แถว
func TestAppendIdempotentByRef(t *testing.T)             // Append ref เดิม 2 ครั้ง → created=false ครั้งที่สอง, outbox ไม่เพิ่ม
func TestLedgerUpdateRejected(t *testing.T)              // UPDATE point_ledger → error SQLSTATE 45000; DELETE → error
func TestPointsAPI(t *testing.T)                         // GET /members/me/points → available 150, pending 0, redeemEnabled false, items เรียงใหม่สุดก่อน
func TestNoRedeemRoute(t *testing.T)                     // POST /members/me/points/redeem → 404
func TestPointRuleApprovedChangesWelcome(t *testing.T)   // price.approved point_rule {key:"welcome", value:200} → สมัครใหม่ได้ 200; op delete → กลับเป็น 100; entity product → ไม่มีผล
```

- [ ] **Step 2: รันให้ fail** — `make test-int PKG=./services/member/... RUN=Ledger|Point|Welcome|Invite`
- [ ] **Step 3: เขียน migration, `Ledger`, `Points`, `PointRuleHandler`, hook welcome / invite, handler** ค่าเริ่มของพ้อยอ่านจาก env ใน `main.go`
- [ ] **Step 4: รันให้ผ่าน**
- [ ] **Step 5: Commit** — `git commit -m "feat(member): append-only point ledger with welcome and invite points"`

### Task 4: พ้อยไกด์ "รอตรวจ" จาก `referral.created` + ตัดพ้อย

**Files:**
- Create: `services/member/guidepoints.go`
- Modify: `services/member/main.go` (ลง consumer), `services/member/http.go`, `api/openapi/member.yaml`
- Test: `services/member/guidepoints_test.go` (integration: DB + NATS)

**Interfaces:**
- Consumes: `consumer.Run`, `Ledger.Append`, payload `referral.created` ตาม contracts §5 (มี `guideMemberId` เพราะไกด์ต้องเป็นสมาชิก, contracts §3)
- Produces:
  - `func ReferralPointsHandler(s *MemberService) consumer.Handler` (Name `member-referral-points`, EventTypes `[events.ReferralCreated]`) — ลงพ้อยให้ `guideMemberId` ตรง ๆ: `Append{memberId: guideMemberId, reason: guide_referral, status: pending, refKind: "referral", refID: referralId, guideId, delta: Points("guide_referral"), note: flags เป็น JSON}` · `guideMemberId` ว่าง → คืน `ErrGuideMemberMissing` (Nak, ครบ MaxDeliver ไป DLQ)
  - `func (s *MemberService) CutReferralPoints(ctx, referralID, reason, actor string) (Entry, error)` — หา entry `guide_referral` ของ referralId (ไม่พบ `ErrReferralNotFound`) แล้ว `Append{reason: cut, status: pending, refKind: "referral", refID: referralId, guideId: เดิม, delta: -original, note: reason, actor}` + `audit_log` action `cut_points` ใน tx เดียว · ตัดซ้ำ → `ErrAlreadyCut`
  - HTTP: `POST /admin/points/cut {referralId, reason}` (`authn.RequireRole("agent_manager","admin")`) ได้ `200 Entry` · `404 referral_not_found`, `409 already_cut`, `400 reason_required` — Admin (Plan 14) ใช้คู่กับคิวตรวจ referral ของ Plan 09 Task 7

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestReferralGivesPendingPoints(t *testing.T)      // referral.created → ledger guide_referral 20 pending ของ guideMemberId, points.changed status pending มี guideId
func TestReferralDuplicateOnce(t *testing.T)           // publish referralId เดิม 2 ครั้ง (คนละ event id) → ledger 1 แถว
func TestReferralWithoutGuideMemberDLQ(t *testing.T)   // guideMemberId ว่าง → handler error ErrGuideMemberMissing, ledger ว่าง
func TestCutPoints(t *testing.T)                       // POST cut → entry -20 reason cut, audit_log 1 แถว, available ไม่เปลี่ยน pending = 0
func TestCutTwice409(t *testing.T)
func TestCutRequiresRole(t *testing.T)                 // X-Roles: viewer → 403
```

- [ ] **Step 2: รันให้ fail** — `make test-int PKG=./services/member/... RUN=Guide|Referral|Cut`
- [ ] **Step 3: เขียน handler + `CutReferralPoints` + route** (consumer ใช้ `AckWait` 30s, `MaxDeliver` 10 ค่าเริ่มของ chassis)
- [ ] **Step 4: รันให้ผ่าน**
- [ ] **Step 5: Commit** — `git commit -m "feat(member): pending guide points from referral.created and point cut"`

### Task 5: Consent + gRPC MemberService + `mid` ใน JWT ของ Identity

**Files:**
- Create: `services/member/consent.go`, `services/member/grpc.go`, `api/proto/member/v1/member.proto`, `services/identity/memberlookup.go`
- Modify: `services/member/main.go` (register gRPC), `services/member/http.go`, `api/openapi/member.yaml`, `services/identity/token.go` (Plan 02), `services/identity/main.go`, `deploy/compose.yaml` (identity env `MEMBER_GRPC_ADDR=member:9090`)
- Test: `services/member/consent_test.go`, `services/member/grpc_test.go` (integration), `services/identity/mid_test.go`

**Interfaces:**
- Produces (member):
  - `member.proto` ตาม contracts §4 ตรงตัว (`GetMemberByContact`, `GetContactPrefs`, `GetCompanion`; `GetCompanion` เขียน server ใน Task 6 ตอนนี้คืน `Unimplemented`)
  - `GetMemberByContact(contact)` normalize ก่อนหา (email lower-case) · ไม่พบหรือ `deleted_at` ไม่ NULL → `codes.NotFound` · `name` = `first_name + " " + last_name`
  - `GetContactPrefs(member_id)` → `{lang, marketing_consent (consent_log ล่าสุดของ kind marketing), phone, email}` · ไม่พบ → `codes.NotFound`
  - `func (s *MemberService) SetMarketingConsent(ctx, memberID string, granted bool, channel, ip string) error` เขียน `consent_log` แถวใหม่เมื่อค่าเปลี่ยนเท่านั้น (version จาก env `CONSENT_VERSION`, ค่าเริ่ม `2027-01`) + outbox `member.consent_changed` `{memberId, marketingConsent, changedAt}` ใน tx เดียว (Report ใช้คำนวณอัตรายินยอม)
  - HTTP: `GET /members/me/consents` ได้ `{terms, pdpa, marketing, history:[{kind, granted, version, createdAt}]}` · `PUT /members/me/consents {marketing}` ได้ `200` · `PATCH /members/me {lang}` ได้ `200 MemberView` และถ้ามีฟิลด์ `firstName|lastName|nationality|idNumber` ได้ `400 field_immutable`
- Produces (identity):
  - `type MemberLookup interface { MemberIDByContact(ctx context.Context, contact string) (string, error) }` · `GRPCMemberLookup` timeout 1 วินาที, `NotFound` → `""`, nil
  - `TokenIssuer` เพิ่มฟิลด์ `Members MemberLookup` · `Issue` เรียก lookup แล้วใส่ claim `mid` (ทั้ง `/auth/verify` และ `/auth/refresh`) · lookup error อื่น → `mid = ""` + log WARN `member_lookup_failed` (ไม่ทำให้ login ล้ม)

- [ ] **Step 1: เขียน test ที่ fail**

```go
// member
func TestWithdrawMarketing(t *testing.T)            // PUT marketing=true แล้ว false → consent_log marketing 3 แถว (สมัคร false, true, false), outbox member.consent_changed 2 แถว, GET → marketing false
func TestSameValueNoNewRow(t *testing.T)            // PUT false ซ้ำ → ไม่เพิ่มแถว ไม่มี member.consent_changed
func TestImmutableFields(t *testing.T)              // PATCH {idNumber} → 400 field_immutable; PATCH {lang:"ja"} → 200
func TestGRPCGetMemberByContact(t *testing.T)       // bufconn: "+66812345678" → member_id, thai_verified true, id_last4 "0708"; ไม่มี → NotFound; "Foo@X.com" หา "foo@x.com" เจอ
func TestGRPCGetContactPrefs(t *testing.T)          // marketing_consent สะท้อนค่าล่าสุด, lang ตาม PATCH
// identity
func TestRefreshAddsMid(t *testing.T)               // fake lookup คืน "m-1" → access token claim mid == "m-1"
func TestVerifyMidEmptyWhenNotMember(t *testing.T)  // fake NotFound → mid == ""
func TestLookupErrorDoesNotFailLogin(t *testing.T)  // fake error → 200, mid == "", log มี member_lookup_failed
```

- [ ] **Step 2: รันให้ fail** — `make test-int PKG=./services/member/... RUN=Consent|GRPC|Immutable` และ `make test PKG=./services/identity/... RUN=Mid`
- [ ] **Step 3: เขียน `member.proto` แล้วรัน `make gen`**
- [ ] **Step 4: เขียน consent + gRPC server + `GRPCMemberLookup` + แก้ `Issue`**
- [ ] **Step 5: รันให้ผ่าน** ทั้ง member และ identity (test ของ Plan 02 ต้องยังผ่าน)
- [ ] **Step 6: Commit** — `git commit -m "feat(member): consent, MemberService grpc; feat(identity): mid claim"`

### Task 6: รายชื่อผู้ติดตาม + gRPC GetCompanion + consumer `account.deleted`

**Files:**
- Create: `services/member/migrations/00003_companions.sql`, `services/member/companion.go`, `services/member/accountdeleted.go`
- Modify: `services/member/grpc.go`, `services/member/http.go`, `services/member/main.go`, `api/openapi/member.yaml`
- Test: `services/member/companion_test.go`, `services/member/accountdeleted_test.go` (integration)

**Interfaces:**
- Consumes: `ParseIdentity`, `IDCipher` (aad `"companion:"+id`)
- Produces:
  - ตาราง `companions(id, member_id, first_name, last_name, nationality, id_kind, id_country, id_enc, id_hmac, id_last4, thai_verified, created_at, updated_at, UNIQUE(member_id, id_hmac))` ลบแบบ hard delete
  - `type CompanionView struct { CompanionID, FirstName, LastName, Nationality, IDKind, IDLast4 string; ThaiVerified bool }`
  - HTTP: `GET /members/me/companions` · `POST /members/me/companions {firstName, lastName, nationality, idNumber, passportCountry?}` ได้ `201` · `PUT /members/me/companions/{id}` · `DELETE /members/me/companions/{id}` ได้ `204` · error `409 companion_exists`, `422 companion_limit` (เกิน 20), `404 companion_not_found` (รวมกรณีเป็นของคนอื่น)
  - gRPC `GetCompanion(member_id, companion_id)` → `Holder{name, nationality, id_last4, thai_verified}` · `companion_id == member_id` คืนข้อมูลของสมาชิกเอง · ไม่ใช่ของ member นี้ → `codes.NotFound`
  - `func AccountDeletedHandler(s *MemberService) consumer.Handler` (Name `member-account-deleted`, EventTypes `[events.AccountDeleted]`, publisher identity ตาม Plan 08 Task 5) — หา member ด้วย `memberId` (ว่าง → หาด้วย `contact`) แล้ว `phone/email/id_enc = NULL`, `first_name/last_name = "-"`, `deleted_at = now`, ลบ `companions` ทั้งหมด + `audit_log` action `account_deleted` · **คง `id_hmac`** (contracts §1) จึงสมัครใหม่ด้วยเลขบัตรเดิมไม่ได้ · คง `point_ledger`, `consent_log`, `purchases`, `visits` (อ้างด้วย memberId) · ไม่พบ member หรือลบแล้ว → no-op
- ผู้ติดตามสัญชาติไทยได้ `thai_verified` จาก check digit เท่านั้น (ไม่มี OTP) ตาม F3 "ราคาไทย / ต่างชาติคิดรายใบ"

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestCompanionCRUD(t *testing.T)               // POST → GET มี 1 รายการ idLast4 ถูก → PUT ชื่อ → DELETE → GET ว่าง
func TestCompanionDuplicateID409(t *testing.T)
func TestCompanionLimit20(t *testing.T)            // คนที่ 21 → 422
func TestOtherMembersCompanion404(t *testing.T)    // member B ลบของ A → 404
func TestGetCompanionSelfAndOther(t *testing.T)    // companion_id == member_id → ชื่อสมาชิก; ของคนอื่น → NotFound
func TestCompanionResponseHasNoFullID(t *testing.T)
// accountdeleted_test.go
func TestAccountDeletedAnonymizes(t *testing.T)        // publish account.deleted → GetMemberByContact เดิม NotFound, phone/email/id_enc NULL, ชื่อ "-", companions ว่าง, ledger ยังอยู่, id_hmac ยังอยู่; ส่งซ้ำ → no-op
func TestReRegisterAfterDeletionRejected(t *testing.T) // สมัครใหม่ด้วยเบอร์เดิม + เลขบัตรเดิม → 409 id_number_taken (ไม่ได้ Welcome point ซ้ำ)
```

- [ ] **Step 2: รันให้ fail** — `make test-int PKG=./services/member/... RUN=Companion|AccountDeleted|ReRegister`
- [ ] **Step 3: เขียน migration + handler + gRPC + `AccountDeletedHandler`** (ลง consumer ใน `main.go`)
- [ ] **Step 4: รันให้ผ่าน**
- [ ] **Step 5: Commit** — `git commit -m "feat(member): companions, GetCompanion and account.deleted consumer"`

### Task 7: ประวัติซื้อ / เข้าชม + ผูกออเดอร์ Guest

**Files:**
- Create: `services/member/migrations/00004_history.sql`, `services/member/history.go`
- Modify: `services/member/register.go` (hook ผูก Guest), `services/member/main.go`, `services/member/http.go`, `api/openapi/member.yaml`
- Test: `services/member/history_test.go` (integration)

**Interfaces:**
- Consumes: payload `order.paid`, `ticket.scanned` ตาม contracts §5, `RegisterTx` hooks
- Produces:
  - ตาราง `purchases(order_id PK, order_no, member_id NULL, guest_contact NULL, paid_amount_satang BIGINT, item_count INT, kinds JSON, visit_dates JSON, paid_at, bound_at NULL, INDEX(member_id, paid_at), INDEX(guest_contact))` และ `visits(ticket_id PK, member_id, visit_date DATE, gate_id, scanned_at, INDEX(member_id, visit_date))`
  - `func OrderPaidHandler(s *MemberService) consumer.Handler` (Name `member-history-order`, `[events.OrderPaid]`) — `memberId` มี → บันทึกกับ member · ไม่มีแต่มี `guestContact` → normalize แล้วหา member ด้วย contact ถ้าเจอให้ผูกเลย (`bound_at = now`) ไม่เจอเก็บ `guest_contact` · `paid_at` = `Envelope.OccurredAt` · `INSERT IGNORE` ด้วย `order_id`
  - `func ScanHandler(s *MemberService) consumer.Handler` (Name `member-history-scan`, `[events.TicketScanned]`) — บันทึกเฉพาะ `result == "ok" && !duplicate && memberId != ""` · `visit_date` = `scannedAt` แปลงเป็นวันที่ Asia/Bangkok · `INSERT IGNORE` ด้วย `ticket_id`
  - `func BindGuestPurchases(ctx, tx *sql.Tx, memberID, contact string, now time.Time) (int64, error)` — ลงเป็น hook ใน `RegisterTx`
  - HTTP: `GET /members/me/history?cursor&limit` ได้ `{visitDays, purchases:{items:[{orderId, orderNo, paidAmountSatang, itemCount, kinds, visitDates, paidAt}], nextCursor}, visits:[{visitDate, gateId}]}` โดย `visitDays` = จำนวน `visit_date` ไม่ซ้ำ (ใช้วัดการกลับมาเที่ยวซ้ำ)

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestOrderPaidMember(t *testing.T)                 // memberId → purchases 1 แถว paidAmountSatang ตรง
func TestOrderPaidDuplicate(t *testing.T)              // event ซ้ำ (event id ต่าง orderId เดิม) → 1 แถว
func TestGuestOrderBoundOnRegister(t *testing.T)       // order.paid guestContact "+66811111111" → สมัครด้วยเบอร์นี้ → GET history มีออเดอร์นั้น
func TestGuestOrderBoundWhenMemberExists(t *testing.T) // member มีอยู่แล้ว + order.paid guestContact "FOO@x.com" → ผูกทันที
func TestScanOkCountsVisit(t *testing.T)               // scannedAt "2027-03-01T18:00:00Z" → visit_date "2027-03-02"
func TestScanDuplicateOrRejectedIgnored(t *testing.T)  // result used / duplicate true / ไม่มี memberId → ไม่บันทึก
func TestVisitDaysDistinct(t *testing.T)               // 2 บัตรวันเดียวกัน + 1 บัตรอีกวัน → visitDays 2
```

- [ ] **Step 2: รันให้ fail** — `make test-int PKG=./services/member/... RUN=Order|Guest|Scan|Visit`
- [ ] **Step 3: เขียน migration, handler 2 ตัว, hook, route**
- [ ] **Step 4: รันให้ผ่าน**
- [ ] **Step 5: Commit** — `git commit -m "feat(member): purchase and visit history with guest order binding"`

### Task 8: กรอก Invite code ภายหลัง + seed

**Files:**
- Create: `services/member/lateinvite.go`, `services/member/seed.go`
- Modify: `services/member/http.go`, `api/openapi/member.yaml`
- Test: `services/member/lateinvite_test.go`, `services/member/seed_test.go` (integration)

**Interfaces:**
- Consumes: `InviteValidator`, `Ledger.Append`, `outbox.Write`
- Produces:
  - `func (s *MemberService) ApplyLateInvite(ctx context.Context, memberID, code, ip, deviceID string) (Entry, error)` — `SELECT … FOR UPDATE` member → มี `invite_code` แล้ว `ErrInviteAlreadySet` → `now > created_at + INVITE_LATE_DAYS วัน` `ErrInviteWindowClosed` → `ValidateInvite` (เรียกก่อน lock เพื่อไม่ถือ lock ระหว่าง gRPC แล้วตรวจซ้ำหลัง lock) → update invite fields + `Append{reason: invite}` + outbox `member.invite_added` `{memberId, inviteCode, guideId, companyId, ip, deviceId, at}` (contracts §5: Agent สร้าง Referral จาก event นี้ ส่วน Report ไม่นับเป็นสมาชิกใหม่) ใน tx เดียว
  - HTTP: `POST /members/me/invite {code}` ได้ `200 {entry, balance}` · `409 invite_already_set`, `410 invite_window_closed`, `422 invite_invalid`, `503 invite_unavailable`
  - `MemberView.inviteDeadline` = `created_at + INVITE_LATE_DAYS` เมื่อยังไม่มีโค้ด
  - `POST /internal/seed` (404 เมื่อ `DEMO_MODE != "true"`) สร้างตามข้อมูลตัวอย่างร่วมใน contracts §7 โดย memberId อ่านจาก `deploy/seed/fixtures.json` (Plan 16 Task 5, ทุก service ใช้ร่วมกัน):

| fixture | contact | ข้อมูล |
|---|---|---|
| สมาชิกคนไทย | `+66811111111` | สมชาย ใจดี, TH, เลขบัตรตาม contracts §7, marketing true |
| สมาชิกจีน | `+8613800138000` | Wang Wei, CN, passport `E12345678` / CN, lang `zh` |
| สมาชิกญี่ปุ่น | `taro@example.jp` | Taro Yamada, JP, passport `TK1234567` / JP, lang `ja` |
| ไกด์ | `+66822222222` | ไกด์เดโม่ (memberId ของไกด์ใน Plan 09 seed), TH |

  ทุกคนได้ Welcome point ผ่าน `Ledger.Append` (ref เดิม → idempotent) และผู้ติดตาม 1 คนของสมาชิกคนไทย · เลขบัตรไทยใน contracts §7 (`1101700230708`) check digit ไม่ผ่าน `thaiid.Valid` (เลขที่ถูกคือ `1101700230708`) seed ต้องใช้ค่าที่ contracts แก้แล้ว ดู "ข้อเสนอแก้ contracts"

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestLateInviteOnce(t *testing.T)            // ครั้งแรก 200 + ledger invite 50 + outbox member.invite_added 1 แถว (ไม่มี member.registered เพิ่ม); ครั้งที่สอง 409 invite_already_set
func TestLateInviteWindow(t *testing.T)          // เลื่อนนาฬิกา 30 วัน 1 วิ → 410
func TestLateInviteInvalid(t *testing.T)         // fake valid=false → 422, ไม่มี ledger
func TestLateInviteUnavailable(t *testing.T)     // fake error → 503
func TestLateInviteConcurrent(t *testing.T)      // 5 goroutine โค้ดเดียวกัน → 200 หนึ่งครั้ง ledger invite 1 แถว
func TestRegisteredWithInviteCannotAddLate(t *testing.T) // สมัครพร้อมโค้ดแล้ว → 409
func TestSeedIdempotent(t *testing.T)            // เรียก 2 ครั้ง → members 4 แถว, ledger 4 แถว, memberId ตรงกับ fixtures.json; DEMO_MODE=false → 404
```

- [ ] **Step 2: รันให้ fail** — `make test-int PKG=./services/member/... RUN=LateInvite|Seed`
- [ ] **Step 3: เขียน `ApplyLateInvite`, route, seed**
- [ ] **Step 4: รันให้ผ่าน** แล้วรันทั้ง service — `make test PKG=./services/member/... && make test-int PKG=./services/member/... && make build SVC=member`
- [ ] **Step 5: ทดสอบบน local:** `make up && make seed` → OTP ด้วย `+66811111111` (ดูรหัสใน Mock Console) → `POST /auth/refresh` ได้ token ที่มี `mid` = memberId ของสมาชิกคนไทยใน `fixtures.json` → `GET /api/member/members/me/points` ได้ available 100
- [ ] **Step 6: Commit** — `git commit -m "feat(member): late invite code and demo seed"`

---

## ข้อเสนอแก้ contracts

รวมเข้า contracts v2 แล้ว (หัวข้อ 1 ลบบัญชีเก็บ `id_hmac`, หัวข้อ 3 ไกด์ต้องเป็นสมาชิก, หัวข้อ 4 `guide_member_id`, หัวข้อ 5 `member.invite_added` / `member.consent_changed` / `guideMemberId` / `point_rule`)

ข้อที่ยังค้าง:

1. **หัวข้อ 7 เลขบัตรประชาชนของสมาชิกคนไทย:** `1101700230708` check digit ไม่ผ่าน (ผลรวมถ่วงน้ำหนัก 146 → check digit ต้องเป็น 8) ขอแก้เป็น `1101700230708` ไม่อย่างนั้น seed สมัครไม่ผ่าน `ParseIdentity`
