# Demo Slice (12 พ.ย. 2026 11:00) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** demo ที่ใช้งานได้จริงตั้งแต่ต้นจนจบ สำหรับนำเสนอลูกค้า **พฤ. 12 พ.ย. 2026 11:00** ตามเส้นทาง: เว็บ Concept A → สมัครสมาชิกด้วย OTP (mock) + เลขบัตร + invite code `DEMOGUIDE1` → ได้ welcome point → ซื้อบัตร 4 ขั้น → จ่ายผ่านหน้า KBank จำลอง → ได้ e-Ticket QR รายใบ → สแกนที่ Gate Scanner ผ่าน / ใช้แล้ว → Admin dashboard แสดงยอด

**Architecture:** เป็นส่วนย่อยของแผนเฟส 1 ใช้ [contracts v2](2026-11-11-contracts.md) ทุกชื่อ microservice ภาษา Go บน MariaDB (1 schema ต่อ service) + NATS JetStream (outbox / consumer idempotent) + gateway ตรวจ JWT · ระบบภายนอกเป็น mock ทั้งหมด (mockext) · เว็บเป็น Next.js · ทุกอย่างรันด้วย `docker compose` บนเครื่องเดียว

**Tech Stack:** Go 1.23, MariaDB 10.11, NATS 2.10 (JetStream), Next.js 14 (App Router, TypeScript), Docker Compose, Make

**Spec:** [system-design.md](../../system-design.md) F1, F3, F4, S1, S3, S5 · [DESIGN.md](../../design/DESIGN.md) · แผนเต็มที่อ้างถึงในแต่ละ task (อ่านเฉพาะส่วนที่ระบุ)

## Global Constraints

- **Build / test / run ใน Docker เท่านั้น** ผ่าน `make` ห้ามรัน `go`, `npm`, `node` บนเครื่องโดยตรง (`make test`, `make test-int`, `make up`, `make fe-test APP=web` ฯลฯ ตาม contracts §1)
- **ระบบภายนอกทั้งหมดเป็น mock** (mockext) และเรียกผ่าน interface + base URL ใน env
- **ชื่อ, รูปแบบ event, header, QR, เงิน (สตางค์), เวลา ต้องตาม contracts v2** ห้ามตั้งชื่อใหม่
- **เวลาจำกัด:** ทำเฉพาะสิ่งที่ระบุในแต่ละ task ห้ามเพิ่มฟีเจอร์ ฟีเจอร์ในแผนเต็มที่ไม่ได้ระบุ = ข้าม
- **ทุก task ต้องจบด้วย stack ที่ยัง `make up` ได้** และ test ของ task ผ่าน
- **Test:** Go ใช้ test จริงกับ MariaDB / NATS ใน compose (`//go:build integration`) สำหรับ logic ที่แตะ DB · ฝั่งเว็บเน้น smoke ของเส้นทางหลัก ไม่ต้องทำ test ครบทุก component
- **ภาษา:** เว็บต้องสลับ 7 ภาษาได้ (`th en zh ja ko de fr`) โดยข้อความ UI หลักแปลครบอย่างน้อย TH / EN / ZH ภาษาที่เหลือใช้ EN แทนได้
- **ข้อมูล demo:** ตาม contracts §7 (สมาชิกไทย `+66811111111` เลขบัตร `1101700230708`, invite `DEMOGUIDE1`, staff `<role>@demo.nongnooch.local` / `Demo1234!`)
- **Commit** ทุก task บน branch `demo-slice` พร้อมบรรทัด `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`

## Review Focus

1. **จ่ายเงินแล้ว webhook มาซ้ำ** ต้องได้บัตรชุดเดียว ไม่ออกซ้ำ (Task 6, 7)
2. **สแกน QR ใบเดียวกันสองครั้ง** ครั้งที่สองต้องได้ "ใช้แล้ว" พร้อมเวลา (Task 7)
3. **เลขบัตรประชาชน** เก็บแบบเข้ารหัส + HMAC ห้ามโผล่ใน log / response เกิน 4 หลักท้าย (Task 5)
4. **ราคาไทย** ซื้อได้เฉพาะเมื่อผู้ถือเป็นคนไทยที่ยืนยันเลขบัตรแล้ว (Task 6)
5. **การ demo ต่อหน้าลูกค้า**: `make up && make seed` จากเครื่องเปล่าต้องขึ้นครบภายใน 5 นาที และ `make demo-reset CONFIRM=yes` กลับสู่สถานะเริ่มต้นได้ (Task 10)

## File Structure

ตาม contracts §2: `pkg/chassis/*`, `services/{mockext,identity,notify,member,commerce,ticket,gateway}`, `apps/web`, `apps/gate`, `apps/admin` (รวมเป็นแอปเดียวได้ใน demo ดู Task 9), `deploy/`, `Makefile`

---

### Task 1: Repo + Docker toolchain + Chassis

**อ้างอิง:** [Plan 01](2026-11-11-01-platform-chassis.md) Task 1–6 (ทำครบตามแผนนั้น ยกเว้น Task 7 codegen) · contracts §1, §2, §5
**ผลลัพธ์:** `go.mod` (module `nongnooch/platform`), `Makefile` (`up`, `down`, `test`, `test-int`, `build`, `logs`), `deploy/compose.yaml` (mariadb 10.11 + init สร้าง schema 8 ตัวและ user ต่อ schema, nats `-js`, service `gotest` ใน profile tools), `deploy/Dockerfile.go`, `pkg/chassis/{config,db,events,outbox,consumer,server,obs}`, `services/example` ไม่ต้องทำ (ใช้ test ของ chassis แทน)

- [ ] **Step 1:** ทำตาม Plan 01 Task 1–6 ตามลำดับ (TDD ตามแผนนั้น) โดย `events.All` มี 20 ตัวตาม contracts §5 · obs ใช้ `slog` JSON และ OTel แบบ noop ถ้าไม่มี endpoint (ไม่ต้องมี collector ใน compose)
- [ ] **Step 2:** ตรวจ: `make up` ขึ้น mariadb + nats healthy, `make test PKG=./pkg/...` และ `make test-int PKG=./pkg/...` ผ่านทั้งหมด
- [ ] **Step 3:** Commit `feat(chassis): dockerized toolchain and platform chassis`

### Task 2: mockext (Console + Turnstile + SMS + KBank)

**อ้างอิง:** [Plan 00](2026-11-11-00-mock-external.md) Task 1, 2, 4 (ข้าม Task 3 OAuth และ Task 5 Push / Legacy) · `MOCKEXT_PUBLIC_URL` ตาม contracts §2
**ผลลัพธ์:** service `mockext` ใน compose port 8090 · Console ที่ `/` แสดง SMS (OTP) และการจ่ายเงินล่าสุด · `/turnstile/v0/siteverify` · `/sms/send` + `GET /sms/messages?to=` · `/pg/payments`, `/pg/checkout/{id}` (ปุ่ม จ่ายสำเร็จ / ล้มเหลว / ยกเลิก / ไม่ส่ง webhook / ยอดไม่ตรง), webhook ลงลายเซ็น `X-Mock-Signature` + retry, `webhook:resend`, inquiry `GET /pg/payments?ref=`, `POST /pg/refunds`
- หน้า checkout ใช้สีแบรนด์จาก DESIGN.md (เขียว `#074536` / ทอง `#D9A52E`) และขึ้นหัวว่า "KBank Payment Gateway (จำลอง)"

- [ ] **Step 1:** ทำตาม Plan 00 Task 1, 2, 4 (TDD) · Store เก็บใน memory
- [ ] **Step 2:** ตรวจ: `make test PKG=./services/mockext/...` ผ่าน, `make up` แล้ว `curl -s localhost:8090/healthz` ได้ 200
- [ ] **Step 3:** Commit `feat(mockext): console, turnstile, sms and kbank mocks`

### Task 3: Identity (OTP + JWT) + Notify (ส่ง OTP)

**อ้างอิง:** [Plan 02](2026-11-11-02-identity-otp.md) Task 1–5 (Task 5: SMS ผ่าน mockext อย่างเดียว ข้ามอีเมล / Mailpit) · contracts §3
**ผลลัพธ์:** `POST /auth/otp`, `POST /auth/verify` (JWT EdDSA `typ=user`, `mid` ว่าง), `POST /auth/refresh`, `POST /auth/logout`, `GET /.well-known/jwks.json` · service `notify` consume `otp.requested` ส่งไป mockext `/sms/send` ข้อความตามภาษา 7 ไฟล์
- **identity เตรียมจุดเติม `mid`:** interface `MemberLookup{ GetMemberIDByContact(ctx, contact) (string, error) }` ใช้ตอน `/auth/verify` และ `/auth/refresh` ใน task นี้ให้ใช้ implementation ที่คืน `""` เสมอ แล้ว Task 5 จะเปลี่ยนเป็นตัวที่เรียก member

- [ ] **Step 1:** ทำตาม Plan 02 Task 1–5 ตามขอบเขตข้างบน (TDD)
- [ ] **Step 2:** ตรวจ end-to-end: `POST /auth/otp` ด้วย `turnstileToken:"mock-ok"` → เห็น OTP ใน mockext Console / `GET /sms/messages?to=+66811111111` → `POST /auth/verify` ได้ token
- [ ] **Step 3:** Commit `feat(identity): otp login, jwt and sms delivery via notify`

### Task 4: Gateway + authn

**อ้างอิง:** [Plan 12](2026-11-11-12-api-gateway.md) Task 1 (authn) และส่วน routing / JWT / header / CORS · contracts §2, §3 (ข้าม rate limit, OpenAPI รวม, queue token)
**ผลลัพธ์:** `pkg/chassis/authn` (`Principal`, `From`, `Require`, `RequireRole`) · service `gateway` port **8000**: route `/api/<svc>/*` → `http://<svc>:8080/*`, บล็อก `/internal/*`, ตรวจ JWT ผ่าน JWKS ของ identity (cache), ลบแล้วใส่ header `X-User-*`, `X-Member-Id`, `X-Roles`, `X-Lang`, `X-Request-Id`, `X-Client-IP`, `X-Device-Id`, route สาธารณะตาม contracts §2, token ผิดได้ `401 invalid_token`, CORS ให้ `localhost:3000-3002`

- [ ] **Step 1:** TDD ตาม Plan 12 สำหรับ authn + proxy (test: header ปลอมจาก client ถูกลบ, `/internal` ได้ 404, token หมดอายุได้ 401, route สาธารณะผ่านโดยไม่มี token)
- [ ] **Step 2:** ตรวจ: `curl localhost:8000/api/identity/.well-known/jwks.json` ได้ key
- [ ] **Step 3:** Commit `feat(gateway): jwt-verifying reverse proxy and authn package`

### Task 5: Member & Point (สมัคร + welcome / invite point)

**อ้างอิง:** [Plan 03](2026-11-11-03-member-point.md) Task 1, 2, 3, 5 (เฉพาะ GetMemberByContact) · contracts §1 (error code), §4, §5
**ผลลัพธ์:** schema `member` · เข้ารหัสเลขบัตร (AES-GCM, key `MEMBER_ID_ENC_KEY`) + HMAC (`MEMBER_ID_HMAC_KEY`) + mask 4 หลัก · `POST /members` (ต้องมี JWT `typ=user` ใช้ `X-User-Sub` เป็น contact): ชื่อ, สัญชาติ, `idType` `thai_id|passport`, `idNumber` (ตรวจ check digit 13 หลัก), consent, `inviteCode?` · 1 contact / 1 เลขบัตร = 1 บัญชี (`409 id_number_taken`) · PointLedger append-only: welcome 100, invite 50 เมื่อ invite code = `DEMOGUIDE1` หรือ code ที่ seed ไว้ (**demo ไม่มี agent service** ให้ member ตรวจ invite code จากตาราง `demo_invite_codes` ที่ seed ไว้ แทนการเรียก gRPC `ValidateInvite` ผ่าน interface `InviteValidator`) · `GET /members/me` (ชื่อ, สัญชาติ, `thaiVerified`, `idLast4`), `GET /members/me/points` (balance + history) · outbox `member.registered` · gRPC `GetMemberByContact` และ `GetCompanion` (ส่ง `companion_id` = `"me"` ได้ข้อมูลตัวเอง) · identity เปลี่ยน `MemberLookup` เป็นตัวที่เรียก gRPC นี้
- `POST /internal/seed`: สมาชิกไทย / จีน / ญี่ปุ่นตาม contracts §7 + invite `DEMOGUIDE1`

- [ ] **Step 1:** TDD (check digit: `1101700230708` ผ่าน, `1101700230705` ไม่ผ่าน · สมัครซ้ำเลขบัตรเดิมได้ 409 · welcome + invite รวม 150 · ไม่มีเลขเต็มใน response / log)
- [ ] **Step 2:** ตรวจ: สมัครผ่าน gateway แล้ว `/auth/refresh` ได้ token ที่มี `mid`
- [ ] **Step 3:** Commit `feat(member): registration with encrypted id, welcome and invite points`

### Task 6: Commerce (สินค้า + ออเดอร์ + จ่ายเงิน)

**อ้างอิง:** [Plan 05](2026-11-11-05-commerce-catalog-order.md) Task 1–4 (ไม่ต้องมี consumer `price.approved` ใน demo: สินค้า / ราคา / โควตามาจาก seed), [Plan 06](2026-11-11-06-commerce-payment.md) Task 1–3 (สร้างการจ่าย + webhook + outbox `order.paid`) · contracts §4, §5
**ผลลัพธ์:** `GET /products?visitDate=&lang=`, `GET /products/availability`, `POST /orders` (วันเข้าชม, รายการบัตรพร้อมผู้ถือรายใบ, ราคาไทยเฉพาะผู้ถือที่ `thai_verified` ผ่าน `GetCompanion`, จองโควตา 15 นาที, `Idempotency-Key`) · `POST /orders/{id}/payments {method, returnUrl}` → `paymentUrl` ของ mockext · `POST /webhooks/kbank` ตรวจลายเซ็น, idempotent ด้วย eventId, ทำ transaction เดียว Payment paid + Order confirmed + outbox `order.paid` · `GET /orders/{id}`, `GET /orders` (ของฉัน) · `GET /admin/stats/today` (role staff ใดก็ได้): ยอดขายวันนี้ (สตางค์), จำนวนออเดอร์, จำนวนบัตร แยกตามประเภท · ข้าม: waiting room, กระทบยอด, คืนเงิน, promo
- seed สินค้า: บัตรรายวันผู้ใหญ่ ไทย 300 / ต่างชาติ 600 บาท, เด็ก ไทย 150 / ต่างชาติ 300 บาท, บัตรรายปี ไทย 1,500 / ต่างชาติ 3,000 บาท, โควตา 5,000 ใบ / วัน ล่วงหน้า 60 วัน (ราคา demo ไม่ใช่ราคาจริง)

- [ ] **Step 1:** TDD (ราคาไทยกับผู้ถือต่างชาติ → `422 thai_price_not_allowed` · webhook ซ้ำ eventId เดิม → order.paid 1 ครั้ง · โควตาไม่พอ → `409 sold_out`)
- [ ] **Step 2:** ตรวจ end-to-end: สร้างออเดอร์ → เปิด `paymentUrl` กดจ่ายสำเร็จ → `GET /orders/{id}` เป็น `confirmed`
- [ ] **Step 3:** Commit `feat(commerce): catalog, orders and kbank payment with order.paid`

### Task 7: Ticket & Gate (ออกบัตร QR + สแกน)

**อ้างอิง:** [Plan 07](2026-11-11-07-ticket-gate.md) Task 1, 2, 5 · contracts §5, §6
**ผลลัพธ์:** consume `order.paid` ออก Ticket รายใบ (Unique ID + QR `NN1.` ลงลายเซ็น Ed25519 ตาม contracts §6, idempotent ด้วย orderId) + outbox `ticket.issued` · `GET /tickets/me` (บัตรของฉัน + `qr`) · gRPC `GetTickets` · `GET /gate/snapshot` (`publicKeys[]`, บัตรวันนี้) · `POST /gate/scans {qr | ticketId, gateId}` (role `gate`) ตรวจลายเซ็น + สถานะแบบ atomic ตอบ `ok|used|void|wrong_date|invalid` พร้อม `holderName`, `idLast4`, `usedAt` + outbox `ticket.scanned` · `GET /admin/stats/today`: จำนวนสแกนเข้าวันนี้ · ข้าม: transfer, บัตรรายปี activation, legacy, offline batch
- `services/ticket/testdata/qr_vectors.json` ตาม contracts §6

- [ ] **Step 1:** TDD (QR ผิดลายเซ็น → invalid · สแกนซ้ำ → used พร้อมเวลาแรก · สแกนพร้อมกัน 2 request → ok ได้ครั้งเดียว · order.paid ซ้ำ → บัตรชุดเดียว)
- [ ] **Step 2:** ตรวจ end-to-end ต่อจาก Task 6: หลังจ่ายเงิน `GET /tickets/me` เห็น QR ภายใน 30 วินาที
- [ ] **Step 3:** Commit `feat(ticket): signed qr tickets and gate scan api`

### Task 8: Website (Next.js) — Concept A + สมัคร + ซื้อบัตร + บัตรของฉัน

**อ้างอิง:** [Plan 13](2026-11-11-13-website.md) Task 1 (tokens), 2 (scaffold + i18n), 3 (landing), 4 (สมัคร / ล็อกอิน แบบ OTP เท่านั้น), 5 (checkout), 7 (บัตรของฉัน + พ้อย) · [DESIGN.md](../../design/DESIGN.md) · ภาพอ้างอิง `docs/design/ref/web-landing-concept-a.png`, `assets/figma-buy-tickets.png`
**ผลลัพธ์:** `apps/web` port 3000 ใน compose (`deploy/Dockerfile.web`), BFF `/bff/*` เก็บ token ใน httpOnly cookie และส่ง `X-Forwarded-For` · path `/th /en /zh /ja /ko /de /fr` + ตัวสลับภาษา
- **Landing Concept A:** Nav sticky (utility bar + เมนู + ปุ่มทอง Buy Tickets + Sign in), Hero พร้อมการ์ดกิจกรรม 6 ใบ, section Products & Services, News, Newsletter → ชวนสมัครสมาชิก, Footer · ใช้ token สี / ตัวอักษร / radius จาก DESIGN.md · รูปใช้ไฟล์ใน `apps/web/public/images` (ใช้ภาพ placeholder ที่สวยงามได้ หรือ export จาก Figma ถ้ามี)
- **สมัคร / ล็อกอิน:** เบอร์โทร → OTP (Turnstile mock token `mock-ok`) → ถ้ายังไม่มีบัญชี: ชื่อ, สัญชาติ, เลขบัตร / พาสปอร์ต, PDPA, invite code (รับจาก `?invite=` หรือ `/r/{code}`) → แสดง "ได้รับ 150 พ้อย"
- **ซื้อบัตร 4 ขั้นตาม Figma:** เลือก pass → ไทย / ต่างชาติ → วันเข้าชม → จำนวน + ชื่อผู้ถือรายใบ (ใบแรกเป็นตัวเองได้) → สรุปออเดอร์ → จ่าย (redirect ไป mockext) → กลับมาหน้ายืนยัน
- **บัตรของฉัน:** QR รายใบ (สร้างภาพ QR ฝั่ง client จากสตริง `qr`), ชื่อผู้ถือ, 4 หลักท้ายเลขบัตร · **พ้อยของฉัน:** ยอด + ประวัติ + "เปิดแลกพ้อยเร็ว ๆ นี้"
- ข้าม: social login, agent pages, web push, SEO, campaign, transfer

- [ ] **Step 1:** scaffold + tokens + i18n (TH / EN / ZH ครบ ภาษาอื่น fallback EN) + `make fe-build APP=web` ผ่าน
- [ ] **Step 2:** landing + สมัคร + checkout + บัตรของฉัน
- [ ] **Step 3:** Playwright smoke 1 เส้น (`make e2e APP=web`): สมัครด้วย OTP จาก mockext `GET /sms/messages` → ซื้อบัตร → จ่ายที่ mockext → เห็น QR
- [ ] **Step 4:** Commit `feat(web): concept a site, otp signup, ticket checkout and my tickets`

### Task 9: Gate Scanner + Admin Dashboard

**อ้างอิง:** [Plan 14](2026-11-11-14-admin-gate.md) Task 1 (login staff), 7 (สแกน แบบออนไลน์), 10 (dashboard) · contracts §3 staff
**ผลลัพธ์:** staff login: เพิ่ม `POST /auth/staff/login {email,password}` ใน identity (bcrypt, staff seed ตาม contracts §7, JWT `typ=staff` 8 ชม.) · `apps/admin` port 3001 มีสองส่วน
- `/gate`: เปิดกล้องสแกน QR (ไลบรารี `@zxing/browser` หรือเทียบเท่า) + ช่องวาง QR สำหรับ demo บนโน้ตบุ๊ก → เรียก `POST /api/ticket/gate/scans` → หน้าจอผลขนาดใหญ่: เขียว "ผ่าน" + ชื่อผู้ถือ + 4 หลักท้าย / แดง "ใช้แล้ว เวลา …" / "ไม่ถูกต้อง"
- `/dashboard`: การ์ดตัวเลข ยอดขายวันนี้ (บาท), ออเดอร์, บัตรที่ขาย, สมาชิกใหม่วันนี้, คนเข้าประตูวันนี้ (ดึงจาก `GET /admin/stats/today` ของ commerce, member, ticket ผ่าน gateway) refresh ทุก 10 วินาที
- member เพิ่ม `GET /admin/stats/today` (สมาชิกใหม่วันนี้, รวม)

- [ ] **Step 1:** staff login + stats endpoint ของ member (TDD)
- [ ] **Step 2:** หน้า gate + dashboard ด้วย token จาก DESIGN.md
- [ ] **Step 3:** ตรวจ: สแกน QR จาก Task 8 ได้ "ผ่าน" แล้วสแกนซ้ำได้ "ใช้แล้ว" · dashboard ตัวเลขเปลี่ยนภายใน 10 วินาที
- [ ] **Step 4:** Commit `feat(admin): staff login, gate scanner and live dashboard`

### Task 10: Seed + demo script + ทดสอบจากเครื่องเปล่า

**อ้างอิง:** [Plan 16](2026-11-11-16-infra.md) Task 5 (seed) · contracts §7
**ผลลัพธ์:** `make seed` เรียก `POST /internal/seed` ตามลำดับ identity → member → commerce → ticket · `make demo-reset CONFIRM=yes` (`docker compose down -v` + up + seed) · `deploy/seed/fixtures.json` · ไฟล์ `docs/demo-script.md`: ขั้นตอนการ demo ทีละขั้น (URL ทุกหน้า, บัญชี, ปุ่มที่กด, สิ่งที่ลูกค้าจะเห็น, แผนสำรองถ้าขั้นใดล่ม) + เวลาเริ่มก่อน demo
- seed ออเดอร์ย้อนหลังวันนี้เล็กน้อย (เช่น 12 ออเดอร์, 30 บัตร, 8 สแกน) ให้ dashboard ไม่ว่าง

- [ ] **Step 1:** seed + reset
- [ ] **Step 2:** ทดสอบจากเครื่องเปล่า: `make demo-reset CONFIRM=yes` แล้วรัน `make e2e APP=web` ผ่าน และเดินตาม `docs/demo-script.md` ด้วย curl / Playwright ครบทุกขั้น บันทึกเวลาที่ใช้
- [ ] **Step 3:** Commit `chore(demo): seed, reset and demo script`
