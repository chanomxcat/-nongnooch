# Spec: System Design Diagrams จาก Roadmap v7

**วันที่:** 11 พ.ย. 2026
**ผู้อ่าน:** ทีม dev ของ Wisdom Vast (ใช้คุยต่อเรื่องสถาปัตยกรรม)
**แหล่งข้อมูล:** `Nongnooch_Roadmap_v7.pptx` (ข้อความดึงไว้ที่ `docs/source/roadmap-v7-text.txt`) และ `ROADMAP.md` v2.1 หัวข้อ 2.1, 4.2 (E1–E10), 6, 7, 8, 9

## การเปลี่ยนแปลง 11 พ.ย. 2026 (ร่าง 2)

อนุมัติแล้ว ส่วนนี้มีผลเหนือข้อความเดิมในเอกสารนี้

- **Microservices แทน monolith:** แบ่งตาม domain 8 service ในเฟส 1 คือ Identity, Member & Point, Agent, Commerce, Ticket & Gate, Content, Notification และ Report เฟส 3 เพิ่ม Engagement เฟส 4 เพิ่ม Partner
  - ทุก client เข้าผ่าน API Gateway
  - ระหว่าง service ใช้ gRPC เมื่อต้องรอคำตอบทันที และใช้ NATS JetStream + outbox สำหรับ event
  - ใช้ 1 schema ต่อ service บน MariaDB cluster เดียว
  - Checkout เป็น saga แบบ choreography
  - ส่ง trace และ log ผ่าน OpenTelemetry
- **7 ภาษาตั้งแต่เฟส 1:** TH, EN, ZH, JA, KO, DE, FR (ไม่มี RU) ภาษาที่ยังไม่แปลแสดง EN แทน `[สมมติฐาน]`
- **diagram ที่เปลี่ยน:**
  - เขียนใหม่: O2 และ S1, S3–S10
  - เพิ่ม: **O3 Event Map** (รวมเป็น 23 ภาพ)
  - แก้ label: O1, F4, F5, F9 และ S2
- **คำถามเปิดที่เพิ่ม:** Q19 Gateway, Q20 Orchestration, Q21 ความเสี่ยงกำหนดเวลา, Q22 การแปลเนื้อหา 7 ภาษา, Q23 QA หลายภาษา
- ROADMAP.md และสไลด์ v7 ไม่แก้ตาม เอกสารหลักมีหมายเหตุบอกจุดที่ต่างกันไว้แล้ว

## 1. เป้าหมาย

ทีม dev ดูแล้วเห็นตรงกันใน 3 เรื่อง

1. ระบบมีส่วนใดบ้าง
2. ข้อมูลไหลผ่านส่วนเหล่านั้นอย่างไร ครบทั้ง 4 เฟส
3. ยังมีจุดใดที่ต้องตัดสินใจ

## 2. ผลลัพธ์ที่ต้องส่ง

| ไฟล์ | เนื้อหา |
|---|---|
| `docs/system-design.md` | เอกสารหลัก (ต้นฉบับ) มีโค้ด Mermaid ทุกภาพ และใต้โค้ดแต่ละชุดมี `![](diagrams/<id>.svg)` |
| `docs/diagrams/<id>.mmd` | โค้ด Mermaid แยกไฟล์ละภาพ ใช้เป็น input ของ mermaid-cli |
| `docs/diagrams/<id>.svg` | ภาพที่ export แล้ว มีไอคอน Lucide ครบ |
| `docs/package.json` | devDependencies: `@mermaid-js/mermaid-cli@12.2.0`, `@iconify-json/lucide` และ script `npm run diagrams` |
| `docs/README.md` | วิธี render ใหม่ และวิธี import Mermaid เข้า draw.io |

**ให้ `.mmd` เป็นต้นฉบับเดียว** ใน `system-design.md` โค้ด Mermaid เป็นสำเนาที่ script สร้างจาก `.mmd` เวลาแก้ให้แก้ที่ `.mmd` แล้วรัน `npm run diagrams` ซึ่งจะ export SVG และอัปเดตบล็อกโค้ดใน `.md` ให้ตรงกัน

## 3. ข้อตกลงด้าน style

- **ภาษา:** label เป็นภาษาไทย ชื่อเทคนิคคงภาษาอังกฤษ (Go API, MariaDB, Webhook)
- **สีบอกเฟส (classDef)**

  | คลาส | เฟส | fill | stroke |
  |---|---|---|---|
  | `p1` | 1 | `#dcfce7` | `#16a34a` |
  | `p2` | 2 | `#dbeafe` | `#2563eb` |
  | `p3` | 3 | `#ffedd5` | `#ea580c` |
  | `p4` | 4 | `#f3e8ff` | `#9333ea` |
  | `assume` | สมมติฐาน | `#fef9c3` | `#ca8a04` (เส้นประ) |

- **ป้าย `[สมมติฐาน]`** ใช้กับส่วนที่ roadmap ยังไม่ได้ระบุ ทุกจุดต้องมีรายการอยู่ใน section 4 ของเอกสารหลัก
- **ไอคอน:** Lucide ผ่าน icon shape ของ Mermaid 11 (`id@{ icon: "lucide:<name>", form, label, pos: "b" }`)
  - ใช้กับคน, client, infra และระบบภายนอก
  - **ไม่ใช้**กับโมดูลภายใน Go API เพราะจะรก
  - **ไม่ใช้**ใน sequence diagram เพราะ Mermaid ยังไม่รองรับ
  - Lucide ไม่มีโลโก้แบรนด์ จึงใส่ชื่อเทคโนโลยีไว้ใน label เสมอ
- **ลิงก์ข้ามภาพ:** ใช้กล่อง subroutine `[[F1 สมัครสมาชิก]]` แทนการวาดซ้ำ
- **sequence:** เปิด `autonumber` ไว้อ้างอิงเลขขั้นตอนในที่ประชุม และใช้ `alt` / `opt` กับเงื่อนไขสำคัญ

### ตารางไอคอน

| ส่วน | ไอคอน | label |
|---|---|---|
| นักท่องเที่ยว | `user` | นักท่องเที่ยว |
| Agent / ไกด์ | `users` | บริษัททัวร์ / ไกด์ |
| เจ้าหน้าที่ | `user-cog` | เจ้าหน้าที่นงนุช |
| ผู้บริหาร | `briefcase` | ผู้บริหาร |
| Website | `globe` | Next.js · SSR · TH/EN/ZH |
| App | `smartphone` | Flutter · Tourist / Agent · GMS/HMS |
| Admin / CMS | `layout-dashboard` | Next.js |
| Gate Scanner | `scan-line` | Web App · Offline Ed25519 |
| Cloudflare | `shield` | CDN · WAF · Rate limit · Turnstile |
| Go API | `server` | Modular monolith · OpenAPI |
| MariaDB | `database` | 10.6+ · InnoDB · utf8mb4 |
| Object Storage | `hard-drive` | รูป · เอกสาร Agent |
| Job Worker | `list-todo` | ตาราง Job · SKIP LOCKED |
| KBank PG | `credit-card` | บัตร · PromptPay |
| Email | `mail` | Email |
| SMS | `message-square` | SMS OTP (+86) |
| Push | `bell` | FCM + Huawei Push Kit |
| Social login | `log-in` | LINE · Google · Apple (เฟส 2: FB, WeChat, Kakao, WhatsApp) |
| ระบบเดิม | `building-2` | ขายบัตร · ประตู · POS · บัญชี |
| Trip.com / Partner | `external-link` | Trip.com · Partner |

## 4. โครงเอกสาร `system-design.md`

0. **วิธีอ่าน:** legend สีเฟส, ความหมายของป้ายสมมติฐาน, ความหมายของกล่อง `[[ ]]`
1. **System Overview**
   - **1.1 Context:** ผู้ใช้ 4 กลุ่ม และระบบภายนอกทั้งหมด
   - **1.2 Container + Module:** ใช้ร่างที่อนุมัติแล้ว (`docs/diagrams/drafts/ov.mmd`) แก้ให้โมดูลเฟส 1 จัดเป็นกริด 3 แถว แทนแถวเดียวที่กว้างเกินไป
2. **Tech Stack:** ตาราง ส่วน → เทคโนโลยี → ไอคอน → เหตุผล ดึงจาก ROADMAP §9 และระบุสถานะที่ยังรอยืนยัน (KBank, รูปแบบ Cloud #23, AppGallery จีน #24)
3. **Flow Chart**

   | id | Flow | เฟส |
   |---|---|---|
   | F1 | นักท่องเที่ยวสมัคร + Invite code + Welcome/Invite point (1 เบอร์/อีเมล = 1 บัญชี) | 1 |
   | F2 | Agent: บริษัทสมัคร (หนังสือรับรอง) → อนุมัติ → ไกด์สมัคร → QR → พ้อยรอตรวจ | 1 |
   | F3 | ซื้อบัตร (ใช้ร่าง `drafts/f3.mmd`) | 1 |
   | F4 | เข้าประตู: online/offline, รายวัน/รายปี (รูปผู้ถือ), สแกนซ้ำ, คืนเงินแล้ว | 1 |
   | F5 | CMS / ราคา / โปรโมชัน / กติกาพ้อย: ร่าง → รออนุมัติ → อนุมัติ/ตีกลับ + Audit log | 1 |
   | F6 | แลกพ้อย: ไกด์แลกตั๋วฟรี, สมาชิกใช้เป็นส่วนลด, 100 บาท = 1 พ้อย, หักคืนเมื่อคืนเงิน | 2 |
   | F7 | Agent ซื้อบัตร Bulk + เจน QR | 3 |
   | F8 | QR event → voucher อาหาร/กิจกรรม → สแกนหน้าร้าน; Garden Mission → Reward | 3 |
   | F9 | Sponsor campaign: ยื่น → นงนุชอนุมัติ → แสดง → รายงาน | 3 |
   | F10 | Partner / ของฝาก / จองโรงแรม-MICE + แบ่งรายได้ | 4 |

4. **Sequence Diagram**

   | id | Sequence | เฟส |
   |---|---|---|
   | S1 | สมัครด้วย OTP (SMS, อีเมลสำรอง) + Invite → PointLedger (Welcome, Invite, ไกด์รอตรวจ) | 1 |
   | S2 | Social login (LINE/Google/Apple) + ผูกกับบัญชีเดิม | 1 |
   | S3 | Checkout → KBank → Webhook idempotent → Ticket Ed25519 → Job ส่งอีเมล/push (ใช้ร่าง `drafts/s3.mmd`) | 1 |
   | S4 | Job กระทบยอดรายชั่วโมง + คืนเงินจาก Admin | 1 |
   | S5 | สแกนประตู online + offline แล้ว sync + ส่งเข้าระบบเดิม `[สมมติฐาน]` | 1 |
   | S6 | Waiting room: ออกบัตรคิวลงลายเซ็น → ปล่อยเข้าตาม capacity ที่ตั้งใน Admin | 1 |
   | S7 | Push routing: FCM หรือ Huawei Push Kit ตาม device token | 1 |
   | S8 | ใช้พ้อยเป็นส่วนลดตอน checkout + หักคืนพ้อยเมื่อคืนเงิน | 2 |
   | S9 | Scan จ่ายเงินในสวน (KBank) + ได้พ้อย + POS `[สมมติฐาน]` | 3 |
   | S10 | สั่งซื้อสินค้า Partner + บันทึกส่วนแบ่ง | 4 |

5. **คำถามเปิดสำหรับทีม dev:** ตาราง `#` · คำถาม · diagram ที่เกี่ยวข้อง · ลิงก์ไปยัง ROADMAP §15

## 5. นอกขอบเขต

- ER diagram (เพิ่มเป็น section ใหม่ได้ถ้าขอ)
- ไฟล์ `.drawio` (ทีม import Mermaid เข้า draw.io เองได้: Arrange → Insert → Advanced → Mermaid)
- user flow ระดับหน้าจอที่ผูกกับ Figma

## 6. การตรวจสอบ

- `npm run diagrams` ต้อง render ครบ 22 ภาพโดยไม่มี error
- เปิดดู SVG ทุกภาพ ต้องเห็นไอคอน Lucide ครบ (ไม่มีกล่อง "?") ตัวอักษรไทยไม่แตก และ label ไม่ทับกัน
- ทุกป้าย `[สมมติฐาน]` ต้องมีรายการอยู่ใน section 5 ของเอกสารหลัก
- ทุกกล่อง `[[ ]]` ต้องชี้ไป id ที่มีอยู่จริง
