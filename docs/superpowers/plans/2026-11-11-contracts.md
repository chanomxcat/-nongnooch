# Phase 1 Shared Contracts

ทุกแผน implementation ของเฟส 1 (Plan 00–16) **ต้องใช้ชื่อ, type และรูปแบบตามไฟล์นี้** ถ้าแผนใดต้องเปลี่ยนสัญญา ห้ามแก้ในแผนนั้นเอง ให้เสนอใน PR ที่แก้ไฟล์นี้ก่อน

**Spec:** [system-design.md](../../system-design.md) · [DESIGN.md](../../design/DESIGN.md) · [ดัชนีแผน](2026-11-11-phase1-index.md)

## 1. กติการ่วม

- **Build / test ใน Docker เท่านั้น:** `make test`, `make test-int`, `make build`, `make gen`, `make seed` และงานฝั่ง frontend ก็รันใน container (`node:20-bookworm`, `ghcr.io/cirruslabs/flutter:3.24.0`) ผ่าน make target ห้ามรัน `go`, `npm`, `flutter` บนเครื่องโดยตรง
- **Demo:** ระบบภายนอกทุกตัวใช้ mockext / Mailpit ตาม [Plan 00](2026-11-11-00-mock-external.md) และเรียกผ่าน interface + base URL ใน env
- **7 ภาษา:** `th en zh ja ko de fr` (ZH = ตัวย่อ) ถ้าไม่มีข้อความในภาษานั้นให้ใช้ `en`
- **เงิน:** `int64` หน่วยสตางค์ (THB × 100) field ชื่อลงท้าย `Satang` เช่น `amountSatang`
- **เวลา:** UTC RFC3339 (`2027-03-01T02:00:00Z`) ส่วน**วันเข้าชม**เป็น `YYYY-MM-DD` ตามเวลา Asia/Bangkok field ชื่อ `visitDate`
- **ID:** UUIDv7 string ทุกตัว (`events.New` / `uuid.NewV7`) ยกเว้นเลขที่คนอ่าน เช่น `orderNo` = `NN` + `YYMMDD` + 6 หลัก
- **JSON:** camelCase · error `{"error":"snake_case_code","message":"ข้อความสำหรับ log"}` (ข้อความที่แสดงให้ผู้ใช้แปลฝั่ง frontend จาก `error`) · list ใช้ `?cursor=&limit=` (ค่าเริ่ม 20 สูงสุด 100) ตอบ `{"items":[...],"nextCursor":"..."}`
- **เลขบัตรประชาชน / พาสปอร์ต:** ส่งข้าม service ได้แค่ `idLast4` และ `nationality` ห้ามส่งเลขเต็มใน event, log หรือ response (ยกเว้น API ถอดรหัสของ Member ที่ต้องมีสิทธิ์)

## 2. Repo และ service

Monorepo คือ repo นี้ ([github.com/chanomxcat/-nongnooch](https://github.com/chanomxcat/-nongnooch)) โค้ดอยู่ที่ root คู่กับ `docs/` และ `roadmap/` (Go module `nongnooch/platform`)

```
pkg/chassis/*            Plan 01
services/<svc>/          Go service ละโฟลเดอร์ (migrations/ ภายใน)
api/openapi/<svc>.yaml   OpenAPI 3.1 ภายนอก
api/proto/<svc>/v1/*.proto
gen/{go,ts,dart}/        จาก make gen (commit ลง repo)
apps/web/                Next.js เว็บนักท่องเที่ยว + หน้า Agent (Plan 13)
apps/admin/              Next.js Admin / CMS / Dashboard (Plan 14)
apps/gate/               Next.js PWA Gate Scanner (Plan 14)
apps/mobile/             Flutter Tourist + Agent (Plan 15)
packages/tokens/         design tokens: tokens.json → tokens.css + tokens.dart (Plan 13 Task 1)
deploy/                  compose.yaml, Dockerfile.go, Dockerfile.web, seed/
```

| Service (compose) | Schema | HTTP / gRPC ในเครือข่าย | พอร์ตบนเครื่อง | แผน |
|---|---|---|---|---|
| gateway | – | 8080 | **8000** | 12 |
| identity | `identity` | 8080 / 9090 | – | 02, 08 |
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

service ภายนอกเข้าถึงได้ผ่าน **gateway เท่านั้น** ที่ `http://localhost:8000/api/<svc>/...` gateway จะตัด `/api/<svc>` ออกแล้วส่งต่อไปที่ `http://<svc>:8080/...` ส่วน `/api/<svc>/internal/*` ถูกบล็อกจากภายนอก

## 3. Authentication

JWT แบบ EdDSA ออกโดย identity · JWKS อยู่ที่ `http://identity:8080/.well-known/jwks.json`

| `typ` | ได้มาจาก | claims |
|---|---|---|
| `user` | OTP / Social login (Plan 02, 08) | `sub` = E.164 หรืออีเมล, `ctype` = `phone\|email`, `mid` = memberId (ว่างถ้ายังไม่สมัคร), `lang` |
| `staff` | `POST /auth/staff/login` (Plan 04 สร้างใน identity service) | `sub` = staffId, `roles` = รายการจาก 7 บทบาทด้านล่าง, `name` |

- บทบาท staff: `admin`, `editor`, `approver`, `finance`, `gate`, `agent_manager`, `viewer`
- หลังสมัครสมาชิกเสร็จ frontend เรียก `POST /auth/refresh` แล้ว identity จะเรียก gRPC `member.GetMemberByContact` เพื่อใส่ `mid` ลงใน token ใหม่
- **gateway ตรวจ JWT แล้วส่ง header ต่อ:** `X-User-Type`, `X-User-Sub`, `X-Member-Id`, `X-Roles` (คั่นด้วย `,`), `X-Lang`, `X-Request-Id`, `X-Client-IP`, `X-Device-Id` (มาจาก header `X-Device-Id` ของ client) โดยลบ header เหล่านี้ที่ client ส่งมาเองทิ้งก่อน
- service อ่านตัวตนจาก header เหล่านี้ผ่าน `pkg/chassis/authn` (Plan 12 Task 1) ซึ่งมี `authn.From(ctx) Principal` และ middleware `authn.Require(types ...string)`, `authn.RequireRole(roles ...string)`
- Agent / ไกด์ ล็อกอินด้วย OTP ได้ `typ=user` แล้ว agent service ใช้ `X-User-Sub` (เบอร์ E.164) หาไกด์ของตัวเอง

## 4. gRPC (`api/proto/<svc>/v1`, package `nongnooch.<svc>.v1`)

```proto
service MemberService {        // member:9090  (Plan 03)
  rpc GetMemberByContact(GetMemberByContactRequest{string contact}) returns (Member{string member_id; string lang; string nationality; bool thai_verified; string name; string id_last4});
  rpc GetContactPrefs(GetContactPrefsRequest{string member_id}) returns (ContactPrefs{string lang; bool marketing_consent; string phone; string email});
  rpc GetCompanion(GetCompanionRequest{string member_id; string companion_id}) returns (Holder{string name; string nationality; string id_last4; bool thai_verified});
}
service AgentService {         // agent:9090  (Plan 09)
  rpc ValidateInvite(ValidateInviteRequest{string code}) returns (ValidateInviteResponse{bool valid; string guide_id; string company_id});
}
service TicketService {        // ticket:9090 (Plan 07)
  rpc GetTickets(GetTicketsRequest{string order_id}) returns (GetTicketsResponse{repeated Ticket tickets; bool complete});
  rpc VoidTickets(VoidTicketsRequest{string order_id; string reason}) returns (VoidTicketsResponse{int32 voided});
}
```

`Ticket{string ticket_id; string kind; string visit_date; string holder_name; string status; string qr}` โดย status = `active|used|void|transferred`

## 5. Events (`evt.<type>` บน stream `EVENTS`, Envelope ตาม Plan 01)

| Type | Publisher | Payload |
|---|---|---|
| `otp.requested` | identity | `{requestId, kind, contact, code, lang}` |
| `member.registered` | member | `{memberId, nationality, thaiVerified, lang, channel: "web"\|"app"\|"qr", inviteCode?, guideId?, companyId?, ip, deviceId, registeredAt}` |
| `referral.created` | agent | `{referralId, memberId, guideId, companyId, flags: ["same_device"\|"same_ip"\|"sequential_phone"\|"burst"\|"daily_cap"]}` |
| `agent.approved` | agent | `{kind: "company"\|"guide", id, companyId, contactPhone, contactEmail, lang, decision: "approved"\|"rejected"\|"suspended", reason?}` |
| `points.changed` | member | `{memberId, entryId, deltaPoints, reason: "welcome"\|"invite"\|"guide_referral"\|"adjust"\|"cut", status: "available"\|"pending", balance}` (ไกด์เป็นสมาชิกด้วย ใช้ memberId ของไกด์) |
| `price.approved` | content | `{approvalId, changes: [{entity: "product"\|"price_rule"\|"promo_code"\|"show_round"\|"quota", op: "upsert"\|"delete", data: {...}}]}` (data ตามตารางของ commerce, Plan 05) |
| `content.published` | content | `{approvalId, contentType, contentId, locales: [..], urls: [..]}` |
| `order.paid` | commerce | `{orderId, orderNo, memberId?, guestContact?, lang, paidAmountSatang, paymentId, items: [{orderItemId, productId, kind: "day"\|"annual"\|"restaurant"\|"show", visitDate?, showRoundId?, holder: {name, nationality, idLast4, thaiPrice}, priceSatang}]}` (1 item = บัตร 1 ใบ) |
| `order.expired` | commerce | `{orderId, reason: "hold_timeout"\|"payment_failed"}` |
| `order.refunded` | commerce | `{orderId, refundId, amountSatang, mode: "money"\|"voucher", reason}` |
| `payment.discrepancy` | commerce | `{paymentId, orderId, expectedSatang, actualSatang}` |
| `ticket.issued` | ticket | `{orderId, orderNo, memberId?, guestContact?, lang, tickets: [{ticketId, kind, visitDate?, holderName}]}` |
| `ticket.scanned` | ticket | `{ticketId, memberId?, gateId, scannedAt, result: "ok"\|"used"\|"void"\|"wrong_date"\|"expired"\|"invalid", offline, duplicate}` |
| `ticket.transferred` | ticket | `{oldTicketId, newTicketId, fromMemberId, toMemberId?, toContact, lang}` |

event ของเฟส 2–4 (`mission.completed`, `bill.paid`, `partner.order.updated`) ยังไม่ทำ

## 6. QR e-Ticket (Plan 07 ออก, Plan 14 Gate ตรวจ, Plan 13 / 15 แสดง)

`NN1.<b64url(payloadJSON)>.<b64url(ed25519 signature ของ "NN1." + b64url(payloadJSON))>`

payload: `{"t":"<ticketId>","k":"day"|"annual","d":"2027-03-01"|null,"h":"<idLast4>"}` (b64url ไม่มี padding) · ลายเซ็นใช้ private key จาก env `TICKET_SIGNING_KEY` ส่วน public key ส่งให้เครื่องประตูผ่าน snapshot

## 7. Seed และ demo

`make seed` (Plan 1.16.11) เรียก `POST /internal/seed` ของแต่ละ service ตามลำดับ identity → member → agent → content → commerce → ticket ทุก service ที่มีข้อมูลต้องมี endpoint นี้ ทำงานเมื่อ `DEMO_MODE=true` เท่านั้น และรันซ้ำได้ (idempotent)
