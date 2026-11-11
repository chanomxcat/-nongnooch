# Platform Chassis Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** สร้างแพ็กเกจ Go ที่ทุก microservice ใช้ร่วมกัน ได้แก่ config, DB + migration, outbox, outbox relay ไป NATS JetStream, consumer แบบ idempotent พร้อม dead-letter, HTTP + gRPC server พร้อม health check และ OpenTelemetry รวมถึงสภาพแวดล้อม local ด้วย Docker Compose

**Architecture:** เป็น Go module เดียว (`nongnooch/platform`) แบบ monorepo แพ็กเกจ `pkg/chassis/*` ใช้ร่วมกัน และ service แต่ละตัวอยู่ใต้ `services/<name>` event ทุกตัวเขียนลงตาราง `outbox` ใน transaction เดียวกับข้อมูล จากนั้น relay จะ publish ไป stream `EVENTS` (subject `evt.<event type>`) โดยใช้ event id เป็น `Nats-Msg-Id` เพื่อให้ JetStream กันข้อความซ้ำ ฝั่ง consumer บันทึก `processed_events` ใน transaction เดียวกับงานของ handler

**Tech Stack:** Go 1.23, MariaDB 10.6+ (`github.com/go-sql-driver/mysql`), `github.com/pressly/goose/v3`, `github.com/nats-io/nats.go` (แพ็กเกจ `jetstream`), `google.golang.org/grpc` + health, OpenTelemetry Go SDK (OTLP gRPC exporter), `github.com/google/uuid` (UUIDv7), Docker Compose

**Spec:** [docs/system-design.md](../../system-design.md) หัวข้อ 1.2, 1.3, 2 · ดัชนีแผน: [2026-11-11-phase1-index.md](2026-11-11-phase1-index.md)

## Global Constraints

- **Build และ test ใน Docker เสมอ (มติ 11 พ.ย. 2026):** ห้ามรัน `go build` / `go test` / `go run` บนเครื่องโดยตรง ให้ใช้ `make test`, `make test-int`, `make build` ที่รันใน container `golang:1.23-bookworm` คู่กับ MariaDB / NATS ใน `deploy/compose.yaml` (กำหนดใน Plan 01 Task 1)
- Backend เป็น **Go microservices** แต่ละ service "เป็นเจ้าของ schema ของตัวเองบน MariaDB cluster และ**ห้าม join หรืออ่าน schema ของ service อื่น**"
- "**MariaDB 10.6+ cluster** (InnoDB, `utf8mb4`), 1 schema ต่อ service"
- "ถ้าต้องรอคำตอบทันที เรียกกันด้วย **gRPC** … นอกนั้นสื่อสารด้วย **event ผ่าน NATS JetStream**"
- "event ต้องเขียนลง **outbox** ใน transaction เดียวกับข้อมูลก่อน แล้วจึง publish"
- "consumer ต้องเป็น **idempotent** เพราะ JetStream ส่งแบบ at-least-once"
- "ส่ง trace และ log ผ่าน **OpenTelemetry**"
- "ทุก schema มีตาราง `outbox` และ `processed_events` (สำหรับ idempotency)"
- Saga: "เกินจำนวนครั้งที่กำหนด → dead-letter + แจ้ง on-call" (S3)
- ชื่อ event ทั้ง 20 ตัวตาม contracts §5 (17 ตัวตาม O3 + `account.deleted`, `member.invite_added`, `member.consent_changed`) ใช้ตรงตามนี้เป็นชื่อ type

## Review Focus

1. **Relay หลาย instance พร้อมกัน:** ถ้ารัน 2 replica ต้องไม่ publish แถวเดียวกันซ้ำจนเกิดผลซ้ำ ใช้ `FOR UPDATE SKIP LOCKED` + `Nats-Msg-Id` (ทดสอบใน Task 4)
2. **NATS ล่มระหว่าง relay:** แถวที่ publish ไม่สำเร็จต้องยังเป็น unpublished และถูกส่งในรอบถัดไป ไม่หายไป (Task 4)
3. **Handler error หลัง insert processed_events:** ต้อง rollback ทั้งก้อน ไม่อย่างนั้นข้อความที่ redeliver มาจะถูกข้ามทั้งที่งานยังไม่ได้ทำ (Task 5)
4. **ข้อความพิษ (poison message):** handler fail ทุกครั้ง ต้องย้ายไป dead-letter หลังส่งครบ `MaxDeliver` ไม่ใช่วนส่งไม่จบ (Task 5)
5. **ข้อความภาษาไทย / จีน / emoji ใน payload:** ต้องผ่าน outbox → NATS → consumer ได้ไม่เพี้ยน ซึ่งขึ้นกับการตั้ง `utf8mb4` ใน DSN (Task 2, Task 5)

---

## File Structure

```
go.mod                                  module nongnooch/platform
Makefile                                up / down / test / test-int
deploy/compose.yaml                     mariadb:10.11, nats:2.10 -js, minio, otel-collector
deploy/mariadb-init.sql                 CREATE DATABASE ต่อ service + user แยกสิทธิ์
pkg/chassis/config/config.go            โหลด env ร่วม
pkg/chassis/db/db.go                    Open + Migrate (goose, embed FS)
pkg/chassis/db/migrations/00001_outbox.sql   outbox + processed_events (ทุก service ใช้ไฟล์นี้)
pkg/chassis/events/events.go            Envelope, ชื่อ event 20 ตัว, Subject()
pkg/chassis/outbox/outbox.go            Write(tx, ...) + Relay
pkg/chassis/consumer/consumer.go        Run(...) idempotent + DLQ
pkg/chassis/server/server.go            HTTP + gRPC + health + graceful shutdown
pkg/chassis/obs/obs.go                  OTel tracer/meter/logger + slog
api/proto/                              (proto ของ gRPC แต่ละ service เพิ่มในแผนของ service นั้น)
api/openapi/                            (OpenAPI ต่อ service)
services/example/main.go                service ตัวอย่างที่ใช้ chassis ครบทุกส่วน (ใช้เป็น smoke test)
```

---

### Task 1: Repo + Local environment

**Files:**
- Create: `go.mod`, `Makefile`, `deploy/compose.yaml`, `deploy/mariadb-init.sql`, `deploy/Dockerfile.go`, `.dockerignore`, `pkg/chassis/config/config.go`
- Test: `pkg/chassis/config/config_test.go`

**Interfaces:**
- Produces: `config.Base{ServiceName, HTTPAddr, GRPCAddr, DBDSN, NATSURL, OTLPEndpoint string}` และ `config.Load(service string) (Base, error)` อ่าน env `HTTP_ADDR` (ค่าเริ่ม `:8080`), `GRPC_ADDR` (`:9090`), `DB_DSN` (บังคับมี), `NATS_URL` (`nats://localhost:4222`), `OTEL_EXPORTER_OTLP_ENDPOINT` (ว่างได้ = ปิด export)

- Produces (Makefile ทุก target รันใน Docker ห้ามเรียก `go` บนเครื่อง):
  - `make up` / `make down`: `docker compose -f deploy/compose.yaml up -d --wait` (mariadb, nats, mockext, mailpit มี healthcheck ครบ)
  - `make test PKG=./... RUN=`: `docker compose run --rm gotest go test -race -count=1 -run "$(RUN)" $(PKG)` (unit test ไม่ต้องใช้ DB)
  - `make test-int PKG=./...`: เหมือน `make test` แต่ใส่ `-tags=integration` และ service `gotest` ต้อง `depends_on` mariadb / nats แบบ `condition: service_healthy` พร้อม env `TEST_DB_DSN=root:root@tcp(mariadb:3306)/test?parseTime=true&charset=utf8mb4`, `TEST_NATS_URL=nats://nats:4222`
  - `make build SVC=identity [REGISTRY= TAG=]`: `docker build -f deploy/Dockerfile.go --build-arg SVC=$(SVC) -t $(REGISTRY)/$(SVC):$(TAG) .` (`REGISTRY ?= nongnooch`, `TAG ?= dev` จึงได้ `nongnooch/identity:dev`, contracts §1)
  - service `gotest` ใน compose: image `golang:1.23-bookworm` (มี gcc สำหรับ `-race`), mount repo ที่ `/src`, volume `gomod:/go/pkg/mod` และ `gobuild:/root/.cache/go-build` เพื่อไม่ต้องโหลด module ซ้ำ, `working_dir: /src`, `profiles: [tools]` (ไม่ขึ้นตอน `make up`)
  - `deploy/Dockerfile.go`: multi-stage `golang:1.23-bookworm` → `gcr.io/distroless/static-debian12:nonroot`, `CGO_ENABLED=0`, ARG `SVC` เลือก `./services/$SVC`
  - integration test ทุกไฟล์ขึ้นต้นด้วย `//go:build integration`

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestLoadRequiresDSN(t *testing.T) {
    t.Setenv("DB_DSN", "")
    _, err := config.Load("identity")
    if err == nil || !strings.Contains(err.Error(), "DB_DSN") { t.Fatalf("want DB_DSN error, got %v", err) }
}
func TestLoadDefaults(t *testing.T) {
    t.Setenv("DB_DSN", "u:p@tcp(localhost:3306)/identity")
    c, _ := config.Load("identity")
    if c.HTTPAddr != ":8080" || c.GRPCAddr != ":9090" || c.ServiceName != "identity" { t.Fatal(c) }
}
```

- [ ] **Step 2: รันให้ fail** — `make test PKG=./pkg/chassis/config/...` ต้องได้ FAIL (ยังไม่มี package)
- [ ] **Step 3: เขียน `config.Load`** อ่านด้วย `os.Getenv` ตรง ๆ ไม่ใช้ไลบรารี config
- [ ] **Step 4: เขียน `deploy/compose.yaml`**: `mariadb:10.11` (ตั้ง `--character-set-server=utf8mb4 --collation-server=utf8mb4_unicode_ci`), `nats:2.10` (`-js -sd /data`), `minio/minio`, `otel/opentelemetry-collector`
- [ ] **Step 5: เขียน `deploy/mariadb-init.sql`** สร้าง database 8 ตัว ได้แก่ `identity member agent commerce ticket content notify report` และ user ชื่อเดียวกัน ให้สิทธิ์เฉพาะ `GRANT ALL ON <name>.*` เท่านั้น (บังคับกติกาห้ามอ่าน schema อื่นตั้งแต่ระดับ DB)
- [ ] **Step 6: เขียน `Makefile`, service `gotest`, `deploy/Dockerfile.go`** ตาม Produces
- [ ] **Step 7: ตรวจ** — `make up && make test PKG=./pkg/chassis/config/...` ต้อง PASS (รันใน container `gotest` เครื่องไม่ต้องติดตั้ง Go) และ `docker compose -f deploy/compose.yaml exec mariadb mariadb -uidentity -pidentity -e "SELECT 1 FROM commerce.dual"` ต้องได้ `Access denied`
- [ ] **Step 8: Commit** — `git commit -m "chore: repo skeleton, dockerized build/test, compose, config"`

### Task 2: DB + migration ร่วม (outbox, processed_events)

**Files:**
- Create: `pkg/chassis/db/db.go`, `pkg/chassis/db/migrations/00001_outbox.sql`
- Test: `pkg/chassis/db/db_test.go` (integration, ใช้ env `TEST_DB_DSN`)

**Interfaces:**
- Consumes: `config.Base.DBDSN`
- Produces: `db.Open(dsn string) (*sql.DB, error)` บังคับ `parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci&loc=UTC` และ `db.Migrate(ctx, *sql.DB, serviceFS fs.FS) error` รัน migration ของ chassis ก่อน แล้วตามด้วย migration ของ service (ใช้ goose table แยกกัน: `goose_chassis`, `goose_service`)

ตาราง (ค่าที่กำหนดในแผนนี้):

```sql
CREATE TABLE outbox (
  id CHAR(36) PRIMARY KEY,           -- UUIDv7 = event id
  type VARCHAR(64) NOT NULL,         -- เช่น order.paid
  payload LONGTEXT NOT NULL,         -- JSON ของ Envelope
  created_at DATETIME(6) NOT NULL,
  published_at DATETIME(6) NULL,
  INDEX idx_unpublished (published_at, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE processed_events (
  consumer VARCHAR(64) NOT NULL,
  event_id CHAR(36) NOT NULL,
  processed_at DATETIME(6) NOT NULL,
  PRIMARY KEY (consumer, event_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestMigrateCreatesChassisTables(t *testing.T) {
    d := testdb(t)                       // เปิด TEST_DB_DSN, ไม่มี env = t.Fatal (ห้าม skip)
    if err := db.Migrate(ctx, d, fstest.MapFS{}); err != nil { t.Fatal(err) }
    for _, tbl := range []string{"outbox", "processed_events"} { assertTableExists(t, d, tbl) }
}
func TestUTF8RoundTrip(t *testing.T) {
    // insert "สวนนงนุช 农努奇 🌺" ลง outbox.payload แล้วอ่านกลับต้องตรงทุก byte
}
func TestMigrateTwiceIsNoop(t *testing.T) { /* เรียก Migrate 2 ครั้ง ไม่ error */ }
```

- [ ] **Step 2: รันให้ fail** — `make test-int PKG=./pkg/chassis/db/...` → FAIL
- [ ] **Step 3: เขียน `Open` และ `Migrate`** ใช้ `//go:embed migrations/*.sql` และ `goose.NewProvider(goose.DialectMySQL, ...)` ร่วมกับ `goose.WithTableName`
- [ ] **Step 4: รันให้ผ่าน** — คำสั่งเดิมต้อง PASS ทั้ง 3 test
- [ ] **Step 5: Commit** — `git commit -m "feat(chassis): db open + shared outbox migrations"`

### Task 3: Event envelope + ชื่อ event

**Files:**
- Create: `pkg/chassis/events/events.go`
- Test: `pkg/chassis/events/events_test.go`

**Interfaces:**
- Produces:
  - `type Envelope struct { ID string; Type string; OccurredAt time.Time; Payload json.RawMessage }` (JSON tag: `id,type,occurredAt,payload`)
  - ค่าคงที่ 20 ตัวตาม contracts §5: `OTPRequested = "otp.requested"`, `MemberRegistered`, `ReferralCreated`, `AgentApproved`, `OrderPaid`, `OrderRefunded`, `TicketIssued`, `TicketScanned`, `PriceApproved`, `ContentPublished`, `PointsChanged`, `MissionCompleted`, `OrderExpired`, `PaymentDiscrepancy`, `BillPaid`, `PartnerOrderUpdated`, `TicketTransferred` และ `AccountDeleted = "account.deleted"`, `MemberInviteAdded = "member.invite_added"`, `MemberConsentChanged = "member.consent_changed"`
  - `func Subject(eventType string) string` คืนค่า `"evt." + eventType`
  - `const Stream = "EVENTS"` และ `const DLQPrefix = "dlq."`
  - `func New(eventType string, payload any) (Envelope, error)` ID เป็น `uuid.NewV7()` และ OccurredAt เป็น UTC

- [ ] **Step 1: เขียน test ที่ fail**: `Subject("order.paid") == "evt.order.paid"`, `New` ได้ ID ที่เรียงตามเวลา (สร้าง 2 ตัว ตัวหลังต้องมากกว่าเมื่อเทียบ string) และ `len(All) == 20` โดยไม่มีชื่อซ้ำ และมี `account.deleted`, `member.invite_added`, `member.consent_changed`
- [ ] **Step 2: รันให้ fail** — `make test PKG=./pkg/chassis/events/...`
- [ ] **Step 3: เขียนตาม Interfaces** พร้อม `var All = []string{...}`
- [ ] **Step 4: รันให้ผ่าน**
- [ ] **Step 5: Commit** — `git commit -m "feat(chassis): event envelope and catalog"`

### Task 4: Outbox write + Relay

**Files:**
- Create: `pkg/chassis/outbox/outbox.go`
- Test: `pkg/chassis/outbox/outbox_test.go` (integration: `TEST_DB_DSN`, `TEST_NATS_URL`)

**Interfaces:**
- Consumes: `events.Envelope`, `events.Subject`, `events.Stream`
- Produces:
  - `func Write(ctx context.Context, tx *sql.Tx, e events.Envelope) error`
  - `func EnsureStream(ctx context.Context, js jetstream.JetStream) error` สร้าง stream `EVENTS` subjects `evt.>` และ `dlq.>`, storage file, `Duplicates: 2 * time.Minute`
  - `type Relay struct { DB *sql.DB; JS jetstream.JetStream; Batch int; Interval time.Duration }` (ค่าเริ่ม Batch 100, Interval 200ms)
  - `func (r *Relay) Run(ctx context.Context) error` และ `func (r *Relay) Tick(ctx context.Context) (int, error)` (ใช้ใน test) คืนจำนวนแถวที่ publish

Tick: `BEGIN` → `SELECT … WHERE published_at IS NULL ORDER BY created_at LIMIT ? FOR UPDATE SKIP LOCKED` → publish ทีละแถวด้วย `jetstream.WithMsgID(id)` → `UPDATE published_at` เฉพาะแถวที่ publish สำเร็จ → `COMMIT` ถ้า publish ล้มเหลวให้หยุดลูปแล้ว commit เฉพาะแถวที่สำเร็จ

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestWriteRollbackLeavesNoEvent(t *testing.T)      // tx.Rollback() แล้ว outbox ต้องว่าง
func TestTickPublishesAndMarks(t *testing.T)           // Write 3 ตัว → Tick == 3, subscriber ได้ 3 ข้อความตามลำดับ, published_at ไม่ null
func TestTwoRelaysNoDoublePublish(t *testing.T)        // Write 200 ตัว, 2 Relay Tick พร้อมกันใน goroutine → stream ได้ 200 ข้อความพอดี
func TestTickWhenNATSDownKeepsRows(t *testing.T)       // ปิด nats conn ก่อน Tick → error, แถวยัง published_at IS NULL; เปิดใหม่ Tick → ส่งครบ
```

- [ ] **Step 2: รันให้ fail** — `make test-int PKG=./pkg/chassis/outbox/...`
- [ ] **Step 3: เขียน `Write`, `EnsureStream`, `Relay`** ตาม Interfaces
- [ ] **Step 4: รันให้ผ่าน** (ทั้ง 4 test, รัน `-count=5 -race` เพื่อจับ race)
- [ ] **Step 5: Commit** — `git commit -m "feat(chassis): transactional outbox and relay"`

### Task 5: Idempotent consumer + dead-letter

**Files:**
- Create: `pkg/chassis/consumer/consumer.go`
- Test: `pkg/chassis/consumer/consumer_test.go` (integration)

**Interfaces:**
- Consumes: `events.Envelope`, `events.Subject`, `events.DLQPrefix`
- Produces:
  - `type Handler func(ctx context.Context, tx *sql.Tx, e events.Envelope) error`
  - `type Config struct { Name string; EventTypes []string; MaxDeliver int; AckWait time.Duration }` (ค่าเริ่ม MaxDeliver 10, AckWait 30s) ใช้ durable consumer ชื่อ `Name` และ FilterSubjects มาจาก `events.Subject` ของแต่ละ type
  - `func Run(ctx context.Context, db *sql.DB, js jetstream.JetStream, cfg Config, h Handler) error`

ลำดับการทำงานต่อหนึ่งข้อความ: `BEGIN` → `INSERT IGNORE processed_events(consumer, event_id)` → ถ้า affected = 0 ให้ `ROLLBACK` + `Ack` (เคยทำแล้ว) → เรียก handler → error ให้ `ROLLBACK` + `Nak` ส่วนสำเร็จให้ `COMMIT` + `Ack` → ถ้า `msg.Metadata().NumDelivered >= MaxDeliver` และยัง error ให้ publish ข้อความเดิมไป `dlq.<Name>` แล้ว `Term` และ log ระดับ ERROR พร้อม attribute `alert=oncall`

- [ ] **Step 1: เขียน test ที่ fail**

```go
func TestDuplicateDeliveryHandledOnce(t *testing.T)   // publish event id เดียวกัน 2 ครั้ง (คนละ MsgID) → handler ถูกเรียก 1 ครั้ง
func TestHandlerErrorRollsBackMarker(t *testing.T)    // handler fail ครั้งแรก สำเร็จครั้งที่สอง → ถูกเรียก 2 ครั้ง, processed_events มี 1 แถว
func TestPoisonGoesToDLQ(t *testing.T)                // MaxDeliver 3, handler fail ตลอด → เรียก 3 ครั้ง, มีข้อความใน dlq.<name> 1 ข้อความ
func TestHandlerWritesInSameTx(t *testing.T)          // handler insert ตาราง test แล้ว return error → ตาราง test ว่าง
func TestThaiPayloadRoundTrip(t *testing.T)           // payload "สวนนงนุช 农努奇 🌺" ถึง handler ตรงทุก byte
```

- [ ] **Step 2: รันให้ fail** — `make test-int PKG=./pkg/chassis/consumer/...`
- [ ] **Step 3: เขียน `Run`** ตามลำดับข้างบน ใช้ `consumer.Consume` (push-style callback ของ `jetstream`)
- [ ] **Step 4: รันให้ผ่าน** `-race`
- [ ] **Step 5: Commit** — `git commit -m "feat(chassis): idempotent consumer with DLQ"`

### Task 6: Server + Observability + service ตัวอย่าง

**Files:**
- Create: `pkg/chassis/obs/obs.go`, `pkg/chassis/server/server.go`, `services/example/main.go`
- Test: `pkg/chassis/server/server_test.go`, `services/example/smoke_test.go` (integration)

**Interfaces:**
- Consumes: ทุกแพ็กเกจจาก Task 1–5
- Produces:
  - `obs.Setup(ctx, cfg config.Base) (shutdown func(context.Context) error, err error)` ตั้ง TracerProvider + MeterProvider (OTLP gRPC) และ `slog` แบบ JSON ที่ใส่ `trace_id` ให้ทุกบรรทัด ถ้า endpoint ว่างให้ใช้ noop
  - `server.New(cfg config.Base, db *sql.DB) *Server` มีฟิลด์ `Mux *http.ServeMux` และ `GRPC *grpc.Server` (ใส่ otelgrpc interceptor ไว้แล้ว)
  - `(*Server).Run(ctx) error` ฟัง HTTP + gRPC, `GET /healthz` (200 เสมอ), `GET /readyz` (200 เมื่อ `db.PingContext` ผ่าน ไม่ผ่านได้ 503), gRPC health service, graceful shutdown 15 วินาทีเมื่อได้ SIGTERM

- [ ] **Step 1: เขียน test ที่ fail**: `/healthz` ได้ 200, `/readyz` ได้ 503 เมื่อ DB ปิด, gRPC `grpc.health.v1.Health/Check` ได้ `SERVING`
- [ ] **Step 2: รันให้ fail**
- [ ] **Step 3: เขียน `obs.Setup`, `server.New`, `Run`**
- [ ] **Step 4: เขียน `services/example`**: migrate → `POST /ping` ที่เขียน outbox `example.pinged` → relay → consumer ที่ insert ตาราง `pongs`
- [ ] **Step 5: smoke test**: `POST /ping` 1 ครั้งแล้วภายใน 5 วินาทีตาราง `pongs` ต้องมี 1 แถว และ trace มี span ทั้งฝั่ง HTTP และ consumer ที่ใช้ trace id เดียวกัน (ส่ง `traceparent` ผ่าน NATS header)
- [ ] **Step 6: รันทั้งหมด** — `make test && make test-int && make build SVC=example` ต้อง PASS
- [ ] **Step 7: Commit** — `git commit -m "feat(chassis): server, otel, example service"`

### Task 7: Contracts + client generation

**Files:**
- Create: `api/openapi/README.md`, `buf.yaml`, `buf.gen.yaml`, `Makefile` target `gen`
- Test: CI step `make gen && git diff --exit-code`

**Interfaces:**
- Produces: `make gen` (รันใน container: `bufbuild/buf`, `node:20` สำหรับ `openapi-typescript`, `openapitools/openapi-generator-cli`) สร้าง Go stub จาก `api/proto/**/*.proto` ไปที่ `gen/go/`, สร้าง TS client จาก `api/openapi/*.yaml` ไปที่ `gen/ts/` (`openapi-typescript`) และ Dart client ไปที่ `gen/dart/` (`openapi-generator-cli -g dart-dio`)

- [ ] **Step 1: เพิ่ม `api/openapi/example.yaml`** (`POST /ping`) และ `api/proto/example/v1/example.proto` (`rpc Ping`)
- [ ] **Step 2: รัน `make gen`** ต้องได้ไฟล์ใน `gen/go`, `gen/ts`, `gen/dart`
- [ ] **Step 3: รัน `make gen && git diff --exit-code`** ต้องไม่มี diff (generate ซ้ำได้ผลเดิม)
- [ ] **Step 4: Commit** — `git commit -m "chore: contract codegen for go, ts, dart"`
