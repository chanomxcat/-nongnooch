# Phase 1 Shared Contracts (v2)

ทุกแผน implementation ของเฟส 1 (Plan 00–16) **ต้องใช้ชื่อ, type และรูปแบบตามไฟล์นี้** ถ้าแผนใดต้องเปลี่ยนสัญญา ห้ามแก้ในแผนนั้นเอง ให้เสนอใน PR ที่แก้ไฟล์นี้ก่อน

**Spec:** [system-design.md](../../system-design.md) · [DESIGN.md](../../design/DESIGN.md) · [ดัชนีแผน](2026-11-11-phase1-index.md)

> **v2 (11 พ.ย. 2026)** รวมข้อเสนอจากแผน 03–16 ดูรายการเปลี่ยนแปลงทั้งหมดในหัวข้อ 11 ท้ายไฟล์

## 1. กติการ่วม

- **Build / test ใน Docker เท่านั้น** ผ่าน make target ห้ามรัน `go`, `npm`, `flutter` บนเครื่องโดยตรง

  | Target | ใช้กับ |
  |---|---|
  | `make up` / `make down` | เปิด / ปิด stack ใน compose |
  | `make test PKG= RUN=` · `make test-int PKG=` | Go unit / integration (`golang:1.23-bookworm`) |
  | `make build SVC= [REGISTRY= TAG=]` | image ของ Go service (ค่าเริ่ม `nongnooch/<svc>:dev`) |
  | `make gen` | proto + OpenAPI → `gen/{go,ts,dart}` |
  | `make fe-install\|fe-lint\|fe-test\|fe-build APP=web\|admin\|gate` | Next.js (`node:20-bookworm`) |
  | `make e2e APP=web\|admin\|gate` | Playwright (`mcr.microsoft.com/playwright:v1.48.2-jammy`) ต่อกับ stack ใน compose |
  | `make build-web APP=` | image ของแอปเว็บ |
  | `make mobile-test` · `make mobile-build FLAVOR=tourist\|agent STORE=gms\|hms` | Flutter (`ghcr.io/cirruslabs/flutter:3.24.0`) ส่วน iOS build ต้องใช้ Mac (นอก Docker) |
  | `make seed` · `make demo-reset CONFIRM=yes` | ข้อมูลตัวอย่าง (หัวข้อ 7) |

- **Demo:** ระบบภายนอกทุกตัวใช้ mockext / Mailpit ตาม [Plan 00](2026-11-11-00-mock-external.md) และเรียกผ่าน interface + base URL ใน env
- **7 ภาษา:** `th en zh ja ko de fr` (ZH = ตัวย่อ) ถ้าไม่มีข้อความในภาษานั้นให้ใช้ `en`
- **เงิน:** `int64` หน่วยสตางค์ (THB × 100) field ชื่อลงท้าย `Satang` เช่น `amountSatang`
- **เวลา:** UTC RFC3339 (`2027-03-01T02:00:00Z`) ส่วน**วันเข้าชม**เป็น `YYYY-MM-DD` ตามเวลา Asia/Bangkok field ชื่อ `visitDate`
- **ID:** UUIDv7 string ทุกตัว (`events.New` / `uuid.NewV7`) ยกเว้นเลขที่คนอ่าน เช่น `orderNo` = `NN` + `YYMMDD` + 6 หลัก
- **ช่องทางติดต่อ (`contact`, `guestContact`, `toContact`):** E.164 (`+66812345678`) หรืออีเมลตัวพิมพ์เล็ก
- **JSON:** camelCase · error `{"error":"snake_case_code","message":"ข้อความสำหรับ log"}` (ข้อความที่แสดงให้ผู้ใช้แปลฝั่ง frontend จาก `error`) · list ใช้ `?cursor=&limit=` (ค่าเริ่ม 20 สูงสุด 100) ตอบ `{"items":[...],"nextCursor":"..."}`
- **เลขบัตรประชาชน / พาสปอร์ต:** ส่งข้าม service ได้แค่ `idLast4` และ `nationality` ห้ามส่งเลขเต็มใน event, log หรือ response (ยกเว้น API ถอดรหัสของ Member ซึ่งเฉพาะบทบาท `admin`)
- **ลบบัญชี (มติ 11 พ.ย. 2026):** ลบชื่อ, ช่องทางติดต่อ และเลขบัตรที่เข้ารหัสไว้ แต่**เก็บ `id_hmac` ไว้** เพื่อกันการลบแล้วสมัครใหม่รับ welcome point ซ้ำ
- **Error code ของ Member ที่ frontend ใช้ร่วม:** `409 id_number_taken` (1 เลขบัตร = 1 บัญชี), `422 id_number_invalid`, `422 invite_invalid`, `409 invite_already_set`, `410 invite_window_closed`, `503 invite_unavailable`, `409 companion_exists`, `422 companion_limit` (ตาม Plan 03)
- **Audit log:** แต่ละ service เก็บของตัวเองและเปิด `GET /admin/audit?entity=&entityId=&cursor=` ในรูปแบบเดียวกัน `{id, at, actorId, actorRoles, action, entity, entityId, before, after, reason?}`

## 2. Repo และ service

Monorepo คือ repo นี้ ([github.com/chanomxcat/-nongnooch](https://github.com/chanomxcat/-nongnooch)) โค้ดอยู่ที่ root คู่กับ `docs/` และ `roadmap/` (Go module `nongnooch/platform`)

```
pkg/chassis/*            Plan 01 (+ authn, queuetoken จาก Plan 12 / 06)
services/<svc>/          Go service ละโฟลเดอร์ (migrations/ ภายใน)
api/openapi/<svc>.yaml   OpenAPI 3.1 ภายนอก
api/proto/<svc>/v1/*.proto
gen/{go,ts,dart}/        จาก make gen (commit ลง repo) · Dart: gen/dart/<svc> package `nongnooch_<svc>_api`
apps/web/                Next.js เว็บนักท่องเที่ยว + หน้า Agent (Plan 13)
apps/admin/              Next.js Admin / CMS / Dashboard (Plan 14)
apps/gate/               Next.js PWA Gate Scanner (Plan 14)
apps/mobile/             Flutter Tourist + Agent (Plan 15)
packages/tokens/         design tokens: tokens.json → CSS + Dart `packages/tokens/dart/lib/nn_tokens.dart` (Plan 13 Task 1, WBS 1.11.10)
deploy/                  compose.yaml, Dockerfile.go, Dockerfile.web, seed/, caddy/
```

| Service (compose) | Schema | HTTP / gRPC ในเครือข่าย | พอร์ตบนเครื่อง | แผน |
|---|---|---|---|---|
| gateway | – | 8080 | **8000** | 12 |
| identity | `identity` | 8080 / 9090 | – | 02, 04 (staff), 08 |
| member | `member` | 8080 / 9090 | – | 03 |
| agent | `agent` | 8080 / 9090 | – | 09 |
| commerce | `commerce` | 8080 / 9090 | – | 05, 06 |
| ticket | `ticket` | 8080 / 9090 | – | 07 |
| content | `content` | 8080 / 9090 | – | 04 |
| notify | `notify` | 8080 / 9090 | – | 02, 10 |
| report | `report` | 8080 / 9090 | – | 11 |
| mockext | – | 8090 | 8090 | 00 |
| web / admin / gate | – | 3000 | 3000 / 3001 / 3002 | 13 / 14 |
| mariadb, nats, minio, mailpit | – | 3306, 4222, 9000, 1025 | 3306, 4222, 9001, 8025 | 01 / 00 |

- service เข้าถึงจากภายนอกได้ผ่าน **gateway เท่านั้น** ที่ `/api/<svc>/...` gateway ตัด `/api/<svc>` ออกแล้วส่งต่อไป `http://<svc>:8080/...` ส่วน `/api/<svc>/internal/*` ถูกบล็อกจากภายนอก
- **Route สาธารณะ (ไม่ต้องมี JWT):** `/api/identity/auth/*`, `/api/identity/.well-known/*`, `/api/content/public/*`, `/api/commerce/products*` (รวม `/products/quote`, `/products/availability`), `/api/commerce/show-rounds`, `/api/commerce/queue/*`, `POST /api/commerce/orders` แบบ guest (ต้องมี queue token), `POST /api/commerce/webhooks/kbank` (commerce ตรวจลายเซ็นเอง), `GET /api/notify/webpush/vapid-public-key`, `POST /api/report/analytics/collect` (rate limit 60 ครั้งต่อนาทีต่อ IP), `GET /api/ticket/transfers/{id}` (ดูบัตรผ่านลิงก์)
- **Demo host (Plan 16):** โดเมนหลัก + `admin.` `gate.` `mock.` `mail.` `files.` `grafana.` หลัง Caddy ทุก service มี `/healthz` และ `/version`
- **IP จริงของผู้ใช้:** gateway ตั้ง `X-Client-IP` จาก `X-Forwarded-For` เฉพาะเมื่อ request มาจาก `TRUSTED_PROXIES` (ค่าเริ่ม: Caddy และ container `web`, `admin`, `gate`) โดยเลือก IP ที่อยู่ขวาสุดซึ่งไม่ได้อยู่ใน trusted list ส่วน BFF ของเว็บ (`/bff/*`, Plan 13) ต้องส่ง `X-Forwarded-For` ต่อทุกครั้ง ไม่อย่างนั้น rate limit และธง `same_ip` จะนับรวมผู้ใช้ทุกคน
- **Idempotency:** `POST /orders` และ `POST /orders/{id}/payments` รับ header `Idempotency-Key` ถ้าส่ง key เดิมซ้ำภายใน 24 ชั่วโมง ต้องตอบผลเดิม
- **ภาษาจีน:** API และ path ใช้ `zh` ส่วน HTML `lang` และ hreflang ใช้ `zh-Hans` · response ของ content ที่ fallback ภาษามี field `lang` บอกภาษาที่ส่งจริง
- **ไฟล์ร่วมฝั่งเว็บ:** `deploy/Dockerfile.web` (build arg `APP`) และ `package.json` แบบ npm workspaces ที่ root สร้างใน Plan 13 แล้ว Plan 14 ใช้ต่อ
- **mockext URL สาธารณะ:** mockext ใช้ `MOCKEXT_PUBLIC_URL` สร้าง `paymentUrl` และ URL ของหน้า OAuth (local `http://localhost:8090`, ใน e2e container `http://mockext:8090`) ส่วนการเรียกระหว่าง server ใช้ `http://mockext:8090` เสมอ

## 3. Authentication

JWT แบบ EdDSA ออกโดย identity · JWKS อยู่ที่ `http://identity:8080/.well-known/jwks.json`

| `typ` | ได้มาจาก | claims | อายุ |
|---|---|---|---|
| `user` | OTP / Social login (Plan 02, 08) | `sub` = E.164 หรืออีเมล, `ctype` = `phone\|email`, `mid` = memberId (ว่างถ้ายังไม่สมัคร), `lang` | access 15 นาที + refresh 30 วัน |
| `staff` | `POST /auth/staff/login {email,password}` (Plan 04 ทำใน identity service) | `sub` = staffId, `roles` = จาก 7 บทบาทด้านล่าง, `name` | 8 ชั่วโมง ไม่มี refresh (demo) |

- บทบาท staff: `admin`, `editor`, `approver`, `finance`, `gate`, `agent_manager`, `viewer`
- identity ใส่ `mid` ทั้งตอน `/auth/verify` และ `/auth/refresh` โดยเรียก gRPC `member.GetMemberByContact` หลังสมัครเสร็จ frontend เรียก `/auth/refresh` เพื่อรับ token ที่มี `mid`
- **Header ที่ gateway ส่งต่อ** (ลบตัวที่ client ส่งมาทิ้งก่อนเสมอ): `X-User-Type`, `X-User-Sub`, `X-Member-Id`, `X-Roles` (คั่นด้วย `,`), `X-Lang`, `X-Request-Id`, `X-Client-IP`
- **`X-Device-Id`** เป็นข้อยกเว้น gateway ส่งต่อค่าที่ client ส่งมา หลังตัดให้เหลือเฉพาะ `[A-Za-z0-9-]{1,64}`
- **ผู้ใช้ที่ยังไม่ล็อกอิน:** `X-Lang` มาจาก `Accept-Language` ถ้าไม่ตรง 7 ภาษาใช้ `en`
- **token ไม่ถูกต้องหรือหมดอายุ** ตอบ `401 invalid_token` ทันที ไม่ปล่อยผ่านเป็นผู้ใช้ไม่ระบุตัวตน
- `pkg/chassis/authn` (Plan 12 Task 1): `type Principal struct { Type, Sub, MemberID string; Roles []string; Lang, ClientIP, DeviceID, RequestID string }`, `authn.From(ctx) Principal`, middleware `authn.Require(types ...string)` และ `authn.RequireRole(roles ...string)` **ต้อง merge ก่อนงานใดที่ใช้ตัวตน** (Plan 03, 04 Task 3, 05, 07, 09)
- Agent / ไกด์ ล็อกอินด้วย OTP ได้ `typ=user` **ไกด์ต้องเป็นสมาชิก** (มี `mid`) agent service ใช้ `X-User-Sub` หาไกด์ของตัวเอง

## 4. gRPC (`api/proto/<svc>/v1`, package `nongnooch.<svc>.v1`)

```proto
service MemberService {        // member:9090  (Plan 03)
  rpc GetMemberByContact(GetMemberByContactRequest{string contact}) returns (Member{string member_id; string lang; string nationality; bool thai_verified; string name; string id_last4});
  rpc GetContactPrefs(GetContactPrefsRequest{string member_id}) returns (ContactPrefs{string lang; bool marketing_consent; string phone; string email});
  rpc GetCompanion(GetCompanionRequest{string member_id; string companion_id}) returns (Holder{string name; string nationality; string id_last4; bool thai_verified});
}
service AgentService {         // agent:9090  (Plan 09)
  rpc ValidateInvite(ValidateInviteRequest{string code}) returns (ValidateInviteResponse{bool valid; string guide_id; string company_id; string guide_member_id});
}
service TicketService {        // ticket:9090 (Plan 07)
  rpc GetTickets(GetTicketsRequest{string order_id}) returns (GetTicketsResponse{repeated Ticket tickets; bool complete});
  rpc VoidTickets(VoidTicketsRequest{string order_id; repeated string order_item_ids; string reason}) returns (VoidTicketsResponse{int32 voided});
}
```

- `Ticket{string ticket_id; string kind; string visit_date; string holder_name; string status; string qr}` โดย status = `active|used|void|transferred`
- `VoidTickets` ที่ `order_item_ids` ว่าง หมายถึง void ทุกใบในออเดอร์
- แผนที่เริ่มก่อนเจ้าของ proto (Plan 05 ก่อน 03, Plan 06 ก่อน 07) ให้สร้างไฟล์ proto ตามหัวข้อนี้ทุกตัวอักษร ถ้ายังไม่มี

## 5. Events (`evt.<type>` บน stream `EVENTS`, Envelope ตาม Plan 01)

| Type | Publisher | Payload |
|---|---|---|
| `otp.requested` | identity | `{requestId, kind, contact, code, lang}` |
| `account.deleted` | identity | `{memberId?, contact, deletedAt}` · consumer: member, notify, agent, report |
| `member.registered` | member | `{memberId, nationality, thaiVerified, lang, channel: "web"\|"app"\|"qr", marketingConsent, inviteCode?, guideId?, companyId?, ip, deviceId, registeredAt}` |
| `member.invite_added` | member | `{memberId, inviteCode, guideId, companyId, ip, deviceId, at}` กรอก invite code ภายหลัง (ไม่นับเป็นสมาชิกใหม่) |
| `member.consent_changed` | member | `{memberId, marketingConsent, changedAt}` |
| `referral.created` | agent | `{referralId, memberId, guideId, guideMemberId, companyId, flags: ["same_device"\|"same_ip"\|"sequential_phone"\|"burst"\|"daily_cap"]}` |
| `agent.approved` | agent | `{kind: "company"\|"guide", id, companyId, memberId?, contactPhone, contactEmail, lang, decision: "approved"\|"rejected"\|"suspended", reason?}` |
| `points.changed` | member | `{memberId, entryId, deltaPoints, reason: "welcome"\|"invite"\|"guide_referral"\|"adjust"\|"cut", status: "available"\|"pending", balance, guideId?}` |
| `price.approved` | content | `{approvalId, changes: [{entity: "product"\|"price_rule"\|"promo_code"\|"show_round"\|"quota"\|"refund_policy"\|"point_rule", op: "upsert"\|"delete", data: {...}}]}` · data ตาม Plan 05 Task 1 (`point_rule` ตาม Plan 03) · consumer: commerce, member |
| `content.published` | content | `{approvalId, action: "publish"\|"unpublish", contentType, contentId, locales: [..], urls: [..]}` |
| `order.paid` | commerce | `{orderId, orderNo, memberId?, guestContact?, lang, paidAmountSatang, paymentId, promoCode?, utm?: {source, medium, campaign}, items: [{orderItemId, productId, kind: "day"\|"annual"\|"restaurant"\|"show", visitDate?, showRoundId?, holder: {name, nationality, idLast4, thaiPrice}, gift?: {contact, name?}, priceSatang}]}` (1 item = บัตร 1 ใบ) |
| `order.expired` | commerce | `{orderId, reason: "hold_timeout"\|"payment_failed"}` |
| `order.refunded` | commerce | `{orderId, refundId, memberId?, guestContact?, lang, amountSatang, mode: "money"\|"voucher", orderItemIds: [..], reason}` (`orderItemIds` ว่าง = ทั้งออเดอร์) |
| `payment.discrepancy` | commerce | `{paymentId, orderId, expectedSatang, actualSatang}` |
| `ticket.issued` | ticket | `{orderId, orderNo, memberId?, guestContact?, lang, tickets: [{ticketId, kind, visitDate?, holderName}]}` |
| `ticket.scanned` | ticket | `{ticketId, kind, memberId?, gateId, scannedAt, result: "ok"\|"used"\|"void"\|"wrong_date"\|"expired"\|"invalid", method: "qr"\|"manual", offline, duplicate}` |
| `ticket.transferred` | ticket | `{oldTicketId, newTicketId, fromMemberId, toMemberId?, toContact?, mode: "contact"\|"link", lang}` |

`events.All` ใน chassis มี **20 ตัว** (17 ตัวเดิม + `account.deleted`, `member.invite_added`, `member.consent_changed`) ส่วน event ของเฟส 2–4 (`mission.completed`, `bill.paid`, `partner.order.updated`) มีชื่อในรายการ แต่ยังไม่มีใคร publish

## 6. QR e-Ticket (Plan 07 ออก, Plan 14 Gate ตรวจ, Plan 13 / 15 แสดง)

`NN1.<b64url(payloadJSON)>.<b64url(ed25519 signature ของ "NN1." + b64url(payloadJSON))>`

- payload: `{"t":"<ticketId>","k":"day"|"annual"|"restaurant"|"show","d":"2027-03-01"|null,"h":"<idLast4>"}` (b64url ไม่มี padding)
- ลายเซ็นใช้ private key จาก env `TICKET_SIGNING_KEY`
- snapshot ของประตูส่ง **`publicKeys[]`** เครื่องประตูต้องลองตรวจกับทุก key ในรายการ จึงเปลี่ยน key ได้โดยไม่ต้องมี `kid` ใน QR และบัตรเก่ายังใช้ได้
- test vector ร่วมอยู่ที่ `services/ticket/testdata/qr_vectors.json` ซึ่ง Plan 14 ต้องใช้ทดสอบตัวตรวจฝั่งประตู

## 7. Seed และ demo

- `make seed` เรียก `POST /internal/seed` ตามลำดับ **identity → member → agent → content → commerce → ticket → notify → report** ทำงานเมื่อ `DEMO_MODE=true` เท่านั้น และรันซ้ำได้ (idempotent) ส่วน `make demo-reset CONFIRM=yes` ล้างข้อมูลแล้ว seed ใหม่
- **ข้อมูลตัวอย่างที่ทุกแผนใช้ร่วมกัน:**

  | ข้อมูล | ค่า |
  |---|---|
  | staff | 1 คนต่อบทบาท อีเมล `<role>@demo.nongnooch.local` รหัสผ่าน `Demo1234!` (ทั้ง 7 บทบาท + `approver2@…` สำหรับทดสอบ four-eyes) |
  | สมาชิกคนไทย | `+66811111111` เลขบัตร `1101700230708` |
  | สมาชิกจีน | `+8613800138000` พาสปอร์ต `E12345678` (CN) |
  | สมาชิกญี่ปุ่น | `taro@example.jp` พาสปอร์ต `TK1234567` (JP) |
  | ไกด์ | `+66822222222` สังกัดบริษัท "Demo Tours" · invite code **`DEMOGUIDE1`** |
  | ตั๋ว QR สำหรับ e2e | ticket seed เขียนไว้ที่ `deploy/seed/demo-tickets.json` |

- UUID ของข้อมูล seed ทุกตัวกำหนดตายตัวใน `deploy/seed/fixtures.json` (สร้างใน Plan 16 Task 5) ทุก service อ่านจากไฟล์นี้

## 8. Waiting room access token (Plan 06 ออก, Plan 12 ตรวจ)

- รูปแบบ: `WR1.<b64url({"s":<seq>,"exp":<unix>,"dev":"<deviceId>"})>.<b64url(ed25519 sig)>` ส่งใน header **`X-Queue-Token`**
- commerce เซ็นด้วย private key `QUEUE_TOKEN_KEY` ส่วน gateway ตรวจด้วย public key `QUEUE_ACCESS_PUBLIC_KEY` ผ่าน `pkg/chassis/queuetoken.Verify(token string, pub ed25519.PublicKey, now time.Time) (Claims, error)`
- route ที่ตรวจกำหนดใน `QUEUE_PROTECTED_ROUTES` (ค่าเริ่ม `POST /api/commerce/orders`) และตอบ error `403 queue_token_required|queue_token_invalid|queue_token_expired`
- **ปิด waiting room อยู่ commerce ก็ออก token ทันที** gateway จึงตรวจแบบเดียวกันเสมอ

## 9. ชื่อ secret ใน env (Plan 16 Task 1)

| Service | env |
|---|---|
| identity | `JWT_ED25519_KEY`, `OTP_PEPPER`, `KBANK_*` ไม่ใช้ |
| member | `MEMBER_ID_ENC_KEY`, `MEMBER_ID_ENC_KID`, `MEMBER_ID_HMAC_KEY` (member เท่านั้น) |
| commerce | `KBANK_API_KEY`, `KBANK_WEBHOOK_SECRET`, `QUEUE_TOKEN_KEY` (Ed25519 private), `QUEUE_TICKET_SECRET` (HMAC ของบัตรคิวภายใน) |
| gateway | `QUEUE_ACCESS_PUBLIC_KEY` |
| ticket | `TICKET_SIGNING_KEY`, `TICKET_PREVIOUS_PUBLIC_KEYS` (คั่นด้วย `,` สำหรับหมุน key) |
| notify | `VAPID_PUBLIC_KEY`, `VAPID_PRIVATE_KEY`, `VAPID_SUBJECT` |
| content, agent, ticket | `S3_ENDPOINT`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_BUCKET`, `S3_PUBLIC_BASE_URL` · bucket: content → `content`, agent → `agent-docs`, ticket → `ticket-photos` (รูปผู้ถือบัตรรายปี) |

## 10. Deep link และ Push

- **Web path:** `/r/{inviteCode}` (QR ของไกด์ · invite code เก็บเป็น `[A-Z0-9]{4,32}` ไม่มีขีด ตอนรับ input ให้ตัดช่องว่าง / ขีด แล้วแปลงเป็นตัวพิมพ์ใหญ่ก่อนตรวจ), `/t/{transferId}` (รับบัตรที่เพื่อนส่ง), `/tickets`, `/tickets/{ticketId}`, `/points`, `/shows`, `/agent` ทุก path มี prefix ภาษา เช่น `/th/r/DEMOGUIDE1` ถ้าไม่มี prefix ใช้ `Accept-Language`
- **App:** Universal Link / App Link ใช้ path เดียวกับเว็บ (เว็บโฮสต์ `/.well-known/apple-app-site-association` และ `/.well-known/assetlinks.json`) ส่วน custom scheme คือ `nongnooch://<path>`
- **Push:** ทุกข้อความมี `data: {type, link}` โดย `link` เป็น path ข้างบน
- **ลงทะเบียนเครื่อง:** `POST /api/notify/devices {token, platform: "ios"|"gms"|"hms"|"web", lang, app: "tourist"|"agent"|"web"}` push ของไกด์ส่งเฉพาะเครื่องที่ `app=agent`

## 11. รายการเปลี่ยนแปลงจาก v1

| หัวข้อ | เปลี่ยน | เสนอโดย |
|---|---|---|
| 1 | make target ฝั่ง frontend / mobile / seed, รูปแบบ contact, การลบบัญชีเก็บ `id_hmac`, รูปแบบ audit | 14, 16, 05, 03 + มติผู้ใช้ |
| 2 | route สาธารณะ, demo host, IP จริงผ่าน BFF, Idempotency-Key, `zh` / `zh-Hans`, Dockerfile.web, `MOCKEXT_PUBLIC_URL` | 06, 10, 12, 13, 16 |
| 3 | อายุ token, `X-Device-Id`, `X-Lang` ผู้ใช้ไม่ล็อกอิน, `401 invalid_token`, field ของ `Principal`, ไกด์ต้องเป็นสมาชิก | 12, 03, 07 |
| 4 | `guide_member_id`, `VoidTickets.order_item_ids` | 03, 07, 11 |
| 5 | event ใหม่ 3 ตัว, field ที่เพิ่มใน `member.registered`, `referral.created`, `agent.approved`, `points.changed`, `price.approved`, `content.published`, `order.paid`, `order.refunded`, `ticket.scanned`, `ticket.transferred` | 03, 04, 05, 07, 08, 10, 11, 14 |
| 6 | `k` เพิ่ม restaurant / show, `publicKeys[]`, test vector | 07, 14 |
| 7 | ลำดับ seed + fixtures ตายตัว (เลขบัตรไทยแก้เป็น `1101700230708` ให้ผ่าน check digit) | 04, 07, 10, 14, 16 |
| 8 | waiting room token (ใช้แบบของ Plan 06) | 06, 12 |
| 9 | ชื่อ secret (+ `QUEUE_TICKET_SECRET`, `S3_BUCKET`, `S3_PUBLIC_BASE_URL`, `TICKET_PREVIOUS_PUBLIC_KEYS`) | 06, 07, 16 |
| 10 | deep link, push data, `app` ของเครื่อง | 10, 15 |
