# เฟส 1: ดัชนีแผน Implementation

**Spec:** [docs/system-design.md](../../system-design.md) ร่าง 3 · [ROADMAP.md](../../../roadmap/ROADMAP.md) v2.4
**ตารางเวลา:** [docs/plan/Nongnooch_Dev_Plan.xlsx](../../plan/Nongnooch_Dev_Plan.xlsx) · [docs/plan/tasks.md](../../plan/tasks.md)

System design แบ่งเป็นหลาย subsystem ที่ทำงานแยกกันได้ (8 service + Web + Admin + Gate + App) ตาม Scope Check ของ writing-plans จึงแยกเป็น 1 แผนต่อ 1 subsystem ทุกแผนสร้างซอฟต์แวร์ที่ทดสอบได้ด้วยตัวเอง และทุกแผนใช้ Plan 01 เป็นฐาน

| # | แผน | WBS ใน Excel | ผู้รับผิดชอบ | เริ่ม | ขึ้นกับ | สถานะ |
|---|---|---|---|---|---|---|
| 00 | [Mock ระบบภายนอก (mockext + Mailpit)](2026-11-11-00-mock-external.md) | 1.1.12–1.1.14 | BE1, BE2, BE3 | 13/11/26 | 01 (server) | เขียนแล้ว |
| 01 | [Platform chassis: Go template, outbox, consumer, contracts](2026-11-11-01-platform-chassis.md) | 1.1.1–1.1.3, 1.1.11 | TL | 13/11/26 | – | เขียนแล้ว |
| 02 | [Identity: OTP + JWT](2026-11-11-02-identity-otp.md) | 1.3.1, 1.3.2, 1.9.2 | BE1 | 19/11/26 | 01 | เขียนแล้ว |
| 03 | Member & Point: สมัคร, เลขบัตรเข้ารหัส, PointLedger | 1.4.x | BE1 | 30/11/26 | 01, 02 | ยังไม่เขียน |
| 04 | Content (CMS) + Approval + RBAC | 1.8.x | TL | 25/11/26 | 01 | ยังไม่เขียน |
| 05 | Commerce: Catalog, Inventory, Order | 1.6.1–1.6.3 | BE2 | 16/11/26 | 01 | ยังไม่เขียน |
| 06 | Commerce: KBank, Webhook, กระทบยอด, คืนเงิน, Waiting room | 1.6.4–1.6.9 | BE2, BE3 | 08/12/26 | 05 | ยังไม่เขียน |
| 07 | Ticket & Gate: ออกบัตร Ed25519, Gate API, Transfer | 1.7.x | BE2 | 15/12/26 | 01, 06 | ยังไม่เขียน |
| 08 | Social login + WeChat | 1.3.3–1.3.5 | BE1 | 08/12/26 | 02 | ยังไม่เขียน |
| 09 | Agent: บริษัท, ไกด์, Invite, Referral, ตรวจจับผิดปกติ | 1.5.x | BE1 | 14/12/26 | 03 | ยังไม่เขียน |
| 10 | Notification: Device, Template 7 ภาษา, Push routing | 1.9.1, 1.9.3–1.9.5 | BE3 | 01/12/26 | 01 | ยังไม่เขียน |
| 11 | Report: Read model, Dashboard API, Export | 1.10.x | BE3 | 17/12/26 | 01 | ยังไม่เขียน |
| 12 | API Gateway | 1.1.4 | TL | 03/12/26 | 01, 02 | ยังไม่เขียน |
| 13 | Website (Next.js) | 1.11.x | FE1 | 16/11/26 | contracts จาก 01 | ยังไม่เขียน |
| 14 | Admin / CMS / Dashboard + Gate Scanner | 1.12.x, 1.13.x | FE2 | 16/11/26 | 04 | ยังไม่เขียน |
| 15 | Mobile (Flutter) Tourist + Agent | 1.14.x | MB1, MB2 | 01/12/26 | 13 | ยังไม่เขียน |
| 16 | Infra: Staging, CI/CD, Cloudflare, Observability | 1.1.5–1.1.10 | DO | 01/12/26 | 01, Q18, Q20 | ยังไม่เขียน |

## โหมด Demo (มติ 11 พ.ย. 2026)

งานนี้เป็น demo ระบบภายนอกทุกตัวจึงใช้ mock จาก Plan 00 ส่วน service ของเราเรียกผ่าน interface + URL ใน env เมื่อจะต่อของจริงให้เขียน adapter มาแทน

| ระบบภายนอก (O1) | ใช้ใน demo | แผนที่เกี่ยวข้อง |
|---|---|---|
| KBank PG (บัตร, PromptPay, Alipay+, WeChat Pay, UnionPay), Webhook, กระทบยอด, คืนเงิน | mockext `/pg` + หน้า checkout จำลอง | 06 |
| Cloudflare Turnstile | mockext `/turnstile` + widget จำลองฝั่งเว็บ / แอป | 02, 13, 15 |
| SMS OTP (+86) | mockext `/sms` ดูผลใน Mock Console | 02 |
| Email | Mailpit (container) | 02, 10 |
| LINE / Google / Apple / WeChat login | mockext `/oauth/{p}` (OIDC + WeChat UnionID) | 08 |
| FCM / Huawei Push Kit | mockext `/push` | 10 |
| Web Push (VAPID) | **ของจริง** เพราะไม่ต้องมีบัญชี | 10 |
| ระบบขายบัตร / ประตูเดิม | mockext `/legacy` | 07 |
| Trip.com | ลิงก์ออกไปภายนอกตามปกติ (ไม่ต้อง mock) | 13 |
| Cloudflare CDN / WAF, Cloud | ไม่ใช้ใน demo · รันด้วย Docker Compose | 16 |

ลำดับที่แนะนำให้เขียนแผนต่อ คือเรียงตามวันเริ่มในตาราง โดยเขียนแต่ละแผนให้เสร็จก่อนวันเริ่มงานนั้นอย่างน้อย 2 วันทำงาน
