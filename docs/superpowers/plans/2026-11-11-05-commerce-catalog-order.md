# Commerce: Catalog, Inventory, Order Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** service `commerce` ส่วนแรก: รับราคา / สินค้า / โค้ดส่วนลด / รอบการแสดง / โควตาที่อนุมัติแล้วจาก `price.approved`, คิดราคาไทย / ต่างชาติรายใบพร้อม Pre-sale และโค้ดส่วนลด, จองโควตารายวันและที่นั่งต่อรอบแบบไม่ขายเกิน 15 นาที และสร้างออเดอร์หลายใบที่ระบุผู้ใช้บัตรรายใบ (สมาชิก / ผู้ติดตาม / Guest, Upsell, บัตรรายปี + ของขวัญ)

**Architecture:** Go service `services/commerce` (schema `commerce`) บน chassis (Plan 01) แยกแพ็กเกจตามความรับผิดชอบ: `catalog` (ข้อมูลจาก `price.approved`), `pricing` (ฟังก์ชันบริสุทธิ์ ไม่แตะ DB), `inventory` (ตัวนับ held / sold ต่อ bucket ด้วย `UPDATE … WHERE capacity - held - sold >= ?` แบบ atomic), `order` (สร้างออเดอร์ + job หมดเวลา) ราคาไทยตรวจสิทธิ์ผ่าน gRPC `MemberService` (contracts §4) ทุกการเปลี่ยนสถานะที่ต้องแจ้ง service อื่นเขียน outbox ใน transaction เดียวกัน Plan 06 ต่อยอดด้วย Payment / Refund / Waiting room โดยใช้ฟังก์ชันที่ Task 3–5 ประกาศไว้

**Tech Stack:** Go 1.23 + chassis (`config`, `db`, `events`, `outbox`, `consumer`, `server`), `pkg/chassis/authn` (Plan 12 Task 1), `google.golang.org/grpc` + stub จาก `make gen`, `github.com/google/uuid`, MariaDB 10.11

**Spec:** [docs/system-design.md](../../system-design.md) F3, S3 ขั้น 1–3, หัวข้อ 1.2 (Commerce), 2 · [ROADMAP.md](../../../roadmap/ROADMAP.md) 4.2 E2 · [contracts](2026-11-11-contracts.md) · ขึ้นกับ [Plan 01](2026-11-11-01-platform-chassis.md), Plan 12 Task 1 (`authn`) และ proto ของ Plan 03 (ถ้ายังไม่มี Task 4 สร้าง `api/proto/member/v1/member.proto` ตาม contracts §4 ทุกตัวอักษร)

## Global Constraints

- **Build และ test ใน Docker เสมอ (มติ 11 พ.ย. 2026):** ห้ามรัน `go` / `npm` / `flutter` บนเครื่อง ใช้ `make test`, `make test-int`, `make build SVC=commerce`, `make gen`, `make seed` เท่านั้น integration test ขึ้นต้นด้วย `//go:build integration`
- **โหมด Demo:** ระบบภายนอกทุกตัวใช้ mockext / Mailpit ตาม [Plan 00](2026-11-11-00-mock-external.md) เรียกผ่าน interface + base URL ใน env (แผนนี้ไม่เรียกระบบภายนอก ส่วน KBank อยู่ใน Plan 06)
- **ทำตาม [contracts](2026-11-11-contracts.md) ห้ามแก้เอง** ข้อที่ต้องเปลี่ยนอยู่ในหัวข้อ "ข้อเสนอแก้ contracts" ท้ายแผน
- ทุก service ที่มีข้อมูลต้องมี `POST /internal/seed` ทำงานเมื่อ `DEMO_MODE=true` เท่านั้น และรันซ้ำได้ (Task 6)
- ข้อกำหนดทั้งหมดใน Global Constraints ของ Plan 01 (1 schema ต่อ service, outbox ใน tx เดียวกัน, consumer idempotent, OTel)
- เงิน `int64` สตางค์ field ลงท้าย `Satang` · เวลา UTC RFC3339 · `visitDate` = `YYYY-MM-DD` ตาม Asia/Bangkok · ID เป็น UUIDv7 · `orderNo` = `NN` + `YYMMDD` + 6 หลัก · error `{"error":"snake_case","message"}` · list `?cursor=&limit=` (20 / สูงสุด 100)
- ข้ามบริการส่งได้แค่ `idLast4` และ `nationality` ห้ามรับหรือเก็บเลขบัตร / พาสปอร์ตเต็มใน commerce
- E2: "จองโควตาชั่วคราว 15 นาทีระหว่างชำระเงิน, ปล่อยคืนเมื่อหมดเวลา"
- E2: "ราคาคนไทยซื้อได้เฉพาะบัญชีที่ยืนยันด้วยเลขบัตรประชาชน (Guest ซื้อได้เฉพาะราคาต่างชาติ [รอยืนยัน]), บัตรราคาคนไทยแต่ละใบต้องระบุผู้ใช้ที่เป็นคนไทย"
- E2: "ซื้อหลายใบในคำสั่งซื้อเดียว แต่ละใบระบุชื่อ-นามสกุลและเลขบัตรประชาชน / พาสปอร์ตของผู้ใช้ (ใช้เลือกราคาไทย / ต่างชาติรายใบ)"
- E2: "ตั้งจำนวนที่นั่งต่อรอบใน CMS (POC ใช้ 1,500 ที่ / รอบ [รอยืนยัน]), แสดงที่นั่งคงเหลือ, แจ้งทันทีเมื่อรอบเต็ม, เจ้าหน้าที่เห็นยอดจองต่อรอบแบบ real-time"
- E2: Upsell "เสนอหลังเลือกบัตรเข้าสวน, ผูกกับวันเข้าชม" · บัตรรายปี "ต้องมีบัญชีสมาชิก" · ของขวัญ "ระบุอีเมล / เบอร์ผู้รับ, ผู้รับเปิดใช้บัตรด้วยบัญชีสมาชิกของตัวเอง"
- E2: "ราคาพิเศษใช้ได้เฉพาะช่วงวันที่กำหนด, เปิดปิดจาก CMS" · E6: "ตั้งราคา โปรโมชั่น โค้ดส่วนลด … ทุกการเปลี่ยนต้องผ่านผู้อนุมัติก่อนมีผล" (commerce ไม่มี API แก้ราคาเอง รับจาก `price.approved` อย่างเดียว)
- E2: "ยังซื้อบัตรรายวันแบบ Guest ได้ และผูกออเดอร์เข้าบัญชีภายหลังได้ด้วยอีเมล / เบอร์เดิม"
- Route สาธารณะตาม contracts หัวข้อ 2: `GET /products` (รวม `/products/*`), `GET /show-rounds` และ `POST /orders` แบบ guest (gateway ตรวจ `X-Queue-Token` ก่อน) · `guestContact` เป็น E.164 หรืออีเมลตัวพิมพ์เล็ก (contracts หัวข้อ 1)

**ค่าที่แผนนี้กำหนดเพิ่ม เพราะ spec ไม่ได้ระบุ (ให้ PO ยืนยันใน review):** 1 ออเดอร์มี `visitDate` เดียว, สูงสุด 20 รายการต่อออเดอร์, ผู้ถือราคาไทยห้ามซ้ำกันในออเดอร์เดียว (บัตรเข้าสวน / รายปี), วันที่ไม่มีแถวโควตาถือว่า "ยังไม่เปิดขาย", เลือก PriceRule ด้วย priority สูงสุดก่อน แล้วราคาต่ำสุด, ส่วนลดเปอร์เซ็นต์ปัดเศษลง (floor) รายใบ, `Idempotency-Key` จำ 24 ชม., job หมดเวลาทำงานทุก 30 วินาที, บัตรรายปีแบบของขวัญคิดราคาต่างชาติ (ยังไม่รู้ผู้ถือ)

## Review Focus

1. **แย่งที่นั่งรอบสุดท้าย:** 50 คนกดจองพร้อมกันเมื่อเหลือ 10 ที่ ต้องสำเร็จ 10 คนพอดี ไม่มีค่า held + sold เกิน capacity และออเดอร์ที่ต้องใช้หลาย bucket ต้องจองได้ทั้งหมดหรือไม่ได้เลย (Task 3 `TestConcurrentHoldsNoOversell`, `TestHoldAllOrNothing`)
2. **ปลอมสิทธิ์ราคาไทย:** client ส่ง `audience:"thai"` กับผู้ถือแบบกรอกชื่อเอง, ผู้ติดตามที่ยังไม่ยืนยัน, Guest หรือใช้ตัวเองเป็นผู้ถือซ้ำหลายใบ ต้องได้ 422 และถ้า Member service ล่มต้องได้ 503 ไม่ใช่คิดราคาต่างชาติเงียบ ๆ หรือปล่อยราคาไทย (Task 4)
3. **`price.approved` มาซ้ำหรือสลับลำดับ:** การอนุมัติเก่าที่มาถึงทีหลังต้องไม่ทับราคาใหม่ และ change ที่ผิดรูปแบบตัวเดียวต้อง rollback ทั้ง event (Task 1 `TestApplyOlderApprovalIgnored`, `TestInvalidChangeRollsBackWholeEvent`)
4. **กดสั่งซื้อซ้ำ / เน็ตหลุดแล้วส่งใหม่:** `POST /orders` ด้วย `Idempotency-Key` เดิมต้องได้ออเดอร์เดิมและไม่จองโควตาซ้ำ (Task 4 `TestIdempotencyKeyReturnsSameOrder`)
5. **ขอบวันตามเวลาไทยและขอบช่วง Pre-sale:** `2027-03-01T17:30:00Z` คือวันที่ 2 มี.ค. ในไทย ซื้อของวันที่ 1 ต้องได้ `visit_date_past`, `saleTo` เป็นขอบเปิด (ณ วินาทีที่ `saleTo` ราคา Pre-sale หมดแล้ว) และวันที่ใน `orderNo` ใช้วันไทย (Task 2, Task 4)

---

## File Structure

```
services/commerce/main.go                    ประกอบ server, relay, consumer price.approved, job หมดเวลา, gRPC client
services/commerce/migrations/00001_catalog.sql   products, price_rules, promo_codes, show_rounds, inventory_buckets
services/commerce/migrations/00002_order.sql     orders, order_items, holds, promo_redemptions, idempotency_keys, doc_no_seq
services/commerce/core/core.go               DBTX, BangkokToday, ParseVisitDate, WriteError, Clock
services/commerce/catalog/changes.go         struct ของ price.approved + Validate
services/commerce/catalog/apply.go           Apply(tx, PriceApproved) + consumer handler
services/commerce/catalog/http.go            GET /products, POST /products/quote
services/commerce/pricing/pricing.go         Resolve, ApplyPromo (ไม่แตะ DB)
services/commerce/inventory/inventory.go     Hold, Release, Commit, Reacquire, Remaining
services/commerce/inventory/http.go          GET /products/availability, /show-rounds, /admin/inventory/rounds
services/commerce/clients/member.go          MemberClient (gRPC) + fake
services/commerce/docno/docno.go             Next(prefix) เลขเอกสารรายวัน
services/commerce/order/model.go             Order, Item, Holder, status, PaidEvent
services/commerce/order/create.go            Create (holder, ราคา, promo, hold) 
services/commerce/order/expire.go            Expire + Expirer job
services/commerce/order/http.go              POST /orders, GET /orders, GET /orders/{id}, POST /orders:claim, GET /admin/orders
services/commerce/seed/seed.go               POST /internal/seed + Register (Plan 06 เพิ่มส่วนของตัวเอง)
api/openapi/commerce.yaml
api/proto/member/v1/member.proto             (สร้างเมื่อยังไม่มี ตาม contracts §4)
deploy/compose.yaml                          (แก้) เพิ่ม commerce
```

---

### Task 1: Service skeleton + Catalog schema + consumer `price.approved`

**Files:**
- Create: `services/commerce/main.go`, `services/commerce/core/core.go`, `services/commerce/migrations/00001_catalog.sql`, `services/commerce/catalog/changes.go`, `services/commerce/catalog/apply.go`, `api/openapi/commerce.yaml` (ส่วน info + components/errors)
- Test: `services/commerce/core/core_test.go`, `services/commerce/catalog/changes_test.go`, `services/commerce/catalog/apply_test.go` (integration)

**Interfaces:**
- Consumes: `config.Load("commerce")`, `db.Open`, `db.Migrate`, `server.New`, `outbox.Relay`, `consumer.Run` (Config.Name = `commerce-price`, EventTypes = `[events.PriceApproved]`)
- Produces:
  - `core.DBTX` (interface ที่ `*sql.DB` และ `*sql.Tx` ใช้ได้: `ExecContext`, `QueryContext`, `QueryRowContext`)
  - `core.BangkokToday(now time.Time) string`, `core.ParseVisitDate(s string) (time.Time, error)` (ได้เที่ยงคืนเวลาไทย), `core.WriteError(w http.ResponseWriter, status int, code, msg string)`, `type Clock func() time.Time`
  - `catalog.Apply(ctx context.Context, tx *sql.Tx, ev PriceApproved) error` และ `catalog.Handler() consumer.Handler`
  - `catalog.RegisterEntity(entity string, validate func(op string, data json.RawMessage) error, apply func(ctx context.Context, tx *sql.Tx, approvalID, op string, data json.RawMessage) error)` ให้แพ็กเกจอื่นใน commerce รับ entity ของตัวเอง (Plan 06 Task 5 ลงทะเบียน `refund_policy`) · entity `point_rule` เป็นของ member (Plan 03) commerce **ข้ามเงียบ ๆ** (ไม่ error ไม่เขียน) เพราะ `price.approved` ส่งถึงทั้ง commerce และ member
  - **รูปแบบ `data` ของ `price.approved` (Plan 04 ต้องสร้างตามนี้ทุก field, JSON camelCase):**

| entity | `op: "upsert"` data | `op: "delete"` data | ผลใน commerce |
|---|---|---|---|
| `product` | `{productId, code, kind: "day"\|"annual"\|"restaurant"\|"show", ageGroup: "adult"\|"child"\|"any", names: {"th":"…","en":"…",…7 ภาษา (ขาดได้ ใช้ en แทน)}, sortOrder: int, active: bool}` | `{productId}` | delete = `active=false` (ออเดอร์เก่ายังอ้างได้) |
| `price_rule` | `{priceRuleId, productId, audience: "thai"\|"foreign", amountSatang: int64, label: "standard"\|"presale"\|"promo", priority: int, visitFrom: "YYYY-MM-DD"\|null, visitTo: "YYYY-MM-DD"\|null, saleFrom: RFC3339\|null, saleTo: RFC3339\|null}` | `{priceRuleId}` | delete = ลบแถว · `visitFrom/To` รวมขอบ · `saleFrom` รวม `saleTo` ไม่รวม · null = ไม่จำกัด |
| `promo_code` | `{promoCodeId, code, discountType: "percent"\|"fixed", value: int64 (1–100 หรือสตางค์), productIds: [..] (ว่าง = ทุกสินค้า), audiences: ["thai"\|"foreign"] (ว่าง = ทั้งคู่), saleFrom, saleTo, visitFrom?, visitTo?, maxUses: int (0 = ไม่จำกัด), active: bool}` | `{promoCodeId}` | `code` เก็บเป็นตัวพิมพ์ใหญ่ · delete = `active=false` |
| `show_round` | `{showRoundId, productId (kind show หรือ restaurant), visitDate, startsAt: RFC3339, capacity: int, active: bool}` | `{showRoundId}` | upsert ตั้ง capacity ของ bucket `round:<showRoundId>` · delete = `active=false` |
| `quota` | `{visitDate, capacity: int}` (โควตาบัตรเข้าสวนรายวัน นับเฉพาะ kind `day`) | `{visitDate}` | upsert ตั้ง capacity ของ bucket `day:<visitDate>` · delete = capacity 0 |

  - Go types ใน `changes.go`: `PriceApproved{ApprovalID string; Changes []Change}`, `Change{Entity, Op string; Data json.RawMessage}`, `ProductData`, `PriceRuleData`, `PromoCodeData`, `ShowRoundData`, `QuotaData` และ `func (c Change) Validate() error` (คืน `ErrUnknownEntity`, `ErrUnknownOp`, `ErrInvalidData` พร้อมชื่อ field)

ตาราง (ทุกตารางมีคอลัมน์ `approval_id CHAR(36) NOT NULL` เก็บ approvalId ล่าสุดที่เขียนแถวนั้น และ `updated_at DATETIME(6)`):

```sql
CREATE TABLE products (id CHAR(36) PRIMARY KEY, code VARCHAR(64) NOT NULL UNIQUE, kind VARCHAR(16) NOT NULL,
  age_group VARCHAR(8) NOT NULL, names JSON NOT NULL, sort_order INT NOT NULL, active BOOL NOT NULL, ...);
CREATE TABLE price_rules (id CHAR(36) PRIMARY KEY, product_id CHAR(36) NOT NULL, audience VARCHAR(8) NOT NULL,
  amount_satang BIGINT NOT NULL, label VARCHAR(16) NOT NULL, priority INT NOT NULL,
  visit_from DATE NULL, visit_to DATE NULL, sale_from DATETIME(6) NULL, sale_to DATETIME(6) NULL, ...,
  INDEX idx_product_audience (product_id, audience));
CREATE TABLE promo_codes (id CHAR(36) PRIMARY KEY, code VARCHAR(32) NOT NULL UNIQUE, discount_type VARCHAR(8) NOT NULL,
  value BIGINT NOT NULL, product_ids JSON NOT NULL, audiences JSON NOT NULL, sale_from DATETIME(6) NULL, sale_to DATETIME(6) NULL,
  visit_from DATE NULL, visit_to DATE NULL, max_uses INT NOT NULL, active BOOL NOT NULL, ...);
CREATE TABLE show_rounds (id CHAR(36) PRIMARY KEY, product_id CHAR(36) NOT NULL, visit_date DATE NOT NULL,
  starts_at DATETIME(6) NOT NULL, active BOOL NOT NULL, ..., INDEX idx_date (visit_date));
CREATE TABLE inventory_buckets (bucket_key VARCHAR(48) PRIMARY KEY,  -- "day:2027-03-01" | "round:<uuid>"
  capacity INT NOT NULL, held INT NOT NULL DEFAULT 0, sold INT NOT NULL DEFAULT 0, approval_id CHAR(36) NOT NULL,
  updated_at DATETIME(6) NOT NULL, CHECK (held >= 0 AND sold >= 0));
```

ไม่มี foreign key ระหว่างตาราง catalog เพื่อให้ลำดับ change ใน event ไม่มีผล

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestBangkokToday(t *testing.T)                 // 2027-03-01T16:59:59Z → "2027-03-01", 2027-03-01T17:00:00Z → "2027-03-02"
func TestValidateRejectsBadData(t *testing.T)       // kind "vip" → ErrInvalidData "kind", amountSatang -1, entity "hotel" → ErrUnknownEntity
func TestApplyProductUpsertAndDelete(t *testing.T)  // upsert แล้ว names.th อ่านกลับได้ "บัตรผู้ใหญ่" · delete → active=false แถวยังอยู่
func TestApplyOlderApprovalIgnored(t *testing.T)    // apply approval B (ใหม่กว่า) ราคา 30000 แล้ว A (เก่ากว่า) ราคา 25000 → ยังเป็น 30000
func TestInvalidChangeRollsBackWholeEvent(t *testing.T) // event 3 change ตัวที่ 3 ผิด → Handler คืน error และไม่มีแถวใดถูกเขียน
func TestQuotaAndRoundSetBucketCapacity(t *testing.T)   // quota 2027-03-01 = 5000 → bucket day:2027-03-01 capacity 5000; show_round capacity 1500 → round:<id> 1500
func TestDuplicateEventAppliedOnce(t *testing.T)    // publish envelope id เดียวกัน 2 ครั้งผ่าน consumer.Run → apply 1 ครั้ง (นับจาก updated_at ไม่เปลี่ยน)
func TestPointRuleIgnored(t *testing.T)             // event มี point_rule + product → product ถูกเขียน, point_rule ไม่ error
func TestRegisteredEntityApplied(t *testing.T)      // RegisterEntity("refund_policy", …) → apply ถูกเรียกใน tx เดียวกัน, error ของมัน rollback ทั้ง event
```

- [ ] **Step 2: รันให้ fail** — `make test PKG=./services/commerce/core/... ./services/commerce/catalog/...` และ `make test-int PKG=./services/commerce/catalog/...`
- [ ] **Step 3: เขียน `core`, `changes.go`, migration และ `Apply`**: validate ทุก change ก่อนเขียน, upsert ด้วย `INSERT … ON DUPLICATE KEY UPDATE col = IF(VALUES(approval_id) >= approval_id, VALUES(col), col)` (UUIDv7 เทียบแบบ string เรียงตามเวลา), quota / show_round เขียน `inventory_buckets.capacity` ใน tx เดียวกัน (ไม่แตะ held / sold)
- [ ] **Step 4: เขียน `main.go`**: config → `db.Open` → `db.Migrate(migrations)` → `server.New` → goroutine `outbox.Relay.Run`, `consumer.Run(commerce-price)` → `Run` · env เพิ่ม `DEMO_MODE` (ค่าเริ่ม false)
- [ ] **Step 5: รันให้ผ่าน** `-race`
- [ ] **Step 6: Commit** — `git commit -m "feat(commerce): service skeleton and price.approved catalog apply"`

### Task 2: Pricing engine + Catalog API

**Files:**
- Create: `services/commerce/pricing/pricing.go`, `services/commerce/catalog/http.go`
- Modify: `api/openapi/commerce.yaml`
- Test: `services/commerce/pricing/pricing_test.go`, `services/commerce/catalog/http_test.go` (integration)

**Interfaces:**
- Consumes: ตาราง catalog จาก Task 1, `core.BangkokToday`, `core.ParseVisitDate`
- Produces:
  - `pricing.Rule{ID, ProductID, Audience, Label string; AmountSatang int64; Priority int; VisitFrom, VisitTo string; SaleFrom, SaleTo *time.Time}`
  - `pricing.Resolve(rules []Rule, productID, audience, visitDate string, now time.Time) (Rule, error)` คืน `ErrNoPrice` เมื่อไม่มีกติกาตรง · เลือก priority สูงสุด → `AmountSatang` ต่ำสุด → `ID` มากสุด
  - `pricing.Promo{ID, Code, DiscountType string; Value int64; ProductIDs, Audiences []string; SaleFrom, SaleTo *time.Time; VisitFrom, VisitTo string; MaxUses int; Active bool}`
  - `pricing.Line{ProductID, Audience, VisitDate string; ListSatang, DiscountSatang, PriceSatang int64}`
  - `pricing.ApplyPromo(p Promo, lines []Line, now time.Time) ([]Line, error)` error `ErrPromoInactive`, `ErrPromoExpired` (นอกช่วงขาย), `ErrPromoNotApplicable` (ไม่มีรายการใดเข้าเงื่อนไข) · ส่วนลดคิดรายใบ `percent` = `floor(List × value / 100)`, `fixed` = `min(value, List)` ราคาไม่ต่ำกว่า 0
  - `catalog.LoadRules(ctx, q core.DBTX, productIDs []string) ([]pricing.Rule, error)`, `catalog.LoadPromo(ctx, q core.DBTX, code string, forUpdate bool) (pricing.Promo, error)` (`ErrPromoNotFound`), `catalog.GetProducts(ctx, q core.DBTX, ids []string) (map[string]Product, error)`, `catalog.Product{ID, Code, Kind, AgeGroup string; Names map[string]string; Active bool}`
  - HTTP (สาธารณะ):
    - `GET /products?visitDate=&lang=` → `{"items":[{productId, kind, ageGroup, name, thaiSatang?, foreignSatang?, label}]}` เฉพาะ `active` เรียง `sortOrder` · `name` ตาม `lang` ถ้าไม่มีใช้ `en` · ราคาไหนไม่มีกติกาให้ละ field
    - `POST /products/quote {visitDate, promoCode?, items:[{productId, audience}]}` → `{items:[Line], subtotalSatang, discountSatang, totalSatang}` (ไม่ตรวจผู้ถือ ไม่จองโควตา) error `422 price_unavailable|promo_not_found|promo_expired|promo_not_applicable|visit_date_past`
  - HTTP (admin, อ่านอย่างเดียว ให้ Plan 14 แสดงค่าปัจจุบันก่อนส่งแก้ผ่าน approval ของ content): `GET /admin/products`, `/admin/price-rules?productId=`, `/admin/promo-codes`, `/admin/show-rounds?visitDate=`, `/admin/quotas?from=&to=` (role `admin|editor|approver|finance|viewer`) ตอบ `{items, nextCursor}` โดยแต่ละ item มีรูปเดียวกับ `data` ของ entity นั้นใน `price.approved` (Task 1) + `updatedAt`, `approvalId` · ไม่มี endpoint แก้ไขใน commerce

- [ ] **Step 1: เขียน test ที่ fail** แบบ table-driven

```go
// rules: adult thai 30000 standard p0, adult foreign 60000 standard p0,
//        adult thai 25000 presale p10 saleFrom 2027-02-01T00:00:00Z saleTo 2027-03-01T00:00:00Z
{"adult","thai","2027-03-05", "2027-02-28T23:59:59Z", 25000, nil},
{"adult","thai","2027-03-05", "2027-03-01T00:00:00Z", 30000, nil},   // saleTo ไม่รวมขอบ
{"adult","foreign","2027-03-05","2027-02-15T00:00:00Z", 60000, nil},
{"child","thai","2027-03-05", "2027-02-15T00:00:00Z", 0, ErrNoPrice},
// visitFrom 2027-04-13 visitTo 2027-04-15 (รวมขอบ) ใช้ได้วันที่ 13 และ 15 ไม่ได้วันที่ 16
```

```go
func TestPromoPercentFloor(t *testing.T)     // 10% ของ 60000 → 6000, 33% ของ 25000 → 8250
func TestPromoFixedNotBelowZero(t *testing.T) // fixed 50000 กับ 30000 → discount 30000 price 0
func TestPromoProductFilter(t *testing.T)    // productIds=[show] → เฉพาะบรรทัด show ได้ส่วนลด, ไม่มี show เลย → ErrPromoNotApplicable
func TestPromoOutsideWindow(t *testing.T)    // now >= saleTo → ErrPromoExpired
func TestProductsEndpointLangFallback(t *testing.T) // lang=ko ไม่มีชื่อ ko → ได้ชื่อ en, inactive ไม่แสดง
func TestQuotePastDate422(t *testing.T)      // now 2027-03-01T17:30:00Z, visitDate 2027-03-01 → 422 visit_date_past
func TestAdminCatalogReadOnly(t *testing.T)  // GET /admin/price-rules role editor → 200 รูปตาม data ของ price.approved + approvalId; ไม่มี role → 403; PUT/POST → 405
```

- [ ] **Step 2: รันให้ fail** — `make test PKG=./services/commerce/pricing/...` และ `make test-int PKG=./services/commerce/catalog/...`
- [ ] **Step 3: เขียน `pricing` (ไม่มี I/O) และ loader + handler** (`catalog.Handler` ใช้ `Clock` ที่ inject ได้)
- [ ] **Step 4: รันให้ผ่าน**
- [ ] **Step 5: Commit** — `git commit -m "feat(commerce): pricing engine with presale and promo codes"`

### Task 3: Inventory (โควตารายวัน + ที่นั่งต่อรอบ, กันขายเกิน)

**Files:**
- Create: `services/commerce/inventory/inventory.go`, `services/commerce/inventory/http.go`, `services/commerce/migrations/00002_order.sql` (ส่วนตาราง `holds`)
- Modify: `api/openapi/commerce.yaml`
- Test: `services/commerce/inventory/inventory_test.go` (integration), `services/commerce/inventory/http_test.go` (integration)

**Interfaces:**
- Consumes: `inventory_buckets` จาก Task 1, `authn.RequireRole`
- Produces (Plan 06 ใช้ `Commit`, `Reacquire`, `Release`):
  - `inventory.DayKey(visitDate string) string` = `"day:"+visitDate`, `inventory.RoundKey(showRoundID string) string` = `"round:"+id`
  - `type Need struct { BucketKey string; Qty int }`
  - `inventory.Hold(ctx context.Context, tx *sql.Tx, orderID string, needs []Need, expiresAt time.Time) error` · error `*SoldOutError{BucketKey string; Remaining int}` (`errors.As`) หรือ `ErrNotSellable` (ไม่มี bucket)
  - `inventory.Release(ctx, tx, orderID string) error` (hold สถานะ `held` → `released`, `held -= qty`)
  - `inventory.Commit(ctx, tx, orderID string) error` (`held` → `committed`, `held -= qty`, `sold += qty`)
  - `inventory.Reacquire(ctx, tx, orderID string) error` (hold ที่ `released` → `committed` โดย `sold += qty` เฉพาะเมื่อ `capacity - held - sold >= qty` ทุก bucket ไม่งั้นคืน `*SoldOutError` และไม่เปลี่ยนอะไร) ใช้กับการจ่ายเงินที่มาหลังหมดเวลาจอง
  - `inventory.Remaining(ctx, q core.DBTX, keys []string) (map[string]int, error)` = `max(0, capacity - held - sold)`
  - HTTP: `GET /products/availability?from=&to=` (ไม่เกิน 62 วัน) → `{"items":[{visitDate, remaining}]}` เฉพาะวันที่มี bucket และ capacity > 0 · `GET /show-rounds?visitDate=` → `{"items":[{showRoundId, productId, kind, startsAt, capacity, remaining}]}` (เฉพาะ active) · `GET /admin/inventory/rounds?visitDate=` (role `admin|viewer|gate`) เพิ่ม `held, sold`

ตาราง:

```sql
CREATE TABLE holds (id CHAR(36) PRIMARY KEY, order_id CHAR(36) NOT NULL, bucket_key VARCHAR(48) NOT NULL, qty INT NOT NULL,
  status VARCHAR(12) NOT NULL,  -- held | released | committed
  expires_at DATETIME(6) NOT NULL, created_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_order_bucket (order_id, bucket_key)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

Hold: รวม `needs` ที่ key ซ้ำ → เรียง `BucketKey` (กัน deadlock) → ต่อ key: `UPDATE inventory_buckets SET held = held + ? WHERE bucket_key = ? AND capacity - held - sold >= ?` ถ้า affected = 0 ให้คืน `SoldOutError` (ผู้เรียก rollback tx ทั้งก้อน จึงได้ all-or-nothing) → insert `holds`

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestHoldDecrementsRemaining(t *testing.T)        // capacity 10, Hold 3 → Remaining 7
func TestHoldAllOrNothing(t *testing.T)               // day เหลือ 5, round เหลือ 0 → SoldOutError{round:…}, rollback แล้ว day ยังเหลือ 5
func TestConcurrentHoldsNoOversell(t *testing.T)      // capacity 10, 50 goroutine คนละ tx Hold qty 1 → สำเร็จ 10 พอดี, held = 10 (-race, -count=5)
func TestReleaseRestores(t *testing.T)                // Hold 3 → Release → Remaining 10, hold.status released, Release ซ้ำไม่ติดลบ
func TestCommitMovesHeldToSold(t *testing.T)          // held 3 sold 0 → held 0 sold 3
func TestReacquireAfterRelease(t *testing.T)          // released แล้วยังว่าง → committed sold 3 · ถ้าคนอื่นจองเต็มแล้ว → SoldOutError ไม่เปลี่ยนตัวนับ
func TestCapacityLoweredBelowUsed(t *testing.T)       // sold 8 แล้ว quota ใหม่ capacity 5 → Remaining 0, Hold ได้ SoldOutError, sold ยัง 8
func TestRoundsEndpoint(t *testing.T)                 // remaining แสดงถูก, /admin/inventory/rounds ไม่มี role → 403, role viewer → ได้ held/sold
func TestCalendarSkipsUnsellable(t *testing.T)        // วันที่ไม่มี quota ไม่อยู่ในผล
```

- [ ] **Step 2: รันให้ fail** — `make test-int PKG=./services/commerce/inventory/...`
- [ ] **Step 3: เขียน `inventory`** ตาม Interfaces (ใช้ conditional UPDATE ไม่ใช้ `SELECT … FOR UPDATE` บน bucket)
- [ ] **Step 4: รันให้ผ่าน** `-race -count=5`
- [ ] **Step 5: Commit** — `git commit -m "feat(commerce): inventory holds without oversell"`

### Task 4: สร้างออเดอร์ (`POST /orders`)

**Files:**
- Create: `services/commerce/clients/member.go`, `services/commerce/docno/docno.go`, `services/commerce/order/model.go`, `services/commerce/order/create.go`, `services/commerce/order/http.go`, `api/proto/member/v1/member.proto` (ถ้ายังไม่มี)
- Modify: `services/commerce/migrations/00002_order.sql`, `services/commerce/main.go` (gRPC client), `api/openapi/commerce.yaml`
- Test: `services/commerce/docno/docno_test.go` (integration), `services/commerce/order/create_test.go` (integration)

**Interfaces:**
- Consumes: `pricing.Resolve`, `pricing.ApplyPromo`, `catalog.LoadRules`, `catalog.LoadPromo`, `catalog.GetProducts`, `inventory.Hold`, `inventory.DayKey`, `inventory.RoundKey`, `authn.From(ctx) Principal`, gRPC `MemberService.GetMemberByContact`, `GetCompanion` (contracts §4)
- Produces:
  - `clients.MemberInfo{MemberID, Name, Nationality, IDLast4 string; ThaiVerified bool}`, `clients.HolderInfo{Name, Nationality, IDLast4 string; ThaiVerified bool}`
  - `type clients.MemberClient interface { GetMemberByContact(ctx, contact string) (MemberInfo, error); GetCompanion(ctx, memberID, companionID string) (HolderInfo, error) }` + `NewMemberGRPC(addr string)` (env `MEMBER_GRPC_ADDR`, ค่าเริ่ม `member:9090`, timeout 2 วินาที) + `clients.FakeMember` สำหรับ test · error `clients.ErrNotFound`, `clients.ErrUnavailable`
  - `docno.Next(ctx, tx *sql.Tx, prefix string, now time.Time) (string, error)` → `prefix + YYMMDD(ไทย) + %06d` ตัวนับแยกตาม prefix + วัน (`INSERT … ON DUPLICATE KEY UPDATE last = LAST_INSERT_ID(last + 1)`) · Plan 06 ใช้ `RC`, `TI`
  - สถานะ: `order.StatusPendingPayment = "pending_payment"`, `StatusConfirmed = "confirmed"`, `StatusExpired = "expired"`, `StatusRefundRequired = "refund_required"`, `StatusRefunded = "refunded"`
  - `order.Order{ID, OrderNo, Status, Lang, MemberID, GuestContact, VisitDate, PromoCode, LastPaymentStatus string; UTM *UTM; SubtotalSatang, DiscountSatang, TotalSatang int64; HoldExpiresAt, CreatedAt time.Time; Items []Item}`
  - `order.Item{ID, ProductID, Kind, Audience, VisitDate, ShowRoundID, GiftContact, GiftName string; Holder Holder; ListSatang, DiscountSatang, PriceSatang int64}`, `order.Holder{Ref /* "self" | "companion:<id>" | "named" */, Name, Nationality, IDLast4 string}`
  - `order.UTM{Source, Medium, Campaign string}` (JSON `{source, medium, campaign}`)
  - `order.CreateInput{VisitDate, Lang, PromoCode, GuestContact, IdempotencyKey string; UTM *UTM; Items []ItemInput}` (`utm` มาจาก query ของหน้าแรกที่ผู้ซื้อเข้าเว็บ / แอป ส่งต่อให้ Report ผ่าน `order.paid`), `ItemInput{ProductID, Audience, ShowRoundID string; Holder HolderInput; Gift *GiftInput}`, `HolderInput{Type /* self|companion|named */, CompanionID, Name, Nationality, IDLast4 string}`, `GiftInput{Contact, Name string}`
  - `func (s *Service) Create(ctx context.Context, p authn.Principal, in CreateInput) (Order, string /*accessKey ของ Guest*/, error)`
  - HTTP: `POST /orders` header `Idempotency-Key?` body ตาม `CreateInput` (camelCase) → `201 {orderId, orderNo, status, holdExpiresAt, subtotalSatang, discountSatang, totalSatang, accessKey?, items:[…]}` · error `400 invalid_request|invalid_contact`, `401 member_required`, `404 product_not_found|round_not_found`, `409 sold_out {bucket, remaining}`, `422 thai_price_not_eligible|guest_thai_price_forbidden|duplicate_holder|upsell_requires_entry|round_mismatch|visit_date_past|visit_date_not_sellable|price_unavailable|promo_not_found|promo_expired|promo_not_applicable|promo_exhausted|too_many_items|gift_not_allowed`, `503 member_unavailable`

กติกาของ `Create` (ตรวจตามลำดับนี้ ข้อแรกที่ไม่ผ่านเป็น error):
1. ผู้ซื้อ: `X-Member-Id` มีค่า = สมาชิก · ไม่มี JWT หรือ `typ=user` ที่ `mid` ว่าง = Guest (ใช้ `X-User-Sub` เป็น `guestContact` ถ้ามี ไม่งั้นต้องส่ง `guestContact` เป็น E.164 หรืออีเมล)
2. รายการ 1–20 (`too_many_items`) · `visitDate` บังคับเมื่อมี kind `day|restaurant|show` · ต้องไม่ก่อน `core.BangkokToday(now)`
3. Guest: ทุกใบ `audience=foreign` (`guest_thai_price_forbidden`), ผู้ถือเป็น `named` เท่านั้น, ห้ามมี `annual` (`401 member_required`)
4. ผู้ถือ: `self` → `GetMemberByContact(X-User-Sub)` · `companion` → `GetCompanion(memberId, companionId)` · `named` → ใช้ชื่อที่ส่งมา (`ThaiVerified=false`) · gRPC ล่ม → `503 member_unavailable`
5. `audience=thai` ต้องมีผู้ถือ `ThaiVerified=true` (`thai_price_not_eligible`) และผู้ถือราคาไทยแบบ `self` / `companion:<id>` ห้ามซ้ำในบัตร kind `day|annual` (`duplicate_holder`)
6. `restaurant|show` ต้องมีบัตร `day` หรือ `annual` ในออเดอร์ (`upsell_requires_entry`) และต้องมี `showRoundId` ที่ active ของ product นั้นในวัน `visitDate` (`round_mismatch`)
7. `gift` ใช้ได้กับ `annual` เท่านั้น (`gift_not_allowed`) ผู้ถือ = `gift.name` และคิดราคา `foreign`
8. คิดราคาทุกใบด้วย `pricing.Resolve` → promo (`LoadPromo(forUpdate=true)` + นับ `promo_redemptions` ที่ยังไม่ `released` เทียบ `maxUses`) → `inventory.Hold` (`day:<visitDate>` ต่อบัตร `day`, `round:<id>` ต่อบัตร upsell, `annual` ไม่ใช้โควตา) หมดอายุ `now + HOLD_TTL` (env ค่าเริ่ม `15m` ปรับได้เฉพาะ demo / test) → insert `orders`, `order_items`, `promo_redemptions` และ `idempotency_keys` ใน tx เดียว
9. `Idempotency-Key` เดิมจากผู้ซื้อเดิม (memberId หรือ guestContact) ภายใน 24 ชม. → คืนออเดอร์เดิม (`200`) ไม่สร้างใหม่ (contracts หัวข้อ 2) · เกิน 24 ชม. ถือเป็น key ใหม่

ตารางเพิ่มใน `00002_order.sql`: `orders` (มี `access_key_hash BINARY(32) NULL` เก็บ SHA-256 ของ accessKey, `last_payment_status VARCHAR(16) NULL`, `utm JSON NULL`, INDEX `(status, hold_expires_at)`, `(member_id, created_at)`, `(guest_contact)`, UNIQUE `order_no`), `order_items`, `promo_redemptions (order_id, promo_code_id, status held|released)`, `idempotency_keys (owner, idem_key, order_id, created_at, PK(owner, idem_key))`, `doc_no_seq (prefix, day, last, PK(prefix, day))`

- [ ] **Step 1: เขียน test ที่ fail** (ใช้ `clients.FakeMember`, `Clock` คงที่ `2027-02-28T03:00:00Z`)

```go
func TestOrderNoFormat(t *testing.T)                 // docno.Next("NN", 2027-02-28T17:30:00Z) → "NN270301000001" แล้ว "NN270301000002"
func TestMemberThaiSelfGetsThaiPrice(t *testing.T)   // self ThaiVerified → priceSatang 30000, holder.idLast4 "1234", total ถูก
func TestNamedHolderThaiRejected(t *testing.T)       // named + thai → 422 thai_price_not_eligible ไม่มีแถว orders / holds
func TestCompanionNotVerifiedThaiRejected(t *testing.T)
func TestDuplicateThaiHolder(t *testing.T)           // self thai 2 ใบ day → 422 duplicate_holder · self thai day + self show → ผ่าน
func TestGuestForeignOnly(t *testing.T)              // guest foreign → 201 + accessKey ยาว ≥ 32 ตัวอักษร, guest thai → 422 guest_thai_price_forbidden
func TestGuestAnnualRequiresMember(t *testing.T)     // 401 member_required
func TestUpsellRequiresEntry(t *testing.T)           // show อย่างเดียว → 422 upsell_requires_entry, round ของวันอื่น → 422 round_mismatch
func TestSoldOutNoPartialHold(t *testing.T)          // round เต็ม → 409 sold_out {bucket:"round:…"}, bucket day ไม่เปลี่ยน
func TestMemberServiceDown503(t *testing.T)          // fake คืน ErrUnavailable → 503 member_unavailable
func TestIdempotencyKeyReturnsSameOrder(t *testing.T) // ส่งซ้ำ key เดิม → 200 orderId เดิม, held ไม่เพิ่ม · key เดิมจากผู้ซื้ออื่น → ออเดอร์ใหม่ · เลื่อนนาฬิกา 24h 1s → ออเดอร์ใหม่
func TestPromoAppliedAndExhausted(t *testing.T)      // maxUses 1: ออเดอร์แรกได้ส่วนลด, ออเดอร์ที่สอง 422 promo_exhausted
func TestAnnualGiftForeignPrice(t *testing.T)        // annual + gift{contact:"friend@example.com"} → audience foreign, giftContact เก็บไว้, ไม่มี hold
func TestMaxItems20(t *testing.T)                    // 21 ใบ → 422 too_many_items
func TestHoldExpiresIn15Min(t *testing.T)            // holdExpiresAt == now + 15m
```

- [ ] **Step 2: รันให้ fail** — `make test-int PKG=./services/commerce/order/... ./services/commerce/docno/...`
- [ ] **Step 3: สร้าง proto ถ้ายังไม่มี แล้ว `make gen`** (ใช้ stub `gen/go/member/v1`)
- [ ] **Step 4: เขียน `clients`, `docno`, `order.Create`, handler** ตามกติกาข้างบน (เรียก gRPC ให้เสร็จก่อน `BEGIN` เพื่อไม่ถือ lock ระหว่างรอเครือข่าย)
- [ ] **Step 5: รันให้ผ่าน** `-race`
- [ ] **Step 6: Commit** — `git commit -m "feat(commerce): create multi-ticket orders with per-holder pricing"`

### Task 5: หมดเวลาจอง (`order.expired`) + อ่าน / ผูกออเดอร์

**Files:**
- Create: `services/commerce/order/expire.go`
- Modify: `services/commerce/order/model.go`, `services/commerce/order/http.go`, `services/commerce/main.go` (เริ่ม Expirer), `api/openapi/commerce.yaml`
- Test: `services/commerce/order/expire_test.go` (integration), `services/commerce/order/read_test.go` (integration)

**Interfaces:**
- Consumes: `inventory.Release`, `outbox.Write`, `events.New(events.OrderExpired, …)`, `authn.From`
- Produces (Plan 06 ใช้):
  - `order.Get(ctx, q core.DBTX, id string) (Order, error)` (`ErrNotFound`), `order.GetForUpdate(ctx, tx *sql.Tx, id string) (Order, error)`
  - `order.SetStatus(ctx, tx, id, status string) error`, `order.SetLastPaymentStatus(ctx, tx, id, s string) error`
  - `order.Expire(ctx, tx *sql.Tx, id, reason string) error`: ทำเฉพาะเมื่อ status = `pending_payment` (ไม่งั้นคืน `ErrNotPending`) → `inventory.Release` → `promo_redemptions` เป็น `released` → status `expired` → `outbox.Write` `order.expired {orderId, reason}` (`reason` ∈ `hold_timeout|payment_failed`)
  - `order.Expirer{DB *sql.DB; Now core.Clock; Interval time.Duration /*30s*/; Batch int /*100*/}` มี `Tick(ctx) (int, error)` และ `Run(ctx) error` · Tick: `SELECT id … WHERE status='pending_payment' AND hold_expires_at <= now ORDER BY hold_expires_at LIMIT ? FOR UPDATE SKIP LOCKED` แล้ว `Expire` ทีละแถว โดย reason = `payment_failed` เมื่อ `last_payment_status = 'failed'` ไม่งั้น `hold_timeout`
  - `order.PaidPayload` (JSON ตาม contracts §5 `order.paid`) และ `order.PaidEvent(o Order, paymentID string, paidSatang int64) (events.Envelope, error)`: item ละ 1 รายการ, `holder.thaiPrice = (Audience == "thai")`, `visitDate` ว่างสำหรับ `annual`, `memberId` / `guestContact` / `promoCode` / `utm` ใส่เฉพาะที่มี, item ของขวัญใส่ `gift: {contact, name?}` จาก `GiftContact` / `GiftName`
  - `order.CanRead(p authn.Principal, o Order, accessKey string) bool`: เจ้าของ (memberId ตรง) หรือ Guest ที่ส่ง `?key=` ตรง hash หรือ staff role `admin|finance|viewer|gate`
  - HTTP: `GET /orders/{id}?key=` → ออเดอร์ (Plan 06 เพิ่ม tickets) · `GET /orders?cursor=&limit=` (สมาชิก) · `POST /orders:claim` (typ=user ที่มี `mid`) ย้ายออเดอร์ที่ `guest_contact = X-User-Sub` มาเป็นของ memberId ตอบ `{claimed: n}` · `GET /admin/orders?q=&cursor=` (role `admin|finance|viewer|gate`) ค้นด้วย `orderNo` ตรงตัว, เบอร์ E.164 หรืออีเมล

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestExpireAfter15Min(t *testing.T)            // เลื่อนนาฬิกา 15m 1s → Tick == 1, status expired, bucket held กลับเป็น 0, outbox order.expired {reason:"hold_timeout"}
func TestExpireReasonPaymentFailed(t *testing.T)   // last_payment_status = failed → reason "payment_failed"
func TestExpireSkipsConfirmed(t *testing.T)        // status confirmed → Tick == 0, Expire คืน ErrNotPending
func TestTwoExpirersNoDoubleRelease(t *testing.T)  // 2 Expirer Tick พร้อมกัน 100 ออเดอร์ → order.expired 100 แถวพอดี, held ไม่ติดลบ
func TestPromoReleasedOnExpire(t *testing.T)       // maxUses 1: ออเดอร์แรกหมดเวลา → ออเดอร์ใหม่ใช้โค้ดได้
func TestPaidEventMatchesContract(t *testing.T)    // JSON มี key ครบตาม contracts §5 order.paid · promoCode "DEMO10", utm {source, medium, campaign} และ items[].gift {contact, name} ของบัตรรายปีของขวัญ · key optional ที่ว่างต้องไม่มีใน JSON
func TestGetOrderAccess(t *testing.T)              // เจ้าของ 200, สมาชิกอื่น 404, guest key ถูก 200 / ผิด 404, staff viewer 200
func TestClaimGuestOrders(t *testing.T)            // guest ซื้อด้วย +66812345678 → login เบอร์เดียวกัน claim → claimed 1 และเห็นใน GET /orders
func TestAdminSearch(t *testing.T)                 // ค้น "NN270301000001" และ "foo@example.com" เจอ, ไม่มี role → 403
```

- [ ] **Step 2: รันให้ fail** — `make test-int PKG=./services/commerce/order/...`
- [ ] **Step 3: เขียน `expire.go`, read / claim / admin handler** และเริ่ม `Expirer.Run` ใน `main.go`
- [ ] **Step 4: รันให้ผ่าน** `-race`
- [ ] **Step 5: Commit** — `git commit -m "feat(commerce): hold expiry job, order.expired, order read and claim"`

### Task 6: Seed + compose + ทดสอบปลายทาง

**Files:**
- Create: `services/commerce/seed/seed.go`, `services/commerce/seed/data.go`
- Modify: `services/commerce/main.go`, `deploy/compose.yaml` (เพิ่ม `commerce` build `deploy/Dockerfile.go` `args: {SVC: commerce}` env `DB_DSN`, `NATS_URL`, `MEMBER_GRPC_ADDR=member:9090`, `DEMO_MODE=true`, healthcheck `/healthz`), `api/openapi/commerce.yaml`
- Test: `services/commerce/seed/seed_test.go` (integration)

**Interfaces:**
- Consumes: `catalog.Apply` (seed สร้าง `PriceApproved` ที่มี `approvalId` คงที่ แล้วเรียก `Apply` ทำให้ใช้เส้นทางเดียวกับข้อมูลจริง)
- Produces:
  - `seed.Register(name string, fn func(ctx context.Context, db *sql.DB, now time.Time) error)` (Plan 06 ลงทะเบียน `refund_policy`, `queue`) และ `POST /internal/seed` → `200 {"seeded":["catalog",…]}` · `DEMO_MODE != "true"` → `404`
  - ข้อมูล demo (ID เป็น UUID คงที่อ่านจาก `deploy/seed/fixtures.json` ของ Plan 16 Task 5 ตาม contracts หัวข้อ 7 เพื่อรันซ้ำได้, ราคาเป็นค่าสมมติสำหรับ demo) · `make seed` เรียก commerce หลัง content และก่อน ticket:

| สินค้า (code) | kind / ageGroup | thai | foreign |
|---|---|---|---|
| `DAY-ADULT` | day / adult | 30000 | 60000 |
| `DAY-CHILD` | day / child | 15000 | 30000 |
| `ANNUAL` | annual / any | 150000 | 300000 |
| `LUNCH-BUFFET` | restaurant / any | 45000 | 45000 |
| `CULTURAL-SHOW` | show / any | 20000 | 20000 |

  - Pre-sale: `DAY-ADULT` thai 25000 / foreign 50000 priority 10 `saleFrom` = วันที่ seed `saleTo` = `2027-03-01T00:00:00Z` (ถ้าผ่านไปแล้วให้ใช้ seed + 30 วัน) · โค้ด `DEMO10` 10% ทุกสินค้า `maxUses` 0
  - quota 5000 ต่อวัน ตั้งแต่วันนี้ (ไทย) ถึง +120 วัน · รอบ `CULTURAL-SHOW` 11:00 และ 15:00 เวลาไทย capacity 1500 · รอบ `LUNCH-BUFFET` 12:00 capacity 200 ทุกวันในช่วงเดียวกัน

- [ ] **Step 1: เขียน test ที่ fail**: `TestSeedIdempotent` (เรียก 2 ครั้ง จำนวนแถวทุกตารางเท่าเดิม, `held/sold` ไม่ถูกรีเซ็ต), `TestSeedRefusedWhenNotDemo` (404), `TestSeedQuoteWorks` (หลัง seed `POST /products/quote` DAY-ADULT foreign + `DEMO10` ได้ total = ราคา × 0.9)
- [ ] **Step 2: รันให้ fail** — `make test-int PKG=./services/commerce/seed/...`
- [ ] **Step 3: เขียน `seed`, ลงทะเบียน route, เพิ่ม service ใน compose**
- [ ] **Step 4: รันให้ผ่าน** แล้ว `make test && make test-int PKG=./services/commerce/... && make build SVC=commerce`
- [ ] **Step 5: ทดสอบปลายทางบน local**: `make up && make seed` → `GET http://localhost:8000/api/commerce/products?visitDate=<พรุ่งนี้>&lang=zh` ได้ 5 รายการ → `GET /api/commerce/queue/status` ได้ `accessToken` → `POST /api/commerce/orders` แบบ Guest (header `X-Queue-Token`, `Idempotency-Key`; foreign 2 ใบ + show 1 ใบ) ได้ 201 → `GET /api/commerce/show-rounds?visitDate=…` remaining ลดลง 1 → รอ 15 นาที (หรือตั้ง `HOLD_TTL=1m` ใน compose override) remaining กลับมาเท่าเดิม และ `nats stream view EVENTS` มี `evt.order.expired`
- [ ] **Step 6: Commit** — `git commit -m "feat(commerce): demo seed and compose wiring"`

---

## ข้อเสนอแก้ contracts

รวมเข้า contracts v2 แล้ว (หัวข้อ 5 `order.paid.items[].gift?`, หัวข้อ 1 รูปแบบ `guestContact`)

## คำถามเปิด

- `POST /orders:claim` ย้ายออเดอร์ Guest เข้าบัญชี แต่บัตรที่ Ticket ออกไปแล้วยังไม่มี `memberId` ต้องมี event (`order.claimed`?) หรือให้ Ticket ค้นด้วย contact เอง
- ราคาเด็ก / ผู้ใหญ่ และเกณฑ์ส่วนสูง (ROADMAP #17) และ capacity 1,500 ที่ / รอบ ยังรอยืนยัน ค่าทั้งหมดมาจาก CMS จึงไม่กระทบโค้ด
