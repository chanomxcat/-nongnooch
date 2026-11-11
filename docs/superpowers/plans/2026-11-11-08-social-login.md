# Social Login + WeChat + ขอลบบัญชี Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ผู้ใช้เข้าสู่ระบบด้วย LINE / Google / Apple (OIDC + PKCE) และ WeChat (เว็บสแกน QR, Official Account ในแอป WeChat, Mobile SDK ผูกด้วย UnionID) ได้ บัญชีที่อีเมลตรงกับบัญชีเดิมต้องยืนยัน OTP อีเมลก่อนผูก ผู้ใช้ใหม่ต้องยืนยันเบอร์ / อีเมลก่อน Admin ซ่อนปุ่ม WeChat ได้ และผู้ใช้ขอลบบัญชีได้เอง ระบบลบภายใน 30 วันแล้วแจ้ง service อื่นผ่าน event

**Architecture:** ต่อยอด service `identity` (schema `identity`) จาก Plan 02 ตาราง `social_identities` ผูก `(provider, subject)` เข้ากับ contact (E.164 / อีเมล) ซึ่งเป็น `sub` ของ JWT ตาม contracts §3 การออก token ใช้ `TokenIssuer.Issue` เดิม (มี `mid` จาก Plan 03 Task 5) ทุก IdP เรียกผ่าน interface + URL ใน env ซึ่งใน demo ชี้ไป mockext (Plan 00 Task 3) งานลบบัญชีเป็น job ใน identity ที่ publish `account.deleted` (contracts §5) ผ่าน outbox ส่วน consumer อยู่ในแผนของ service นั้น (member: Plan 03 Task 6, agent: Plan 09, notify: Plan 10, report: Plan 11)

**Tech Stack:** Go 1.23 + chassis (Plan 01), `github.com/golang-jwt/jwt/v5` (ตรวจ id_token), `crypto/sha256` (PKCE S256), `crypto/rand`, `net/http`

**Spec:** [docs/system-design.md](../../system-design.md) S2, F1 (กิ่ง Social login), หัวข้อ 2 (Social login), Q7, Q24 · [ROADMAP](../../../roadmap/ROADMAP.md) E5 "ลบบัญชีได้เอง", E9, หัวข้อ 12 PDPA · [contracts](2026-11-11-contracts.md) · ขึ้นกับ [Plan 02](2026-11-11-02-identity-otp.md), [Plan 03](2026-11-11-03-member-point.md) Task 5, [Plan 00](2026-11-11-00-mock-external.md) Task 3

## Global Constraints

- **Build และ test ใน Docker เท่านั้น:** `make test`, `make test-int`, `make build`, `make gen`, `make seed` ห้ามรัน `go` / `npm` / `flutter` บนเครื่องโดยตรง integration test ขึ้นต้น `//go:build integration`
- **โหมด Demo:** IdP ทุกตัวใช้ mockext `/oauth/{p}` และ `/oauth/wechat` (Plan 00 Task 3) อีเมล OTP ไป Mailpit เรียกผ่าน interface + base URL ใน env ห้าม hardcode
- **ใช้ชื่อ / type ตาม [contracts](2026-11-11-contracts.md) ห้ามแก้** ถ้าต้องแก้ให้เสนอในหัวข้อ "ข้อเสนอแก้ contracts" ท้ายแผน
- **Seed:** `POST /internal/seed` ของ identity ทำงานเมื่อ `DEMO_MODE=true` เท่านั้นและรันซ้ำได้ (Task 1)
- ข้อกำหนดทั้งหมดใน Global Constraints ของ Plan 01 และ Plan 02
- S2: "OAuth 2.0 / OIDC (PKCE) · WeChat: เว็บสแกน QR · ในแอป WeChat ใช้ Official Account · แอปใช้ Mobile SDK"
- S2: "WeChat ให้ UnionID ไม่มีอีเมล / เบอร์" · "อีเมลตรงกับบัญชีที่มีอยู่ → ขอยืนยัน OTP อีเมลก่อนผูกบัญชี" · ผู้ใช้ใหม่ "ขอเบอร์โทร / อีเมล + OTP … ต้องยืนยันตัวตนก่อนได้พ้อย"
- S2: "Build HMS (Huawei) ไม่มี Google Sign-In ใช้ LINE / WeChat / Apple / OTP แทน" · "ปุ่ม WeChat ซ่อนได้จาก Admin ถ้าบัญชี Open Platform ยังไม่ผ่าน"
- PDPA: "Social login ขอเฉพาะข้อมูลที่จำเป็น (ชื่อ อีเมล)" · E5: "ขอลบบัญชีใน App และเว็บ ดำเนินการภายใน 30 วัน (เงื่อนไขของ Apple / Google และ PDPA)"
- JWT `typ=user`, `sub` = E.164 หรืออีเมล, `ctype`, `mid`, `lang` ตาม contracts §3 (social login ไม่เปลี่ยนความหมายของ `sub`)
- ลบบัญชีตาม contracts §1: ลบชื่อ, ช่องทางติดต่อ และเลขบัตรที่เข้ารหัส แต่**เก็บ `id_hmac`** (ทำฝั่ง member, Plan 03 Task 6) identity ห้ามขอให้ลบ `id_hmac`

**ค่าที่แผนนี้กำหนดเพิ่ม (ให้ PO ยืนยันใน review):** `state` / link token อายุ 10 นาทีใช้ครั้งเดียว, 1 contact ผูกได้ 1 บัญชีต่อ provider, ลบบัญชีครบกำหนด 30 วันหลังขอ ยกเลิกได้ก่อนครบ, job ลบรันทุก 1 ชั่วโมง

**ต่างจาก spec:** S2 ระบุ `POST /auth/social {provider, code}` แผนนี้เพิ่ม `state` (และ `mode` สำหรับ WeChat) เพราะ server เป็นผู้ถือ `code_verifier` / `nonce` ของ PKCE · S2 ระบุ `POST /auth/social/link {otp}` แผนนี้ใช้ `{linkToken, requestId, otp}` ตามเหตุผลเดียวกับ Plan 02

## Review Focus

1. **`state` ซ้ำ / หมดอายุ / ของ provider อื่น** (CSRF, replay) ต้องได้ `400 invalid_state` และไม่เรียก token endpoint (Task 2)
2. **id_token ปลอมหรือผิด `aud` / `iss` / `nonce` / หมดอายุ** ต้องได้ `401 id_token_invalid` (Task 2)
3. **อีเมลจาก IdP ที่ยังไม่ verified** ตรงกับบัญชีเดิม ต้องไม่เข้าเส้นทาง "ผูกด้วย OTP อีเมลเดิม" แต่เป็น `contact_required` (กันยึดบัญชี) และ OTP ที่ใช้ผูกต้องเป็นของอีเมลนั้นจริง (Task 3)
4. **WeChat ตอบไม่มี `unionid`** ต้องได้ `400 wechat_unionid_missing` ห้ามใช้ `openid` แทนเงียบ ๆ เพราะ openid ต่างกันตาม appid (Task 4)
5. **job ลบบัญชีรัน 2 replica หรือรันซ้ำ** ต้องได้ `account.deleted` 1 ครั้งต่อคำขอ (Task 5)

---

## File Structure

```
services/identity/migrations/00002_social.sql      social_identities, social_states, social_pending, provider_settings
services/identity/migrations/00003_deletion.sql    deletion_requests
services/identity/social_providers.go   GET /auth/social/providers + admin toggle + seed
services/identity/oidc.go               OIDCProvider: authorize URL, token exchange, ตรวจ id_token
services/identity/social.go             SocialService: start / callback / link
services/identity/wechat.go             WeChatProvider (web / oa / app)
services/identity/deletion.go           ขอ / ยกเลิก / job ลบบัญชี
services/identity/http.go               (แก้) route
api/openapi/identity.yaml               (แก้)
```

---

### Task 1: Schema + รายการ provider ตามแพลตฟอร์ม + Admin ซ่อนปุ่ม

**Files:**
- Create: `services/identity/migrations/00002_social.sql`, `services/identity/social_providers.go`
- Modify: `services/identity/http.go`, `services/identity/main.go`, `api/openapi/identity.yaml`
- Test: `services/identity/social_providers_test.go` (integration)

**Interfaces:**
- Produces:
  - `type ProviderSetting struct { Provider string; Enabled bool; UpdatedBy string; UpdatedAt time.Time }` provider ∈ `line google apple wechat`
  - `func (s *SocialService) Providers(ctx, platform string) ([]string, error)` platform ∈ `web ios gms hms wechat` · ตัด `google` เมื่อ `hms` · ตัด provider ที่ `enabled=false` หรือไม่มี client id ใน env · `wechat` platform (เบราว์เซอร์ในแอป WeChat) คืน `["wechat"]` เท่านั้น
  - HTTP: `GET /auth/social/providers?platform=` ได้ `200 {"providers":["line","wechat","apple"]}` (ลำดับคงที่ `line wechat google apple`) · `PUT /admin/social/providers/{provider} {enabled}` (`authn.RequireRole("admin")`) ได้ `200 ProviderSetting` · `POST /internal/seed` (เฉพาะ `DEMO_MODE=true`) upsert ทุก provider `enabled=true`

```sql
CREATE TABLE social_identities (
  id CHAR(36) PRIMARY KEY, provider VARCHAR(10) NOT NULL, subject VARCHAR(128) NOT NULL, -- sub หรือ WeChat unionid
  contact VARCHAR(254) NOT NULL, email VARCHAR(254) NULL, created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_provider_subject (provider, subject), UNIQUE KEY uq_provider_contact (provider, contact)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE social_states (
  state CHAR(43) PRIMARY KEY, provider VARCHAR(10) NOT NULL, mode VARCHAR(4) NULL,
  code_verifier CHAR(43) NULL, nonce CHAR(43) NULL, redirect_uri VARCHAR(500) NOT NULL,
  expires_at DATETIME(6) NOT NULL, used_at DATETIME(6) NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE social_pending (
  token_hash BINARY(32) PRIMARY KEY, provider VARCHAR(10) NOT NULL, subject VARCHAR(128) NOT NULL,
  email VARCHAR(254) NULL, link_contact VARCHAR(254) NULL, -- ตั้งเมื่อ link_required
  expires_at DATETIME(6) NOT NULL, used_at DATETIME(6) NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE provider_settings (
  provider VARCHAR(10) PRIMARY KEY, enabled BOOLEAN NOT NULL, updated_by VARCHAR(64) NOT NULL, updated_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestProvidersHMSHasNoGoogle(t *testing.T)     // platform=hms → ["line","wechat","apple"]
func TestProvidersWebAll(t *testing.T)             // web → ["line","wechat","google","apple"]
func TestAdminHidesWeChat(t *testing.T)            // PUT wechat enabled=false (X-Roles: admin) → web ไม่มี "wechat"
func TestToggleRequiresAdmin(t *testing.T)         // X-Roles: editor → 403
func TestInsideWeChatOnlyWeChat(t *testing.T)      // platform=wechat → ["wechat"]
func TestSeedIdempotent(t *testing.T)              // เรียก 2 ครั้ง → provider_settings 4 แถว
```

- [ ] **Step 2: รันให้ fail** — `make test-int PKG=./services/identity/... RUN=Providers|Toggle|Seed|InsideWeChat`
- [ ] **Step 3: เขียน migration, `Providers`, route**
- [ ] **Step 4: รันให้ผ่าน**
- [ ] **Step 5: Commit** — `git commit -m "feat(identity): social schema, provider list per platform, admin toggle"`

### Task 2: OIDC + PKCE (LINE / Google / Apple)

**Files:**
- Create: `services/identity/oidc.go`, `services/identity/social.go`
- Modify: `services/identity/http.go`, `deploy/compose.yaml` (env ด้านล่าง), `api/openapi/identity.yaml`
- Test: `services/identity/oidc_test.go` (unit, `httptest` IdP จำลองที่มี JWKS), `services/identity/social_start_test.go` (integration)

**Interfaces:**
- Produces:
  - `type SocialProfile struct { Provider, Subject, Email string; EmailVerified bool; Name string }`
  - `type IdP interface { AuthorizeURL(state, nonce, codeChallenge, redirectURI string) string; Exchange(ctx context.Context, code, codeVerifier, redirectURI, nonce string) (SocialProfile, error) }`
  - `OIDCProvider{Name, Issuer, BackchannelURL, ClientID, ClientSecret string; Scopes []string; Client *http.Client}` — authorize = `Issuer + "/authorize"` (URL ที่เบราว์เซอร์เปิด) · token = `BackchannelURL + "/token"` · JWKS = `BackchannelURL + "/jwks"` (cache 10 นาที) · ตรวจ id_token: ลายเซ็นตาม `kid`, `iss == Issuer`, `aud == ClientID`, `nonce`, `exp` (leeway 60 วิ) · scope `openid email profile` · error `ErrIDTokenInvalid`, `ErrIdPUnavailable` (timeout 5 วิ / 5xx)
  - env ต่อ provider `P` ∈ `LINE GOOGLE APPLE`: `OIDC_P_ISSUER` (demo `${MOCKEXT_PUBLIC_URL}/oauth/{p}` = `http://localhost:8090/oauth/{p}` ตาม contracts §2), `OIDC_P_BACKCHANNEL_URL` (demo `http://mockext:8090/oauth/{p}`, ว่าง = ใช้ Issuer), `OIDC_P_CLIENT_ID`, `OIDC_P_CLIENT_SECRET` · `SOCIAL_REDIRECT_URIS` (allow-list คั่นด้วย `,`)
  - `func (s *SocialService) Start(ctx, provider, mode, redirectURI string) (authorizeURL, state string, err error)` — สร้าง `state`, `nonce`, `code_verifier` (32 byte random, b64url) เก็บใน `social_states` อายุ 10 นาที · `code_challenge = b64url(sha256(verifier))`
  - `func (s *SocialService) consumeState(ctx, tx, provider, state string) (stateRow, error)` — `SELECT … FOR UPDATE` ต้องตรง provider, ยังไม่ใช้, ไม่หมดอายุ แล้ว set `used_at` · error `ErrInvalidState`
  - HTTP: `POST /auth/social/start {provider, mode?, redirectUri}` ได้ `200 {authorizeUrl, state}` · error `400 provider_disabled|redirect_not_allowed`
- Apple: ในโหมดจริงใช้ `response_mode=form_post` และ client secret เป็น JWT (ทำเป็น `ClientSecretFunc` ภายหลัง) ใน demo ใช้ secret แบบ string ตาม mockext

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestStartBuildsPKCEURL(t *testing.T)          // authorizeUrl มี client_id, code_challenge_method=S256, state, nonce; challenge == b64url(sha256(verifier ในตาราง))
func TestRedirectNotAllowed(t *testing.T)          // redirectUri นอก allow-list → 400
func TestExchangeValidIDToken(t *testing.T)        // IdP จำลองคืน id_token ถูกต้อง → SocialProfile{Subject, Email, EmailVerified}
func TestExchangeWrongAud(t *testing.T)            // aud ต่าง → ErrIDTokenInvalid
func TestExchangeWrongNonce(t *testing.T)
func TestExchangeExpired(t *testing.T)             // exp เลยไป 2 นาที → ErrIDTokenInvalid
func TestExchangeBadSignature(t *testing.T)        // ลงด้วย key อื่น → ErrIDTokenInvalid
func TestIdPTimeout(t *testing.T)                  // token endpoint หน่วง 6 วิ → ErrIdPUnavailable
func TestStateSingleUse(t *testing.T)              // consumeState 2 ครั้ง → ครั้งที่สอง ErrInvalidState; provider อื่น → ErrInvalidState; เลย 10 นาที → ErrInvalidState
```

- [ ] **Step 2: รันให้ fail** — `make test PKG=./services/identity/... RUN=Exchange|IdP` และ `make test-int PKG=./services/identity/... RUN=Start|State`
- [ ] **Step 3: เขียน `OIDCProvider`, `Start`, `consumeState`**
- [ ] **Step 4: รันให้ผ่าน**
- [ ] **Step 5: Commit** — `git commit -m "feat(identity): oidc pkce client for line/google/apple"`

### Task 3: Callback + ผูกบัญชี (`POST /auth/social`, `POST /auth/social/link`)

**Files:**
- Modify: `services/identity/social.go`, `services/identity/otp.go` (Plan 02), `services/identity/http.go`, `api/openapi/identity.yaml`
- Test: `services/identity/social_link_test.go` (integration, fake `IdP` + fake `MemberLookup`)

**Interfaces:**
- Consumes: `IdP.Exchange`, `consumeState`, `OTPService.Verify`, `TokenIssuer.Issue`, `MemberLookup` (Plan 03 Task 5), `NormalizeContact`
- Produces:
  - `func (s *OTPService) RequestTrusted(ctx, c Contact, lang, ip string) (requestID string, err error)` — เหมือน `Request` แต่ไม่ตรวจ Turnstile (ใช้ rate limit เดิม) ใช้เฉพาะภายใน identity
  - `type SocialResult struct { Status string /* "ok"|"link_required"|"contact_required" */; Tokens *Tokens; LinkToken, RequestID, MaskedEmail string }`
  - `func (s *SocialService) Callback(ctx, provider, mode, code, state, lang, ip string) (SocialResult, error)`:
    1. `consumeState` → `Exchange`
    2. พบ `social_identities(provider, subject)` → `Issue(Contact)` → `ok`
    3. `EmailVerified` และ `MemberLookup(email)` เจอสมาชิก → สร้าง `social_pending` (`link_contact = email`) + `RequestTrusted(email)` → `link_required` พร้อม `maskedEmail` (`s***@gmail.com`)
    4. อื่น ๆ → สร้าง `social_pending` (`link_contact = NULL`) → `contact_required` (frontend ทำ `/auth/otp` ของ Plan 02 ต่อ)
  - `func (s *SocialService) Link(ctx, linkToken, requestID, otp string) (Tokens, error)` — หา pending (hash, ยังไม่ใช้, ไม่หมดอายุ: ไม่งั้น `ErrLinkTokenInvalid`) → `OTPService.Verify` → ถ้า `link_contact` ไม่ว่างต้องตรงกับ contact ที่ verify (`ErrLinkContactMismatch`) → insert `social_identities` (ชน `uq_provider_contact` → `ErrProviderAlreadyLinked`) → mark used → `Issue`
  - HTTP: `POST /auth/social {provider, mode?, code, state}` ได้ `200 SocialResult` · `400 invalid_state`, `401 id_token_invalid`, `502 idp_unavailable` · `POST /auth/social/link {linkToken, requestId, otp}` ได้ `200 Tokens` · `400 link_token_invalid`, `403 link_contact_mismatch`, `409 provider_already_linked` + error OTP ของ Plan 02
  - linkToken = random 32 byte b64url เก็บเฉพาะ SHA-256 อายุ 10 นาที

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestLinkedIdentityLogsIn(t *testing.T)            // มีแถว social_identities → ok, access token sub == contact เดิม, mid จาก fake lookup
func TestVerifiedEmailOfMemberNeedsOTP(t *testing.T)   // email verified + member มีอยู่ → link_required, otp_requests 1 แถว kind email, outbox otp.requested
func TestLinkWithEmailOTP(t *testing.T)                // Link ด้วย OTP ที่ถูก → Tokens sub == email, social_identities 1 แถว; ครั้งต่อไป Callback → ok
func TestUnverifiedEmailGoesContactRequired(t *testing.T) // EmailVerified false + member มีอยู่ → contact_required, ไม่มี otp_requests
func TestNewUserLinksToPhone(t *testing.T)             // contact_required → /auth/otp เบอร์ → Link(requestId ของเบอร์) → sub == "+66…"
func TestLinkContactMismatch(t *testing.T)             // link_required ของ a@x.com แต่ Link ด้วย requestId ของเบอร์อื่น → 403
func TestLinkTokenSingleUse(t *testing.T)              // Link สำเร็จแล้วใช้ซ้ำ → 400 link_token_invalid
func TestProviderAlreadyLinked(t *testing.T)           // contact ผูก LINE บัญชีหนึ่งแล้ว LINE อีกบัญชีผูก contact เดิม → 409
```

- [ ] **Step 2: รันให้ fail** — `make test-int PKG=./services/identity/... RUN=Link|Linked|Email|NewUser`
- [ ] **Step 3: เขียน `RequestTrusted`, `Callback`, `Link`, handler**
- [ ] **Step 4: รันให้ผ่าน** `-race` (test ของ Plan 02 ต้องยังผ่าน)
- [ ] **Step 5: ทดสอบกับ mockext:** `make up` → `POST /api/identity/auth/social/start {provider:"line", redirectUri:"http://localhost:3000/th/auth/callback"}` → เปิด `authorizeUrl` เลือกผู้ใช้ทดสอบ → นำ `code` + `state` ไป `POST /auth/social` ได้ `contact_required` → ทำ OTP เบอร์ → Link ได้ token → login LINE ซ้ำได้ `ok`
- [ ] **Step 6: Commit** — `git commit -m "feat(identity): social callback and otp-verified account linking"`

### Task 4: WeChat (เว็บ QR / Official Account / Mobile) ผูกด้วย UnionID

**Files:**
- Create: `services/identity/wechat.go`
- Modify: `services/identity/social.go`, `services/identity/main.go`, `deploy/compose.yaml`, `api/openapi/identity.yaml`
- Test: `services/identity/wechat_test.go` (unit, `httptest`), `services/identity/wechat_flow_test.go` (integration)

**Interfaces:**
- Produces:
  - `WeChatProvider{BaseURL, PublicURL string; Apps map[string]WeChatApp; Client *http.Client}` · `WeChatApp{AppID, Secret string}` key = mode `web|oa|app` จาก env `WECHAT_WEB_APPID/SECRET`, `WECHAT_OA_APPID/SECRET`, `WECHAT_APP_APPID/SECRET` · `WECHAT_BASE_URL` (server เรียก, demo `http://mockext:8090/oauth/wechat`) · `WECHAT_PUBLIC_URL` (เบราว์เซอร์เปิด, demo `${MOCKEXT_PUBLIC_URL}/oauth/wechat`)
  - implement `IdP` แบบรับ `mode`: `AuthorizeURLFor(mode, state, redirectURI string) string` — `web` → `PublicURL + "/connect/qrconnect?appid&redirect_uri&response_type=code&scope=snsapi_login&state"` · `oa` → `PublicURL + "/connect/oauth2/authorize?…&scope=snsapi_userinfo"` · `app` → `""` (แอปเรียก WeChat SDK เอง ใน demo แอปเปิด URL แบบ `web` ของ mockext แทน)
  - `ExchangeWeChat(ctx, mode, code string) (SocialProfile, error)` — `GET BaseURL + "/sns/oauth2/access_token?appid&secret&code&grant_type=authorization_code"` → `Subject = unionid` (ว่าง → `ErrWeChatUnionIDMissing`), `Email = ""` · WeChat ไม่มี PKCE จึงใช้ `state` อย่างเดียว (`code_verifier`, `nonce` เป็น NULL)
  - `Callback` เมื่อ `provider == "wechat"` ต้องมี `mode` ตรงกับที่บันทึกใน `social_states` และผลเป็น `ok` หรือ `contact_required` เท่านั้น (ไม่มีอีเมล)
  - HTTP error เพิ่ม `400 wechat_unionid_missing`, `400 invalid_mode`

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestWeChatWebAuthorizeURL(t *testing.T)        // mode web → path /connect/qrconnect, scope snsapi_login, appid = WECHAT_WEB_APPID
func TestWeChatOAAuthorizeURL(t *testing.T)         // mode oa → /connect/oauth2/authorize, appid = WECHAT_OA_APPID
func TestWeChatExchangeUsesUnionID(t *testing.T)    // httptest คืน openid "o1", unionid "u1" → Subject "u1", Email ""
func TestWeChatMissingUnionID(t *testing.T)         // ไม่มี unionid → ErrWeChatUnionIDMissing
func TestSameUserAcrossModes(t *testing.T)          // integration: ผูกผ่าน web แล้ว login ผ่าน oa (openid ต่าง unionid เดียว) → ok sub เดิม
func TestWeChatNewUserContactRequired(t *testing.T) // ยังไม่ผูก → contact_required ไม่มี link_required
func TestWeChatModeMismatch(t *testing.T)           // start mode web แต่ callback mode oa → 400 invalid_state
func TestWeChatDisabledByAdmin(t *testing.T)        // provider_settings wechat false → start ได้ 400 provider_disabled
```

- [ ] **Step 2: รันให้ fail** — `make test PKG=./services/identity/... RUN=WeChat` และ `make test-int PKG=./services/identity/... RUN=WeChat|SameUser`
- [ ] **Step 3: เขียน `WeChatProvider` + แก้ `Start` / `Callback`** ให้เลือกตาม provider
- [ ] **Step 4: รันให้ผ่าน**
- [ ] **Step 5: ทดสอบกับ mockext:** start `wechat` mode `web` → หน้า "สแกน QR" จำลอง → เลือกผู้ใช้ → Link กับเบอร์ → start mode `oa` ด้วยผู้ใช้เดิม ได้ `ok`
- [ ] **Step 6: Commit** — `git commit -m "feat(identity): wechat login via unionid for web, oa and app"`

### Task 5: ขอลบบัญชี + job 30 วัน + `account.deleted`

**Files:**
- Create: `services/identity/migrations/00003_deletion.sql`, `services/identity/deletion.go`
- Modify: `services/identity/http.go`, `services/identity/main.go`, `api/openapi/identity.yaml`
- Test: `services/identity/deletion_test.go` (integration)

**Interfaces:**
- Consumes: `MemberLookup`, `outbox.Write`, `events.AccountDeleted` (Plan 01, contracts §5), การ revoke refresh token ทั้งหมดของ subject (Plan 02 Task 4)
- Produces:
  - `deletion_requests(id CHAR(36) PK, contact VARCHAR(254), member_id CHAR(36) NULL, status VARCHAR(10) /* pending|cancelled|done */, requested_at, due_at, finished_at NULL, INDEX(status, due_at), INDEX(contact))`
  - HTTP (`authn.Require("user")`, contact = `X-User-Sub`): `POST /account/deletion` ได้ `202 {requestId, dueAt}` (มีคำขอ pending อยู่แล้ว → `200` คำขอเดิม) และ revoke refresh token ทุกตัวของ contact ทันที · `GET /account/deletion` ได้ `200 {status, dueAt}` หรือ `404` · `DELETE /account/deletion` ได้ `204` (pending → cancelled) · `409 deletion_not_pending`
  - `type DeletionJob struct { DB *sql.DB; Members MemberLookup; Now func() time.Time; Interval time.Duration /* 1h */ }` · `func (j *DeletionJob) Tick(ctx) (int, error)` — `SELECT … WHERE status='pending' AND due_at <= now FOR UPDATE SKIP LOCKED LIMIT 100` → ลบ `social_identities` + `otp_requests` + `refresh_tokens` ของ contact → outbox `account.deleted` `{memberId?, contact, deletedAt}` → `status=done` ใน tx เดียว
  - consumer ของ `account.deleted` อยู่ในแผนของแต่ละ service: member ลบชื่อ / contact / เลขบัตรที่เข้ารหัส แต่**เก็บ `id_hmac`** (Plan 03 Task 6), agent ระงับไกด์ (Plan 09), notify ลบ Device (Plan 10), report (Plan 11)

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestRequestDeletionRevokesTokens(t *testing.T)   // 202, dueAt = now+30 วัน, refresh เดิม → 401
func TestRequestTwiceReturnsSame(t *testing.T)
func TestCancelBeforeDue(t *testing.T)                // DELETE → 204, Tick หลัง 31 วัน → 0
func TestTickBeforeDueNoop(t *testing.T)              // เลื่อนนาฬิกา 29 วัน → Tick == 0
func TestTickDeletesAndPublishes(t *testing.T)        // 30 วัน 1 วิ → Tick == 1, social_identities ของ contact ว่าง, outbox account.deleted 1 แถว payload {memberId, contact, deletedAt}
func TestTwoJobsOneEvent(t *testing.T)                // 2 DeletionJob Tick พร้อมกัน → outbox account.deleted 1 แถว
func TestTickNonMemberNoMemberID(t *testing.T)        // contact ที่ยังไม่สมัครสมาชิก → payload ไม่มี memberId
```

- [ ] **Step 2: รันให้ fail** — `make test-int PKG=./services/identity/... RUN=Deletion|Tick|Cancel`
- [ ] **Step 3: เขียน migration, handler, `DeletionJob` (เริ่มใน `main.go` คู่กับ Relay)**
- [ ] **Step 4: รันให้ผ่าน** แล้ว `make test && make test-int PKG=./services/identity/... && make build SVC=identity` (ถ้า Plan 03 Task 6 merge แล้ว ให้รัน `make test-int PKG=./services/member/... RUN=AccountDeleted|ReRegister` ด้วย)
- [ ] **Step 5: Commit** — `git commit -m "feat(identity): account deletion request, 30-day job, account.deleted"`

---

## ข้อเสนอแก้ contracts

รวมเข้า contracts v2 แล้ว (หัวข้อ 5 `account.deleted`, หัวข้อ 1 ลบบัญชีเก็บ `id_hmac`)
