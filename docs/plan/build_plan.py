# Build the dev plan (Excel + Gantt + Markdown) from one task table.
# Run: python docs/plan/build_plan.py
# Source: docs/system-design.md (ร่าง 3) + ROADMAP.md v2.4
from datetime import date, timedelta
from pathlib import Path
from openpyxl import Workbook
from openpyxl.styles import Alignment, Border, Font, PatternFill, Side
from openpyxl.formatting.rule import FormulaRule
from openpyxl.utils import get_column_letter
from openpyxl.worksheet.datavalidation import DataValidation
from openpyxl.workbook.defined_name import DefinedName

OUT = Path(__file__).parent
PLAN_START = date(2026, 11, 13)

# ponytail: วันหยุดราชการไทย ใส่ตามปฏิทินคาดการณ์ วันพระ (มาฆะ วิสาขะ อาสาฬหะ) ต้องเทียบประกาศ ครม. อีกครั้ง
HOLIDAYS = {
    "2026-12-07": "ชดเชยวันพ่อแห่งชาติ", "2026-12-10": "วันรัฐธรรมนูญ", "2026-12-31": "วันสิ้นปี",
    "2027-01-01": "วันขึ้นปีใหม่", "2027-02-22": "ชดเชยวันมาฆบูชา [ตรวจสอบ]", "2027-04-06": "วันจักรี",
    "2027-04-13": "สงกรานต์", "2027-04-14": "สงกรานต์", "2027-04-15": "สงกรานต์",
    "2027-05-03": "ชดเชยวันแรงงาน", "2027-05-04": "วันฉัตรมงคล", "2027-05-20": "วันวิสาขบูชา [ตรวจสอบ]",
    "2027-06-03": "วันเฉลิมฯ พระราชินี", "2027-07-19": "วันเข้าพรรษา [ตรวจสอบ]",
    "2027-07-20": "ชดเชยวันอาสาฬหบูชา [ตรวจสอบ]", "2027-07-28": "วันเฉลิมฯ ร.10",
    "2027-08-12": "วันแม่แห่งชาติ", "2027-10-13": "วันนวมินทรมหาราช", "2027-10-25": "ชดเชยวันปิยมหาราช",
}
HOL = {date.fromisoformat(d) for d in HOLIDAYS}

TEAM = [
    ("PM", "Product Owner / PM", 1, "วางแผน ประสานงาน ติดตามกำหนดการ"),
    ("UX", "UX/UI Designer", 1, "UI เว็บ App Admin Gate"),
    ("TL", "Tech Lead", 1, "Service template, contract, Content service, Gateway, code review"),
    ("BE1", "Backend Go #1", 1, "Identity, Member & Point, Agent"),
    ("BE2", "Backend Go #2", 1, "Commerce, Ticket & Gate"),
    ("BE3", "Backend Go #3 [เสนอเพิ่ม ตาม Q21]", 1, "Notification, Report, sync ระบบเดิม, ช่องทางจ่ายต่างชาติ (เริ่ม 1 ธ.ค.)"),
    ("FE1", "Frontend Next.js #1", 1, "Website + หน้า Agent"),
    ("FE2", "Frontend Next.js #2 [รออนุมัติ #25]", 1, "Admin / CMS / Dashboard / Gate Scanner"),
    ("MB1", "Mobile Flutter #1", 1, "Tourist App: สมัคร, พ้อย, Push + Agent App (เริ่ม ธ.ค.)"),
    ("MB2", "Mobile Flutter #2", 1, "Tourist App: เนื้อหา, ซื้อบัตร, บัตรของฉัน (เริ่ม ธ.ค.)"),
    ("QA1", "QA #1", 1, "Test plan, API + Web"),
    ("QA2", "QA #2", 1, "App, Load test"),
    ("DO", "DevOps", 1, "Cloud, CI/CD, Observability, Security (Full-time ธ.ค.–เม.ย.)"),
    ("NN", "ทีมนงนุช เวิลด์", None, "อนุมัติ ราคา เนื้อหา 7 ภาษา UAT อบรม"),
    ("ALL", "ทั้งทีม WV", None, "งานร่วม เช่น แก้บั๊ก Hypercare"),
]

# wbs | ชื่องาน | ผู้รับผิดชอบ | วันทำงาน (0 = milestone) | เริ่ม: วันที่ / WBS ก่อนหน้า (คั่นด้วย ,) / ว่าง = ต่อจาก sibling ก่อนหน้า | อ้างอิง | รายละเอียด
# แถวที่มีแค่ wbs | ชื่อ = หัวข้อสรุป (เฟส หรือ task)
DATA = """
1 | เฟส 1: เปิดบริการพร้อมระบบสมาชิกและ Agent (Soft launch 1 มี.ค. · Go-Live 1 เม.ย. 2027)
1.0 | Milestone และการตัดสินใจ
1.0.1 | M1 บัญชีและสิทธิ์พร้อม | NN | 0 | 2026-11-13 | ROADMAP M1 | Payment Merchant, Apple / Google / Huawei Developer, SMS ต่างประเทศ, สิทธิ์ API ระบบบัตร / ประตู · Demo: ไม่ขวางงานพัฒนาเพราะใช้ mock (Plan 00)
1.0.2 | ตัดสินใจ API Gateway และ Orchestration | TL | 0 | 2026-11-20 | Q19, Q20 | เลือก Go เขียนเอง / Traefik / Kong และ Docker Compose / k3s / Managed K8s ก่อนเริ่มทำ Gateway (1.1.4) และ Staging (1.1.5)
1.0.3 | ยื่นบัญชี WeChat Open Platform | NN | 0 | 2026-11-20 | Q24, #0.13 | Website + Mobile application + Official Account ในนามนงนุช การตรวจใช้หลายสัปดาห์ ถ้าไม่ผ่านก่อน M4 ให้ซ่อนปุ่ม
1.0.4 | M2 ดีไซน์ ราคา กติกา + เลือก Cloud + โซน / E-KYC | NN | 0 | 2026-11-27 | M2, #23, #27, #28 | UX/UI อนุมัติ, ราคาและกติกาบัตร, กติกาพ้อยฝั่งสะสม, เงื่อนไข Agent, รูปแบบ Cloud, จำนวนโซน, E-KYC
1.0.5 | M3 เนื้อหาครบ 7 ภาษาใน CMS | NN | 0 | 2026-12-11 | M3, Q22 | TH EN ZH JA KO DE FR ถ้าภาษาใดไม่ทัน ตกลงให้แสดง EN แทน
1.0.6 | M4 Feature complete บน Staging | PM | 0 | 2027-01-29 | M4 | ทุกฟีเจอร์เฟส 1 ทั้งเว็บและ App ใช้งานได้บน Staging
1.0.7 | ส่ง Tourist App + Agent App ให้สโตร์ตรวจ | MB1 | 0 | 2027-02-05 | ROADMAP | App Store, Google Play, Huawei AppGallery เผื่อถูกตีกลับ 1–2 รอบ
1.0.8 | M5 UAT sign-off | NN | 0 | 2027-02-12 | M5 | ไม่มีบั๊ก Critical / High ค้าง
1.0.9 | Code freeze + Deploy Production | DO | 0 | 2027-02-19 | ROADMAP | หลังวันนี้แก้เฉพาะบั๊กที่ขวางการขายหรือการสมัคร
1.0.10 | Soft launch | ALL | 0 | 2027-03-01 | ROADMAP [รอยืนยันวัน] | Pre-sale, ไกด์เริ่มชวนสมัคร, สแกนประตูจริง
1.0.11 | Go-Live | ALL | 0 | 2027-04-01 | ROADMAP | เปิดสาธารณะพร้อมแคมเปญ
1.0.12 | ปิด Hypercare + อนุมัติมูลค่าพ้อย | NN | 0 | 2027-04-30 | ROADMAP | ส่งมอบเข้าสู่ SLA ปกติ ยืนยันขอบเขตเฟส 2

1.1 | Platform และ Infrastructure
1.1.1 | Go service template (chassis) | TL | 5 | 2026-11-13 | 1.2 | config, structured log, OpenTelemetry, health check, HTTP + gRPC server, DB migration, ตาราง outbox + processed_events, outbox relay → NATS, consumer แบบ idempotent ทุก service ใช้ร่วมกัน
1.1.2 | Contract: OpenAPI + proto + Event schema | TL | 3 | 1.1.1 | 1.3, O3 | OpenAPI ภายนอกต่อ service, proto gRPC (ValidateInvite, GetTickets, VoidTickets, GetContactPrefs), schema 17 event และชื่อ subject บน JetStream
1.1.3 | Staging ชั่วคราว (VM + Docker Compose) | TL | 2 | 1.8.1 | Q20 | MariaDB, NATS, MinIO, CMS ขึ้นให้นงนุชกรอกเนื้อหาได้ก่อน Staging จริงพร้อม
1.1.4 | API Gateway | TL | 4 | 1.8.3,1.0.2 | O2, S6, Q19 | ตรวจ JWT, route ไป 8 service, rate limit, รวม OpenAPI, ตรวจ access token ของ Waiting room ก่อน route POST /orders
1.1.5 | Staging จริง: MariaDB cluster + NATS JetStream cluster + Object Storage (IaC) | DO | 5 | 2026-12-01 | 2, Q18 | ตามรูปแบบ Cloud ที่เลือก 27 พ.ย., 1 schema ต่อ service, ผู้ใช้ DB แยกต่อ service (ห้ามอ่าน schema อื่น)
1.1.6 | CI/CD 8 service + Web + App build pipeline | DO | 5 | | 2 | pipeline แยกต่อ service, container registry, deploy Staging อัตโนมัติ, Production ต้องมีผู้อนุมัติ, build Flutter 2 Flavor × GMS / HMS
1.1.7 | Cloudflare: DNS, CDN, WAF, Rate limit, Turnstile | DO | 2 | | 2 | Turnstile แทน reCAPTCHA (ถูกบล็อกในจีน), security headers, SSL
1.1.8 | Secret manager + key เข้ารหัสเลขบัตร | DO | 2 | | 2, Q26 | key สำหรับเข้ารหัสระดับฟิลด์และ HMAC, จำกัดบทบาทที่ถอดรหัสได้
1.1.9 | Observability + Monitoring | DO | 4 | | 2 | OTel collector, trace ข้าม service, uptime check, error tracking, crash reporting App, alert ไป on-call, รองรับ SLA 99.5%
1.1.10 | Production environment + Backup + ทดสอบกู้คืน | DO | 5 | 2027-01-11 | 4.3 | ขนาดเครื่องรับพีค, backup อัตโนมัติ, ทดสอบ restore จริง
1.1.11 | สร้าง client TS / Dart อัตโนมัติจาก OpenAPI | TL | 1 | 1.1.3 | 2 | ใช้ร่วมกันใน Next.js และ Flutter
1.1.12 | Mock ระบบภายนอก: Console + Turnstile + SMS + Mailpit + Social login IdP | BE1 | 5 | 2026-11-13 | Demo, O1, S2 | service mockext ตาม Plan 00 Task 1–3: Mock Console, siteverify จำลอง, SMS inbox, Mailpit, OIDC LINE / Google / Apple + WeChat (UnionID)
1.1.13 | Mock KBank Payment Gateway | BE2 | 3 | 1.6.3 | Demo, S3, S4 | Plan 00 Task 4: หน้า checkout จำลอง (สำเร็จ / ล้มเหลว / ไม่ส่ง webhook / ยอดไม่ตรง), webhook ลงลายเซ็น + retry + ส่งซ้ำ, inquiry, refund
1.1.14 | Mock Push FCM / Huawei + ระบบประตูเดิม | BE3 | 2 | 2026-12-01 | Demo, S5, S7 | Plan 00 Task 5: token invalid- ได้ UNREGISTERED, รับ gate-events / tickets แสดงใน Console

1.2 | UX/UI Design
1.2.1 | Web UI ตาม Figma + หน้าที่ยังไม่มีในต้นแบบ | UX | 6 | 2026-11-13 | 4.5 | ใช้ Concept ที่เลือก, ออกแบบเพิ่ม: สมัครสมาชิก + กรอกเลขบัตร, พ้อยของฉัน, หน้า Agent, ผู้ใช้บัตรรายใบ, ส่งบัตรให้เพื่อน, หน้ารอคิว
1.2.2 | App UI: Tourist App + Agent App | UX | 5 | | E5, E10 | ใช้ design system เดียวกับเว็บ
1.2.3 | Admin / CMS UI + Gate Scanner UI | UX | 5 | | E6, F4 | หน้าแก้ 7 ภาษาคู่กัน, Approval inbox, หน้าสแกนและผลสแกน (ผ่าน / ใช้แล้ว / ผิดวัน / ยกเลิก)
1.2.4 | ตรวจ layout 7 ภาษา | UX | 3 | 2027-01-11 | Q23 | DE คำยาว, JA / KO ฟอนต์ บนหน้า Checkout และหน้าสมัคร ทั้งเว็บและ App

1.3 | Identity service
1.3.1 | OTP เบอร์โทร / อีเมล + Turnstile | BE1 | 4 | 1.1.1,1.1.12 | F1, S1 | เก็บ OTP แบบ hash, อายุและจำนวนครั้ง, rate limit, ตรวจ Turnstile token, outbox otp.requested
1.3.2 | JWT / session | BE1 | 2 | 1.9.2 | S1 | access + refresh token, logout, Gateway ตรวจได้
1.3.3 | Social login LINE / Google / Apple + ผูกบัญชี | BE1 | 4 | 1.4.4 | S2, Q7 | Demo ใช้ IdP จำลองใน mockext · OIDC + PKCE, ตาราง SocialIdentity, อีเมลตรงบัญชีเดิมต้องยืนยัน OTP ก่อนผูก, Build HMS ไม่มี Google
1.3.4 | WeChat Login | BE1 | 3 | 1.5.4 | S2, Q24 | Demo ใช้ WeChat จำลองใน mockext · Website QR, Official Account OAuth (ในแอป WeChat), Mobile SDK, ผูกด้วย UnionID, ซ่อนปุ่มได้จาก Admin
1.3.5 | ขอลบบัญชี | BE1 | 1 | 1.4.7 | E5 | รับคำขอจากเว็บและ App ลบภายใน 30 วัน (job), แจ้ง service อื่นผ่าน event

1.4 | Member & Point service
1.4.1 | Schema member + เข้ารหัสเลขบัตร | BE1 | 3 | 1.3.2 | 1.2, F1 | Member, Companion, Consent, เลขบัตร / พาสปอร์ตเข้ารหัส + HMAC hash, แสดงปิดบังเหลือ 4 หลักท้าย
1.4.2 | สมัครสมาชิก + สิทธิ์ราคาไทย / ต่างชาติ | BE1 | 3 | | F1, S1 | ตรวจ check digit 13 หลัก, รูปแบบพาสปอร์ต + ประเทศ, 1 เบอร์ / อีเมล / เลขบัตร = 1 บัญชี, outbox member.registered
1.4.3 | PointLedger + Welcome / Invite point | BE1 | 3 | | F1, S1 | append-only แก้ย้อนหลังไม่ได้, รับ referral.created ให้พ้อยไกด์สถานะ "รอตรวจ" idempotent ด้วย referralId, ไม่มี API แลกพ้อยในเฟส 1
1.4.4 | Consent + gRPC GetContactPrefs | BE1 | 1 | | S7 | ถอนความยินยอมการตลาดได้, ส่ง consent + ภาษาให้ Notification
1.4.5 | รายชื่อผู้ติดตาม + ผูกออเดอร์ Guest | BE1 | 2 | 1.5.6 | F3, E9 | Companion CRUD, ผูกออเดอร์ที่ซื้อแบบ Guest ด้วยอีเมล / เบอร์เดิม
1.4.6 | ประวัติซื้อและประวัติเข้าชม | BE1 | 2 | | S3, S5 | รับ order.paid และ ticket.scanned บันทึกเป็นประวัติ ใช้วัดการกลับมาเที่ยวซ้ำ
1.4.7 | กรอก Invite code ภายหลัง | BE1 | 1 | | F1, Q6 | ได้ 1 ครั้งต่อบัญชี ภายใน N วันตามกติกา

1.5 | Agent service
1.5.1 | บัญชีบริษัททัวร์ | BE1 | 3 | 1.3.3 | F2 | กรอกข้อมูล, อัปโหลดหนังสือรับรองไป Object Storage, OTP ผู้ติดต่อ, สถานะ รอตรวจ / อนุมัติ / ปฏิเสธ, outbox agent.approved
1.5.2 | บัญชีไกด์ภายใต้บริษัท | BE1 | 2 | | F2 | เข้าร่วมด้วยลิงก์เชิญหรือขอเข้าร่วมให้บริษัทอนุมัติ, OTP เบอร์, สังกัดได้ 1 บริษัท, บริษัทเพิ่ม / ระงับไกด์
1.5.3 | InviteCode + QR + gRPC ValidateInvite | BE1 | 2 | | F1, S1 | QR เปิดหน้าสมัครพร้อมโค้ด (deep link เปิด App ถ้าติดตั้ง)
1.5.4 | Referral | BE1 | 2 | | S1, O3 | รับ member.registered สร้าง Referral, outbox referral.created
1.5.5 | กฎตรวจจับความผิดปกติ | BE1 | 3 | 1.3.4 | F2, Q6 | หลายบัญชีจากเครื่อง / IP เดียว, เบอร์เรียงกัน, สมัครถี่, เพดานต่อไกด์ต่อวัน → ติดธงให้ทีมนงนุชตรวจ
1.5.6 | ยอดรายไกด์ / บริษัท + ตัดพ้อย | BE1 | 2 | | F2, E10 | แสดงเฉพาะจำนวน ไม่มีข้อมูลส่วนตัว, ทีมนงนุชตัดพ้อยผิดเงื่อนไขพร้อม Audit log

1.6 | Commerce service
1.6.1 | Catalog + PriceRule | BE2 | 3 | 2026-11-16 | 1.2 | Product, ราคาไทย / ต่างชาติ, Pre-sale ตามช่วงวัน, โค้ดส่วนลด, รับ price.approved
1.6.2 | Inventory: โควตารายวัน + ที่นั่งต่อรอบการแสดง | BE2 | 4 | | F3 | จองชั่วคราว 15 นาที, job ปล่อยคืน + outbox order.expired, ที่นั่ง / รอบตั้งจาก CMS (POC 1,500), กันขายเกิน
1.6.3 | Order | BE2 | 5 | | F3 | หลายใบต่อออเดอร์ ระบุผู้ใช้รายใบ คิดราคาไทย / ต่างชาติรายใบ, Upsell ห้องอาหาร / การแสดง, บัตรรายปี + ของขวัญ, Guest ซื้อได้เฉพาะราคาต่างชาติ [#31]
1.6.4 | KBank Payment + Webhook | BE2 | 5 | 1.1.13 | S3, Q4 | Demo เรียก mockext `/pg` · สร้างรายการบัตร / PromptPay, ตรวจลายเซ็น Webhook, idempotent ด้วย eventId, BEGIN Payment=paid · Order=confirmed · ตัดโควตา · outbox order.paid COMMIT
1.6.5 | กระทบยอดรายชั่วโมง | BE2 | 2 | 1.7.2 | S4 | หา Payment pending > 15 นาที ถาม KBank, paid → ทำแบบ Webhook, failed → ปล่อยโควตา, ยอดไม่ตรง → payment.discrepancy
1.6.6 | คืนเงิน | BE2 | 3 | | S4, E6 | สิทธิ์ Finance, คำนวณค่าธรรมเนียมตามนโยบาย, gRPC VoidTickets แบบ sync ก่อนคืน, เงินหรือ Voucher, outbox order.refunded, Audit log
1.6.7 | Waiting room | BE2 | 4 | 1.7.4 | S6, Q9 | บัตรคิวลงลายเซ็น {seq, issuedAt, sig}, ปล่อยตาม capacity ต่อนาที, access token อายุสั้น, เปิด / ปิดจาก Admin ทันที
1.6.8 | ช่องทางจ่ายต่างชาติ | BE3 | 2 | 2027-01-18 | E3, #11 | Demo ผ่าน mockext `/pg` · Alipay+ / WeChat Pay / UnionPay ตามที่ KBank รองรับ
1.6.9 | ใบเสร็จอิเล็กทรอนิกส์ / ใบกำกับภาษี | BE3 | 2 | | E3 | ส่งทางอีเมล, ขอใบกำกับภาษีเต็มรูป [รอรูปแบบจากฝ่ายบัญชี]

1.7 | Ticket & Gate service
1.7.1 | ออก e-Ticket รายใบ + QR Ed25519 | BE2 | 4 | 1.6.4 | S3, F4 | รับ order.paid, Unique ticket ID + ประเภท + วันที่ + ลายเซ็น, TicketHolder, idempotent ด้วย orderId, outbox ticket.issued ภายใน 30 วิ, gRPC GetTickets
1.7.2 | VoidTickets + รายการ void | BE2 | 1 | | S4 | gRPC void บัตรทันที ใช้ใน snapshot ประตู
1.7.3 | บัตรรายปี | BE2 | 2 | 1.6.6 | F4 | เปิดใช้ 365 วัน, รูปผู้ถือบัตร, แก้ข้อมูลผู้ถือจาก Admin
1.7.4 | Gate API | BE2 | 4 | | S5 | GET snapshot (public key, บัตรวันนี้, รายการ void), POST scans อัปเดต used แบบ atomic, scans:batch ติดธงใช้ซ้ำข้ามประตู, outbox ticket.scanned
1.7.5 | ส่งบัตรให้เพื่อน (TicketTransfer) | BE2 | 3 | 1.6.7 | F11, Q27 | บัตรยังไม่ใช้และก่อนเวลาเข้าชม, ส่งได้ 1 ครั้ง / ใบ [#31], บัตรราคาไทยผู้รับต้องยืนยันเลขบัตร, ออก ID ใหม่ void ID เดิม, outbox ticket.transferred, ลิงก์สำหรับผู้ไม่มีบัญชี
1.7.6 | ค้นหาบัตรที่ประตู | BE2 | 2 | | F4 | ค้นด้วยเลขออเดอร์ / เบอร์ / อีเมล, ปล่อยผ่านพร้อม Audit log
1.7.7 | Sync ระบบประตู / บัตรเดิม | BE3 | 5 | 1.6.9 | Q1, Q2, S5 | Demo ส่งไป mockext `/legacy` · ตาม Integration Spec (1.15.1) ส่งยอดเข้าประตูและสถานะบัตร ถ้าไม่ได้สิทธิ์ใช้ Gate Scanner ของเราแทน (R2)

1.8 | Content service (CMS)
1.8.1 | Schema content + ชนิดเนื้อหา 7 ภาษา | TL | 4 | 1.1.2 | E6, F5 | แก้ 7 ภาษาคู่กัน, Preview, ภาษาที่ยังไม่แปลแสดง EN, login เจ้าหน้าที่เบื้องต้น
1.8.2 | RBAC 7 บทบาท | TL | 2 | 1.1.11 | E6 | Admin, Editor, Approver, Finance, Gate Staff, Agent Manager, Viewer
1.8.3 | Media upload | TL | 2 | | 2 | อัปโหลดไป Object Storage แปลง WebP / AVIF
1.8.4 | Workflow อนุมัติ + Audit log | TL | 4 | 1.1.4 | F5, Q5 | ร่าง → รออนุมัติ → อนุมัติ / ตีกลับ, four-eyes [Q5], ล้าง cache Cloudflare, outbox price.approved / content.published
1.8.5 | Zone / POI + ตารางการแสดง + FAQ + เบอร์ฉุกเฉิน | TL | 3 | | E6, #27 | หมุดต้องอ้างโซนที่มีจริงก่อนเผยแพร่, หมวด: จุดถ่ายรูป ร้านอาหาร ห้องน้ำ รถราง ปฐมพยาบาล
1.8.6 | หน้าตรวจคำแปลที่ขาด | TL | 2 | | E6 | สรุปฟิลด์ที่ยังไม่แปลแยกภาษาและชนิดเนื้อหา กดไปหน้าแก้ได้

1.9 | Notification service
1.9.1 | Schema notify + Device | BE3 | 2 | 1.1.14 | S7 | Device (ios / gms / hms / web), Web Push subscription, ประวัติการส่ง
1.9.2 | Email + SMS OTP (+86) | BE1 | 3 | 1.3.1 | S1 | Demo ส่งผ่าน mockext `/sms` และ Mailpit · รับ otp.requested ส่ง SMS ตามภาษา, SMS ไม่ถึงใช้ Email OTP สำรอง
1.9.3 | Template 7 ภาษา + consumer | BE3 | 3 | 1.9.1 | S7, O3 | ticket.issued, referral.created, agent.approved, ticket.transferred, order.refunded, payment.discrepancy, ไม่มีภาษาใช้ EN
1.9.4 | Push routing FCM / Huawei Push Kit / Web Push | BE3 | 3 | | S7 | Demo: FCM / HMS ส่งไป mockext `/push`, Web Push ใช้ของจริง · เลือกช่องทางตาม platform, แสดงนอกแอป + deep link, ส่งซ้ำทางอีเมลสำหรับเรื่องสำคัญ, ลบ token เสีย
1.9.5 | Push campaign ตั้งเวลา / เลือกกลุ่ม | BE3 | 2 | | E5 | ข้อความการตลาดส่งเฉพาะผู้ยินยอม (GetContactPrefs)

1.10 | Report service
1.10.1 | Read model จาก event | BE3 | 3 | 1.9.5 | O3, E7 | order.paid, member.registered, ticket.scanned, points.changed, ticket.transferred
1.10.2 | Dashboard API | BE3 | 2 | | E7 | ยอดขาย / สมาชิก / Agent / คนเข้าประตูรายชั่วโมง หน่วงไม่เกิน 1 นาที
1.10.3 | Export + Analytics | BE3 | 2 | | E7, E8 | Export CSV / Excel รายวัน, GA4 + server-side event สำหรับ funnel

1.11 | Website (Next.js)
1.11.1 | Scaffold Next.js + i18n 7 ภาษา | FE1 | 3 | 1.11.10 | E1 | /th /en /zh /ja /ko /de /fr + hreflang, SSR, design system, client TS
1.11.2 | Landing + หน้าข้อมูลสวน | FE1 | 5 | | E1, 4.5 | Activities, Accommodations (redirect Trip.com), Auditorium / MICE + ฟอร์มสอบถาม, จุดชวนสมัคร, ข้อมูลจาก CMS
1.11.3 | สมัคร / Login | FE1 | 5 | | F1, S1, S2 | OTP + Turnstile, LINE / WeChat QR / Google / Apple, กรอกเลขบัตร / พาสปอร์ต, PDPA แยกช่องการตลาด, รับ invite จาก URL, สมัครเสร็จใน 2 นาที
1.11.4 | Checkout | FE1 | 6 | | F3, S3, S6 | 4 ขั้นตาม Figma, ผู้ใช้บัตรรายใบ + เลือกผู้ติดตาม, Upsell + ที่นั่งคงเหลือ, จับเวลา 15 นาที, ชำระ KBank, หน้ารอคิว 7 ภาษา
1.11.5 | บัตรของฉัน + พ้อย + โปรไฟล์ | FE1 | 4 | | F11, E9 | ดูบัตร QR, ส่งบัตรให้เพื่อน, ยอดและประวัติพ้อย "เปิดแลกเร็ว ๆ นี้", ผู้ติดตาม, ขอลบบัญชี
1.11.6 | ตารางการแสดง + แผนที่ + FAQ | FE1 | 3 | | E5 | แผนที่ภาพนิ่งซูมได้ แยกโซน, ไฮไลต์รอบถัดไป, เบอร์ฉุกเฉิน
1.11.7 | หน้า Agent | FE1 | 4 | | F2, E10 | สมัครบริษัท + อัปโหลดเอกสาร, ไกด์เข้าร่วม, Invite QR, ยอดผู้สมัครรายไกด์
1.11.8 | Web Push + PWA manifest | FE1 | 2 | | S7, Q25 | ขอสิทธิ์ notification, ลงทะเบียน VAPID subscription, ชวน iOS เพิ่มลงหน้าจอโฮม
1.11.9 | SEO + Performance + Accessibility | FE1 | 4 | | E1 | sitemap, meta / OG จาก CMS, 301 redirect URL เดิม, Lighthouse SEO ≥ 90, LCP ≤ 2.5 วิ, WCAG 2.1 AA หน้าหลัก Checkout สมัคร
1.11.10 | Design tokens + ชุดไอคอน (Web + Flutter) | FE1 | 2 | 2026-11-16 | DESIGN.md 2, 7 | CSS variables จาก DESIGN.md หัวข้อ 2 (สี, ตัวอักษร desktop / mobile, spacing, radius, elevation), export ไอคอน `Icon/*` 30 ตัวเป็น SVG, ไฟล์ token เดียวกันให้ Flutter ทำ ThemeData (MB1 ใช้ใน 1.14.1)
1.11.11 | ข้อความ UI 7 ภาษา (เว็บ) | FE1 | 2 | 1.11.9 | E1, Q22 | แยกไฟล์ข้อความ UI (ปุ่ม, error, label ที่ไม่ได้มาจาก CMS) ต่อภาษา, ตรวจไม่มี key ขาดใน CI, ภาษาที่ขาดใช้ EN
1.11.12 | หน้าแคมเปญ Pre-sale / Soft launch / สงกรานต์ | FE1 | 3 | | 4.1 | template หน้าแคมเปญที่ทีมนงนุชสร้างจาก CMS ได้เอง (hero, ช่วงวันที่, โค้ดส่วนลด, CTA ซื้อบัตร / สมัครสมาชิก), หมดช่วงแล้วซ่อนอัตโนมัติ

1.12 | Admin / CMS / Dashboard (Next.js)
1.12.1 | Admin shell + login + RBAC | FE2 | 2 | 2026-11-16 | E6 | เมนูตามบทบาท
1.12.2 | CMS editor 7 ภาษา + Preview + Media | FE2 | 5 | | E6 | ต้องพร้อมให้นงนุชกรอกเนื้อหาต้น ธ.ค.
1.12.3 | Approval inbox + Audit log | FE2 | 3 | | F5 | อนุมัติ / ตีกลับพร้อมเหตุผล, ดูก่อน / หลัง
1.12.4 | ราคา โปรโมชัน โค้ดส่วนลด กติกาพ้อย ที่นั่งต่อรอบ | FE2 | 3 | | E6 | ทุกการเปลี่ยนผ่านอนุมัติก่อนมีผล
1.12.5 | จัดการ Agent | FE2 | 3 | | F2 | อนุมัติ / ปฏิเสธ / ระงับบริษัทและไกด์, รายการผิดปกติ, ตัดพ้อย
1.12.6 | จัดการออเดอร์และสมาชิก | FE2 | 4 | | E6, S4 | ค้นหาออเดอร์, ส่งบัตรซ้ำ, คืนเงิน, ค้นหาสมาชิก, ปรับพ้อยพร้อมเหตุผล, ระงับบัญชี
1.12.7 | Dashboard + Export | FE2 | 3 | 1.13.2 | E7 | กราฟ real-time ยอดขาย สมาชิก Agent ประตู
1.12.8 | ควบคุม Waiting room + Push campaign + ปุ่ม WeChat | FE2 | 2 | | S6, S7 | เปิด / ปิดคิว, capacity, ตั้งเวลาส่ง Push, เปิด / ปิดปุ่ม WeChat
1.12.9 | Zone / POI editor + หน้าตรวจคำแปล + FAQ | FE2 | 3 | | E6 | ให้นงนุชกรอกหมุดและ FAQ ใน W10

1.13 | Gate Scanner Web App
1.13.1 | สแกน QR online / offline | FE2 | 5 | 1.12.6 | F4, S5 | PWA, กล้องสแกน, ตรวจ Ed25519 ในเครื่อง, IndexedDB snapshot + คิว scan, sync เมื่อเน็ตกลับ, ผลภายใน 1 วินาที
1.13.2 | ค้นหาบัตร + รูปบัตรรายปี | FE2 | 2 | | F4 | เมื่อลูกค้าแสดง QR ไม่ได้, บันทึก Audit log

1.14 | Mobile App (Flutter)
1.14.1 | Setup Flutter | MB1 | 3 | 2026-12-01 | E5 | 2 Flavor (Tourist / Agent) × GMS / HMS, ARB 7 ภาษา, client Dart, deep link
1.14.2 | สมัคร / Login ในแอป | MB1 | 5 | | F1, S2 | OTP, LINE, WeChat SDK, Google (GMS เท่านั้น), Apple, เลขบัตร, PDPA, invite จาก deep link
1.14.3 | เนื้อหา: หน้าแรก การแสดง แผนที่ FAQ | MB2 | 4 | 1.14.1 | E5 | แจ้งเตือนรอบการแสดงได้
1.14.4 | ซื้อบัตร + บัตรของฉัน | MB2 | 6 | | F3, F11 | Checkout ผ่าน KBank, เก็บบัตรไว้ในเครื่องเปิดได้แม้สัญญาณอ่อน, ส่งบัตรให้เพื่อน
1.14.5 | พ้อย + Push FCM / Huawei Push Kit | MB1 | 4 | | S7 | ลงทะเบียน Device, แตะแล้วเปิดหน้าที่เกี่ยวข้อง
1.14.6 | Agent App | MB1 | 5 | | F2, E10 | login ไกด์, Invite QR, ยอดผู้สมัคร, Push เมื่อมีผู้สมัครด้วยโค้ด
1.14.7 | โปรไฟล์ + ผู้ติดตาม + ลบบัญชี | MB2 | 2 | | E5 | ตามเงื่อนไข Apple / Google และ PDPA
1.14.8 | Store listing 7 ภาษา × 3 สโตร์ | MB2 | 2 | 2027-01-21 | W12 | รูป, คำอธิบาย, privacy label / data safety
1.14.9 | Build + ส่งตรวจ 3 สโตร์ | MB1 | 3 | 2027-02-03 | ROADMAP | Tourist App + Agent App ส่งภายใน 5 ก.พ.
1.14.10 | แก้ตามผลรีวิวสโตร์ + ส่งใหม่ | MB1 | 5 | | ROADMAP | เผื่อถูกตีกลับ 1–2 รอบก่อน Soft launch

1.15 | เชื่อมระบบเดิม
1.15.1 | สำรวจระบบเดิม + Integration Spec | TL+NN | 5 | 2026-11-16 | Q1, งาน 0.5 | ร่วมกับ IT นงนุช: API / ไฟล์ / ไม่เชื่อม, ทิศทางข้อมูล, เครื่องสแกนเดิม (Q2), LAN ระหว่างประตู (Q3)
1.15.2 | ตัดสินใจเชื่อมระบบเดิมหรือใช้ Gate Scanner ของเรา | NN | 0 | 2026-11-27 | R2 | ถ้าไม่ได้สิทธิ์ API ภายใน 27 พ.ย. ใช้ระบบตรวจบัตรของเราเอง

1.16 | QA และการทดสอบ
1.16.1 | Test plan + test case | QA1 | 5 | 2026-11-16 | 11 | จาก Acceptance Criteria E1–E10 และ flow F1–F5, F11, S1–S7
1.16.2 | ทดสอบฟีเจอร์ API + Web ตาม sprint | QA1 | 35 | | 11 | ทดสอบทุกฟีเจอร์ที่ขึ้น Staging, automate API test เส้นหลัก
1.16.3 | ทดสอบ App GMS / HMS / iOS | QA2 | 30 | 2026-12-07 | E5 | มือถือจริงทั้ง 3 แบบ
1.16.4 | E2E ทั้งระบบ + 7 ภาษา | QA1 | 5 | 2027-01-21 | W12, Q23 | สมัคร → ซื้อ → ออกบัตร → สแกน → พ้อย → Dashboard
1.16.5 | Load test 3 เท่าพีค + Waiting room | QA2 | 5 | 2027-01-28 | 4.3 | คำสั่งซื้อ / การสมัคร / ผู้ใช้พร้อมกัน ตามตัวเลขใน Launch Checklist
1.16.6 | ทดสอบเฉพาะทาง | QA1 | 5 | 2027-01-28 | 4.3 | Demo: ทดสอบกรณีผิดปกติผ่าน mockext (SMS ไม่ถึง, webhook ซ้ำ / ไม่มา, ยอดไม่ตรง, token push เสีย) · ของจริงภายหลัง: OTP +86 และประเทศหลัก, WeChat บัญชีจริง, เครือข่ายจีน, Push ขณะแอปปิด, สแกน Offline ตัดเน็ตแล้ว sync
1.16.7 | Security test + Pen test | DO | 6 | 2027-02-01 | 12 | OWASP, การเข้ารหัสเลขบัตร, สิทธิ์ตามบทบาท
1.16.8 | UAT รอบ 1 | NN | 5 | 2027-01-28 | W13 | ผู้ใช้จริงของนงนุชทุกฝ่าย
1.16.9 | UAT รอบ 2 | NN | 5 | | W14 | ตรวจบั๊กที่แก้จากรอบ 1
1.16.10 | แก้บั๊กจาก UAT | ALL | 13 | 2027-01-28 | W13–W15 | ปิดบั๊ก Critical / High ก่อน 12 ก.พ.
1.16.11 | ข้อมูลตัวอย่าง + สคริปต์สาธิต (demo) | TL | 4 | 2027-01-04 | Demo | สคริปต์ seed ที่รันใน Docker: บัตร / ราคา / รอบการแสดง, เนื้อหา 7 ภาษา, สมาชิก ไทย / ต่างชาติ, บริษัททัวร์ + ไกด์, ออเดอร์ตัวอย่าง · เขียนสคริปต์สาธิตทีละขั้น (สมัครด้วย invite → ซื้อบัตร → จ่ายใน mockext → สแกนประตู offline → Dashboard) · `make seed` รีเซ็ตกลับสถานะเริ่มต้นได้

1.17 | Launch และ Hypercare
1.17.1 | ซ้อม Deploy Production | DO | 3 | 2027-02-15 | W15 | ซ้อมทั้ง 8 service + migration + rollback
1.17.2 | ทดสอบบน Production | QA1 | 4 | 2027-02-19 | W16, 4.3 | สแกนประตูจริงทุกช่อง online / offline, จ่ายเงินจริงยอดเล็กทุกช่องทางแล้วคืนเงิน, OTP ต่างประเทศ
1.17.3 | เปิดลงทะเบียน Agent / ไกด์ + อบรมไกด์ | NN | 6 | 2027-02-19 | 4.4 | ตรวจเอกสาร อนุมัติ, คลิปสั้น 7 ภาษา + ป้าย QR (WV ทำสื่อ)
1.17.4 | อบรมพนักงาน + ติดตั้งป้าย QR | NN | 5 | 2027-02-22 | 4.3 | ประตู เคาน์เตอร์ ร้านอาหาร Call center ทีมดูแล Agent
1.17.5 | Hypercare Soft launch | ALL | 18 | 2027-03-01 | 4.6 | On-call, วิศวกรประจำหน้างาน 1–2 มี.ค., release แก้ไขรายสัปดาห์
1.17.6 | Code freeze ช่วงเปิดตัว + Load test ซ้ำ + Go / No-go | ALL | 5 | 2027-03-25 | W21 | เพิ่มขนาดเครื่องรับสงกรานต์
1.17.7 | Hypercare Go-Live + สงกรานต์ | ALL | 18 | 2027-04-01 | 4.6 | วิศวกรประจำหน้างาน 1–2 เม.ย. และ 13–15 เม.ย., ประชุมเช้า 15 นาที

1.18 | งานฝั่งนงนุช (ขึ้นกับกำหนดการ)
1.18.1 | ส่งราคา กติกาบัตร กติกาพ้อย เงื่อนไข Agent | NN | 10 | 2026-11-13 | M2 | โครงสร้างข้อมูล Commerce / Member ขึ้นกับเรื่องนี้
1.18.2 | อนุมัติดีไซน์ | NN | 5 | 2026-11-23 | M2 |
1.18.3 | เตรียมเนื้อหา + จ้างแปล JA / KO / DE / FR | NN | 13 | 2026-11-13 | Q22 | เตรียมเป็นไฟล์ก่อน CMS พร้อม
1.18.4 | กรอกเนื้อหา 7 ภาษาใน CMS | NN | 6 | 2026-12-03 | M3 |
1.18.5 | นโยบายความเป็นส่วนตัว เงื่อนไขการขาย คืนเงิน เงื่อนไข Agent 7 ภาษา | NN | 20 | 2026-12-14 | 4.3, #17 |
1.18.6 | กรอกหมุดแผนที่ + FAQ ใน CMS | NN | 5 | 2027-01-11 | W10 |
1.18.7 | แปลข้อความ UI 7 ภาษา (เว็บ + แอป) | NN | 10 | 2027-01-04 | Q22 | WV ส่งไฟล์ข้อความ UI ภาษาอังกฤษให้ภายใน 4 ม.ค. นงนุชส่งคำแปล ZH / JA / KO / DE / FR กลับ

2 | เฟส 2: แลกพ้อยรับสิทธิ์ (พัฒนา เม.ย.–พ.ค. · ใช้งาน มิ.ย. 2027)
2.0 | Milestone
2.0.1 | อนุมัติมูลค่าพ้อย อายุ เพดาน | NN | 0 | 2027-04-30 | 6 | ผ่าน F5 ก่อนเปิดแลก
2.0.2 | ส่ง App เฟส 2 ตรวจ | MB1 | 0 | 2027-05-14 | 6 |
2.0.3 | เปิดใช้งานเฟส 2 + เปิดแลกพ้อย | ALL | 0 | 2027-06-01 | 6 [รอยืนยันวัน] |
2.1 | ออกแบบเฟส 2
2.1.1 | ออกแบบ flow + UI แลกพ้อย ยกเลิก / เลื่อนวัน Chatbot | UX | 10 | 2027-03-15 | 4.6 | ทำช่วง Hypercare มี.ค.
2.2 | แลกพ้อยและพ้อยจากการใช้จ่าย
2.2.1 | HoldPoints + hold / used / compensation | BE1 | 5 | 2027-04-01 | S8 | SELECT FOR UPDATE, ตรวจเพดาน / อายุ, order.expired คืนพ้อย, idempotent ด้วย orderId
2.2.2 | พ้อยจากการใช้จ่าย + หักคืนเมื่อคืนเงิน | BE1 | 3 | | F6, S8, Q10 | 100 บาทที่จ่ายจริง = 1 พ้อย, คืนพ้อยที่ใช้ไป [Q10]
2.2.3 | ไกด์แลกตั๋วฟรี | BE1 | 3 | | F6 | เฉพาะพ้อยสถานะ "ผ่าน", ออก Ticket ราคา 0
2.2.4 | ใช้พ้อยเป็นส่วนลดตอน Checkout | BE2 | 3 | 2027-04-01 | S8 | POST /orders/{id}/points เรียก HoldPoints
2.2.5 | ตรวจพ้อยไกด์ย้อนหลังก่อนเปิดแลก | NN | 10 | 2027-04-19 | F2 | ตัดพ้อยที่ผิดเงื่อนไข
2.3 | โปรโมชันกลับมาเที่ยว + ตั๋วฟรีวันเกิด
2.3.1 | คูปองกลับมาเที่ยวครั้งที่ 2+ | BE1 | 2 | 2.2.3 | F6 | จาก ticket.scanned, ส่งเฉพาะผู้ยินยอม
2.3.2 | Job ตั๋วฟรีวันเกิด | BE1 | 2 | | F6, Q11 | กติกา [รอยืนยัน]
2.4 | Social login เพิ่ม + โปรไฟล์เต็ม
2.4.1 | Facebook / Kakao / WhatsApp | BE2 | 4 | 2.2.4 | 6 | ใช้ SocialIdentity เดิม
2.4.2 | โปรไฟล์: วันเกิด ความสนใจ ภาษา | BE2 | 2 | | 6 |
2.5 | ยกเลิก / เลื่อนวันเข้าชมเอง
2.5.1 | ยกเลิก / คืนเงินเป็นเงินหรือ Voucher | BE2 | 4 | 2.4.2 | F12, Q29 | ตามนโยบาย #17, ใช้ S4 คืนเงิน + void บัตร
2.5.2 | เลื่อนวันเข้าชม | BE2 | 3 | | F12 | 1 ครั้ง ล่วงหน้า 12 ชม., ย้ายโควตา, ออก QR ใหม่ void เดิม, หักพ้อย
2.6 | Chatbot "Ask Nongnooch"
2.6.1 | Chatbot จาก FAQ + CMS 7 ภาษา | TL | 10 | 2027-04-19 | 6 | ส่งต่อเจ้าหน้าที่เมื่อตอบไม่ได้, ถ้าไม่ทัน มิ.ย. ออกพร้อมเฟส 3
2.7 | Frontend และ Mobile เฟส 2
2.7.1 | เว็บ: แลกพ้อย ยกเลิก / เลื่อนวัน Social login โปรไฟล์ Chatbot | FE1 | 15 | 2027-04-19 | 6 |
2.7.2 | Admin: กติกาพ้อย คูปอง วันเกิด ตรวจพ้อยไกด์ | FE2 | 10 | 2027-04-19 | 6 |
2.7.3 | App: แลกพ้อย ยกเลิก / เลื่อนวัน Social login | MB1 | 10 | 2027-04-19 | 6 |
2.7.4 | App: โปรไฟล์ + Chatbot + Agent App แลกตั๋วฟรี | MB2 | 10 | 2027-04-19 | 6 |
2.8 | QA และ Release เฟส 2
2.8.1 | ทดสอบเฟส 2 | QA1 | 15 | 2027-04-26 | 11 |
2.8.2 | UAT เฟส 2 | NN | 8 | 2027-05-17 | 6 |
2.8.3 | Deploy เฟส 2 | DO | 2 | 2027-05-28 | 6 |

3 | เฟส 3: กิจกรรมและผู้สนับสนุน (พัฒนา มิ.ย.–ก.ค. · ใช้งาน ส.ค. 2027)
3.0 | Milestone
3.0.1 | เปิดใช้งานเฟส 3 | ALL | 0 | 2027-08-02 | 7 |
3.1 | Engagement service
3.1.1 | Setup Engagement service + schema engage | TL | 3 | 2027-06-01 | 1.2 | Quest, Checkpoint, Badge, Leaderboard, Voucher, Event, Sponsor, Review, consume ticket.scanned
3.2 | Agent ซื้อบัตร Bulk
3.2.1 | ราคา Agent + BulkOrder | BE2 | 5 | 2027-06-01 | F7, Q12 | จองโควตา, ชำระตาม S3, ส่วนแบ่ง [รอยืนยัน]
3.2.2 | เจน QR รายใบ + แจกลิงก์ / ใบรวม QR + พ้อย Bulk | BE2 | 4 | | F7 |
3.2.3 | พ้อยต่อเนื่องให้ไกด์ | BE1 | 3 | 3.6.1 | 7 | เมื่อลูกค้าที่ชวนกลับมาซื้อซ้ำ (ข้อเสนอ)
3.3 | ล่าสมบัติ (Explorer Passport)
3.3.1 | ภารกิจ + Checkpoint + Geofencing | BE1 | 6 | 2027-06-01 | F8 | สแกนป้าย QR ตามจุดจริง, ตรวจรัศมี กันโกง, ภารกิจตามฤดูกาลจาก CMS, outbox mission.completed
3.3.2 | Badge + Leaderboard | BE1 | 3 | | F8 |
3.4 | ของรางวัลและระดับสมาชิก
3.4.1 | แลก Reward | BE1 | 4 | 3.3.2 | F8 | รหัสใช้ครั้งเดียว, สต็อก, 1 รางวัล / บัญชี / ภารกิจ, ของหมดชดเชยคูปอง
3.4.2 | Tier Silver / Gold / Platinum | BE1 | 3 | | 7 | ตามยอดใช้จ่ายรายปี, ตัวคูณพ้อย [รออนุมัติ]
3.5 | Event และ Voucher
3.5.1 | ปฏิทินอีเวนต์ + ลงทะเบียน + บัตรกิจกรรม | BE2 | 4 | 3.2.2 | F8 |
3.5.2 | QR check-in + ออก Voucher + ร้านสแกน | BE2 | 4 | | F8, Q13 | ร้านใช้ Gate Scanner app [สมมติฐาน]
3.6 | Sponsor, Review, Recommendation
3.6.1 | Recommendation แบบกฎ + รายงานการตลาดเชิงลึก | BE1 | 5 | 3.4.2 | 7 | Segment, Cohort การกลับมาซ้ำ, ผล Agent
3.6.2 | Sponsor campaign | TL | 5 | 3.1.1 | F9, Q14 | สร้างใน CMS ผ่าน F5, impression / click / ใช้สิทธิ์, รายงานไม่มีข้อมูลส่วนบุคคล, ปิดอัตโนมัติ
3.6.3 | รีวิวและให้คะแนน | TL | 4 | | 7 | ทีมนงนุชตรวจก่อนแสดง
3.7 | Frontend และ Mobile เฟส 3
3.7.1 | เว็บ: Bulk Agent อีเวนต์ รีวิว Sponsor | FE1 | 20 | 2027-06-07 | 7 |
3.7.2 | Admin: ภารกิจ ของรางวัล Tier Sponsor รีวิว อีเวนต์ | FE2 | 20 | 2027-06-07 | 7 |
3.7.3 | App: ล่าสมบัติ Geofencing Badge Leaderboard | MB1 | 20 | 2027-06-07 | F8 |
3.7.4 | App: อีเวนต์ Voucher รีวิว Tier + Agent App Bulk | MB2 | 20 | 2027-06-07 | 7 |
3.8 | QA และ Release เฟส 3
3.8.1 | ทดสอบเฟส 3 (รวมทดสอบ Geofencing ในสวน) | QA1 | 15 | 2027-06-28 | 11 |
3.8.2 | ส่ง App ตรวจ + UAT เฟส 3 | NN | 8 | 2027-07-16 | 7 |
3.8.3 | Deploy เฟส 3 | DO | 2 | 2027-07-29 | 7 |

4 | เฟส 4: บริการครบวงจร (พัฒนา ส.ค.–ก.ย. · ใช้งาน ต.ค. 2027)
4.0 | Milestone
4.0.1 | เปิดใช้งานเฟส 4 (ไม่ทันให้ทยอยออก ธ.ค. 2027 ตาม R22) | ALL | 0 | 2027-10-01 | 8 |
4.1 | Partner service
4.1.1 | Partner + Offer + RevenueShare | TL | 5 | 2027-08-02 | S10, Q16 | gRPC GetOffer, consume order.paid / order.refunded, outbox partner.order.updated
4.1.2 | สรุปส่วนแบ่งรายเดือน + แจ้งออเดอร์ Partner | TL | 3 | | S10 | อีเมล / portal [Q16]
4.2 | ร้านของฝาก
4.2.1 | Catalog + สต็อกจาก ERP / POS + ตะกร้าจอง 15 นาที | BE2 | 5 | 2027-08-02 | F10 |
4.2.2 | Pick-up QR + ส่งถึงบ้าน + Tracking webhook | BE2 | 5 | | F10 | Pick-up 7 วัน, ค่าส่ง 50 บาท ฟรีเมื่อครบ 1,000, Kerry / Flash / ไปรษณีย์
4.2.3 | คืนสินค้า + e-Tax Invoice | BE2 | 3 | | 8 |
4.2.4 | สั่งอาหารล่วงหน้า | BE2 | 4 | | 8 | ขึ้นกับการเชื่อม POS
4.3 | จ่ายเงินในสวนและ POS
4.3.1 | สแกนจ่ายในสวน | BE1 | 5 | 2027-08-02 | S9, Q15 | Bill, QR บิลจาก POS, ใช้พ้อยได้, outbox bill.paid
4.3.2 | Member QR ที่ POS | BE1 | 4 | | 8 | QR เปลี่ยนทุก 60 วินาที สะสม / ใช้พ้อย
4.4 | โรงแรม / MICE
4.4.1 | ห้องพัก + ใบเสนอราคา MICE + มัดจำออนไลน์ | BE1 | 5 | 4.3.2 | F10, Q17 |
4.5 | แผนที่นำทางและ Audio Guide
4.5.1 | Content: เส้นทาง, Audio Guide, ข้อมูลพืช | TL | 5 | 4.1.2 | 1.2 | ไฟล์เสียงต่อจุดตามภาษา
4.5.2 | App: แผนที่ Interactive + นำทาง 4 แบบ + ออฟไลน์ | MB1 | 15 | 2027-08-09 | 8 |
4.5.3 | App: Audio Guide + สแกนป้ายต้นไม้ | MB2 | 10 | 2027-08-09 | 8 |
4.6 | ภาษาและสกุลเงินเพิ่ม
4.6.1 | RU / HI + แสดงราคาหลายสกุลเงิน | FE1 | 5 | 2027-08-09 | 8 | ชำระจริงเป็น THB
4.7 | Frontend และ Mobile เฟส 4
4.7.1 | เว็บ: Partner ร้านของฝาก โรงแรม / MICE | FE1 | 15 | 4.6.1 | 8 |
4.7.2 | Admin: Partner ส่วนแบ่ง สต็อก Shipment เส้นทาง Audio | FE2 | 20 | 2027-08-09 | 8 |
4.7.3 | App: ร้านของฝาก สแกนจ่าย Member QR สั่งอาหาร | MB2 | 10 | | 8 |
4.8 | QA และ Release เฟส 4
4.8.1 | ทดสอบเฟส 4 (รวม POS จริง) | QA1 | 15 | 2027-08-30 | 11 |
4.8.2 | ส่ง App ตรวจ + UAT เฟส 4 | NN | 8 | 2027-09-15 | 8 |
4.8.3 | Deploy เฟส 4 | DO | 2 | 2027-09-28 | 8 |
"""

# ความคืบหน้า: wbs → (สถานะ, %, หมายเหตุ) แก้ที่นี่แล้วรันสคริปต์ ไฟล์ทุกไฟล์จะอัปเดตตาม
# สถานะ: ยังไม่เริ่ม · กำลังทำ · เสร็จ · ติดปัญหา  (ไม่ระบุ = ยังไม่เริ่ม 0%)
STATUS = {
    "1.2.1": ("กำลังทำ", 60, "Figma Concept A ครบ Landing, Activities, Accommodations, Auditorium, Buy Tickets · ขาดหน้าสมัคร / พ้อย / Agent / ผู้ใช้บัตรรายใบ / ส่งบัตร (DESIGN.md หัวข้อ 5)"),
    "1.2.2": ("กำลังทำ", 50, "Tourist App 8 กลุ่มหน้าจอใน Figma · ยังไม่มี Agent App และหน้าสมัครแบบเต็ม"),
    "1.0.2": ("กำลังทำ", 0, "ตัวเลือกอยู่ใน system-design Q19, Q20 รอ TL ตัดสิน"),
    "1.15.1": ("กำลังทำ", 10, "รวบรวมคำถามแล้วใน docs/meeting-questions-and-data-request.md และ system-design Q1–Q3"),
}

# งานเตรียมการที่เสร็จแล้ว (ไม่อยู่ใน WBS)
PREWORK = [
    ("System design ร่าง 3 + diagram 25 ภาพ", "เสร็จ", "../system-design.md"),
    ("แผนพัฒนา + Gantt (Excel) + tasks.md", "เสร็จ", "../plan/Nongnooch_Dev_Plan.xlsx"),
    ("ดัชนีแผน implementation 16 แผน", "เสร็จ", "../superpowers/plans/2026-11-11-phase1-index.md"),
    ("Plan 00 Mock ระบบภายนอก (demo)", "เสร็จ (รอ review)", "../superpowers/plans/2026-11-11-00-mock-external.md"),
    ("Plan 01 Platform chassis", "เสร็จ (รอ review)", "../superpowers/plans/2026-11-11-01-platform-chassis.md"),
    ("Plan 02 Identity OTP + JWT", "เสร็จ (รอ review)", "../superpowers/plans/2026-11-11-02-identity-otp.md"),
    ("Plan 03–16", "ยังไม่เขียน", "../superpowers/plans/2026-11-11-phase1-index.md"),
    ("DESIGN.md จาก Figma", "เสร็จ · รอ Designer ยืนยัน 6 ข้อ (หัวข้อ 9)", "../design/DESIGN.md"),
]

PLANS = "../superpowers/plans/"
P00 = PLANS + "2026-11-11-00-mock-external.md"
P01, P02 = PLANS + "2026-11-11-01-platform-chassis.md", PLANS + "2026-11-11-02-identity-otp.md"
# wbs → (ลิงก์แผน, ส่วนของแผน) สำหรับงานที่มีแผน implementation แล้ว
PLAN_LINK = {
    "1.1.12": (P00, "Task 1–3"), "1.1.13": (P00, "Task 4"), "1.1.14": (P00, "Task 5"),
    "1.1.1": (P01, "Task 1–6"), "1.1.2": (P01, "Task 3, 7"), "1.1.3": (P01, "Task 1 (compose)"), "1.1.11": (P01, "Task 7"),
    "1.3.1": (P02, "Task 1–2"), "1.3.2": (P02, "Task 3–4"), "1.9.2": (P02, "Task 5"),
}
# กลุ่ม WBS → เลขแผนในดัชนี (ยังไม่เขียน)
PLAN_INDEX = {"1.3": "08", "1.4": "03", "1.5": "09", "1.6": "05 / 06", "1.7": "07", "1.8": "04", "1.9": "10",
              "1.10": "11", "1.11": "13", "1.12": "14", "1.13": "14", "1.14": "15", "1.1": "12 / 16"}
DEV_OWNERS = {"TL", "BE1", "BE2", "BE3", "FE1", "FE2", "MB1", "MB2", "DO"}

PHASE_COLOR = {"1": "16A34A", "2": "2563EB", "3": "EA580C", "4": "9333EA"}
PHASE_LIGHT = {"1": "DCFCE7", "2": "DBEAFE", "3": "FFEDD5", "4": "F3E8FF"}
TH_MONTH = "ม.ค. ก.พ. มี.ค. เม.ย. พ.ค. มิ.ย. ก.ค. ส.ค. ก.ย. ต.ค. พ.ย. ธ.ค.".split()


def is_work(d): return d.weekday() < 5 and d not in HOL


def next_work(d):
    while not is_work(d):
        d += timedelta(1)
    return d


def add_work(d, n):  # วันที่ n วันทำงานถัดไป
    while n > 0:
        d += timedelta(1)
        n -= is_work(d)
    return d


def parse():
    rows = []
    for line in DATA.strip().splitlines():
        if not line.strip():
            continue
        f = [x.strip() for x in line.split("|")]
        wbs, name = f[0], f[1]
        r = {"wbs": wbs, "name": name, "level": wbs.count("."), "phase": wbs.split(".")[0]}
        if len(f) > 2:
            r.update(owner=f[2], dur=int(f[3]), start_spec=f[4], ref=f[5], detail=f[6] if len(f) > 6 else "")
            r["status"], r["pct"], r["note"] = STATUS.get(wbs, ("ยังไม่เริ่ม", 0, ""))
        rows.append(r)
    return rows


def schedule(rows):
    by = {r["wbs"]: r for r in rows}
    prev = None
    for r in rows:  # ก่อนหน้า: ระบุเอง หรือ sibling ก่อนหน้า
        if "dur" not in r:
            prev = None
            continue
        spec = r["start_spec"]
        r["preds"] = [] if not spec or spec[:2] == "20" else spec.split(",")
        if not spec:
            assert prev, f"{r['wbs']}: งานแรกของกลุ่มต้องระบุวันเริ่มหรืองานก่อนหน้า"
            r["preds"] = [prev["wbs"]]
        prev = r

    def end(r):  # recursive เพราะอ้างอิงงานที่อยู่ถัดไปในตารางได้
        if "end" not in r:
            spec = r["start_spec"]
            s = date.fromisoformat(spec) if spec[:2] == "20" else PLAN_START
            for p in r["preds"]:
                s = max(s, end(by[p]) + timedelta(1))
            r["start"] = s if r["dur"] == 0 else next_work(s)
            r["end"] = r["start"] if r["dur"] == 0 else add_work(r["start"], r["dur"] - 1)
        return r["end"]

    for r in rows:
        if "dur" in r:
            end(r)
    # หัวข้อสรุป: min / max ของลูก
    for r in rows:
        if "dur" in r:
            continue
        kids = [k for k in rows if "dur" in k and k["wbs"].startswith(r["wbs"] + ".")]
        r["start"] = min(k["start"] for k in kids)
        r["end"] = max(k["end"] for k in kids)
    return rows


def check(rows):
    """เตือนเมื่อคนเดียวกันมีงานซ้อนกัน (ไม่นับ NN / ALL / งานทดสอบยาว)"""
    load = {}
    for r in rows:
        if r.get("dur", 0) <= 0 or r["owner"] in ("NN", "ALL") or r["dur"] >= 30:
            continue
        d = r["start"]
        while d <= r["end"]:
            if is_work(d):
                load.setdefault((r["owner"], d), []).append(r["wbs"])
            d += timedelta(1)
    over = {}
    for (o, d), w in load.items():
        if len(w) > 1:
            over.setdefault(o, set()).update(w)
    for o, w in sorted(over.items()):
        print(f"  ซ้อน {o}: {sorted(w)}")
    p1 = [r for r in rows if r["phase"] == "1" and r.get("dur", 0) > 0 and r["owner"] not in ("NN", "ALL", "QA1", "QA2", "DO")
          and r["wbs"].split(".")[1] not in ("14", "16", "17")]
    late = [r["wbs"] for r in p1 if r["end"] > date(2027, 1, 29)]
    print("  งานพัฒนาเฟส 1 ที่เลย M4 29 ม.ค.:", late or "ไม่มี")


# Excel อ่านสีพื้นของ conditional formatting จาก bgColor
def cf_fill(c): return PatternFill(fill_type="solid", start_color="FF" + c, end_color="FF" + c)


def dmy(d): return d.strftime("%d/%m/%y")


def workdays(r): return sum(is_work(r["start"] + timedelta(i)) for i in range((r["end"] - r["start"]).days + 1))


def write_xlsx(rows):
    wb = Workbook()
    ws = wb.active
    ws.title = "แผนงาน + Gantt"
    thin = Side(style="thin", color="D4D4D4")
    head_fill = PatternFill("solid", fgColor="1F2937")
    white = Font(bold=True, color="FFFFFF")
    cols = [("WBS", 8), ("งาน / Subtask", 52), ("ระดับ", 9), ("เฟส", 5), ("ผู้รับผิดชอบ", 10), ("เริ่ม", 10),
            ("สิ้นสุด", 10), ("วันทำงาน", 8), ("ก่อนหน้า", 9), ("อ้างอิง", 10), ("สถานะ", 11), ("%", 6)]
    G0 = len(cols) + 1  # คอลัมน์แรกของ Gantt
    week0 = PLAN_START - timedelta(PLAN_START.weekday())
    weeks = [week0 + timedelta(7 * i) for i in range(52)]

    ws.cell(1, 1, "แผนพัฒนาระบบดิจิทัลสวนนงนุช · เริ่ม 13/11/26").font = Font(bold=True, size=14)
    ws.cell(2, 1, "ที่มา: docs/system-design.md ร่าง 3 + ROADMAP.md v2.4 · แถบ Gantt คำนวณจากคอลัมน์ เริ่ม / สิ้นสุด แก้วันที่แล้วแถบเปลี่ยนตาม · "
                  "สี: เขียว เฟส 1 · ฟ้า เฟส 2 · ส้ม เฟส 3 · ม่วง เฟส 4 · แดง Milestone").font = Font(italic=True, color="6B7280")
    HR = 4  # header row (สัปดาห์), แถว 3 = เดือน
    for i, (h, w) in enumerate(cols, 1):
        c = ws.cell(HR, i, h)
        c.fill, c.font = head_fill, white
        c.alignment = Alignment(vertical="center", horizontal="center", wrap_text=True)
        ws.column_dimensions[get_column_letter(i)].width = w
    for j, wk in enumerate(weeks):
        col = G0 + j
        ws.column_dimensions[get_column_letter(col)].width = 3.2
        c = ws.cell(HR, col, wk)
        c.number_format = "dd"
        c.fill, c.font = head_fill, Font(color="FFFFFF", size=8)
        c.alignment = Alignment(horizontal="center")
        if j == 0 or wk.month != weeks[j - 1].month:
            m = ws.cell(3, col, f"{TH_MONTH[wk.month - 1]} {str(wk.year + 543)[2:]}")
            m.font = Font(bold=True, size=9)
    ws.row_dimensions[HR].height = 30
    DET = G0 + len(weeks)  # รายละเอียดอยู่หลัง Gantt
    c = ws.cell(HR, DET, "รายละเอียด")
    c.fill, c.font = head_fill, white
    ws.column_dimensions[get_column_letter(DET)].width = 100

    r0 = HR + 1
    for i, r in enumerate(rows):
        x = r0 + i
        kind = "Phase" if r["level"] == 0 else "Task" if "dur" not in r else "Milestone" if r["dur"] == 0 else "Subtask"
        indent = {"Phase": 0, "Task": 1, "Subtask": 2, "Milestone": 2}[kind]
        vals = [r["wbs"], r["name"], kind, int(r["phase"]), r.get("owner", ""), r["start"], r["end"],
                None if kind == "Milestone" else f"=NETWORKDAYS(F{x},G{x},Holidays)",
                ", ".join(r.get("preds", [])), r.get("ref", ""), r["status"] if "dur" in r else "",
                r["pct"] / 100 if "dur" in r else None]
        for j, v in enumerate(vals, 1):
            c = ws.cell(x, j, v)
            c.border = Border(bottom=thin)
            if j in (6, 7):
                c.number_format = "dd/mm/yy"
            if j == 12 and v is not None:
                c.number_format = "0%"
        ws.cell(x, 2).alignment = Alignment(indent=indent)
        ws.cell(x, DET, r.get("detail", ""))
        if kind == "Phase":
            for j in range(1, G0):
                ws.cell(x, j).fill = PatternFill("solid", fgColor=PHASE_COLOR[r["phase"]])
                ws.cell(x, j).font = white
        elif kind == "Task":
            for j in range(1, G0):
                ws.cell(x, j).fill = PatternFill("solid", fgColor=PHASE_LIGHT[r["phase"]])
                ws.cell(x, j).font = Font(bold=True)
        elif kind == "Milestone":
            ws.cell(x, 2).font = Font(bold=True, color="B91C1C")
            ws.cell(x, 2).value = "◆ " + r["name"]
    last = r0 + len(rows) - 1

    # Gantt ด้วย conditional formatting
    g1, gN = get_column_letter(G0), get_column_letter(G0 + len(weeks) - 1)
    rng = f"{g1}{r0}:{gN}{last}"
    hit = f"AND($F{r0}<={g1}${HR}+6,$G{r0}>={g1}${HR})"
    rules = [(f'AND($C{r0}="Milestone",{hit})', "DC2626")]
    for ph, col in PHASE_COLOR.items():
        rules.append((f'AND($C{r0}<>"Subtask",$D{r0}={ph},{hit})', col))
        rules.append((f'AND($D{r0}={ph},{hit})', {"1": "4ADE80", "2": "60A5FA", "3": "FB923C", "4": "C084FC"}[ph]))
    for f, col in rules:
        ws.conditional_formatting.add(rng, FormulaRule(formula=[f], fill=cf_fill(col), stopIfTrue=True))
    ws.conditional_formatting.add(f"{g1}{HR}:{gN}{HR}", FormulaRule(
        formula=[f"AND(TODAY()>={g1}${HR},TODAY()<{g1}${HR}+7)"], fill=cf_fill("FACC15")))
    ws.conditional_formatting.add(f"K{r0}:K{last}", FormulaRule(formula=[f'K{r0}="เสร็จ"'], fill=cf_fill("BBF7D0")))
    ws.conditional_formatting.add(f"K{r0}:K{last}", FormulaRule(formula=[f'K{r0}="ติดปัญหา"'], fill=cf_fill("FECACA")))

    dv = DataValidation(type="list", formula1='"ยังไม่เริ่ม,กำลังทำ,เสร็จ,ติดปัญหา"', allow_blank=True)
    ws.add_data_validation(dv)
    dv.add(f"K{r0}:K{last}")
    ws.freeze_panes = ws.cell(r0, 3)
    ws.auto_filter.ref = f"A{HR}:L{last}"

    # Milestones
    ms = wb.create_sheet("Milestones")
    ms.append(["WBS", "Milestone", "วันที่", "ผู้รับผิดชอบ", "เกณฑ์ผ่าน / หมายเหตุ"])
    for r in rows:
        if r.get("dur") == 0:
            ms.append([r["wbs"], r["name"], r["start"], r["owner"], r["detail"]])
    for c in ms[1]:
        c.fill, c.font = head_fill, white
    for row in ms.iter_rows(min_row=2, min_col=3, max_col=3):
        row[0].number_format = "dd/mm/yy"
    for col, w in zip("ABCDE", (8, 55, 11, 12, 80)):
        ms.column_dimensions[col].width = w

    tm = wb.create_sheet("ทีม")
    tm.append(["รหัส", "บทบาท", "จำนวน", "งานหลัก"])
    for t in TEAM:
        tm.append(list(t))
    for c in tm[1]:
        c.fill, c.font = head_fill, white
    for col, w in zip("ABCD", (8, 36, 8, 80)):
        tm.column_dimensions[col].width = w

    hs = wb.create_sheet("Holidays")
    hs.append(["วันที่", "วันหยุด"])
    for d, n in HOLIDAYS.items():
        hs.append([date.fromisoformat(d), n])
    for row in hs.iter_rows(min_row=2, max_col=1):
        row[0].number_format = "dd/mm/yy"
    hs.column_dimensions["A"].width, hs.column_dimensions["B"].width = 11, 34
    wb.defined_names["Holidays"] = DefinedName("Holidays", attr_text=f"Holidays!$A$2:$A${len(HOLIDAYS) + 1}")

    path = OUT / "Nongnooch_Dev_Plan.xlsx"
    try:
        wb.save(path)
    except PermissionError:  # ไฟล์เปิดค้างใน Excel
        path = path.with_stem(path.stem + "_new")
        wb.save(path)
    return path


def write_md(rows):
    L = ["# แผนพัฒนาระบบดิจิทัลสวนนงนุช: Task และ Subtask", "",
         "**เริ่มแผน:** 13/11/26 · **ที่มา:** [system-design.md](../system-design.md) ร่าง 3 และ [ROADMAP.md](../../roadmap/ROADMAP.md) v2.4",
         "**ไฟล์คู่กัน:** [Nongnooch_Dev_Plan.xlsx](Nongnooch_Dev_Plan.xlsx) (Gantt) · สร้างใหม่ทั้งสองไฟล์ด้วย `python docs/plan/build_plan.py` ห้ามแก้ไฟล์นี้ตรง ๆ", "",
         "วันทำงาน = จันทร์–ศุกร์ ไม่นับวันหยุดราชการ (ดูชีต Holidays) · `ต่อจาก` = งานที่ต้องเสร็จก่อน · รหัส F / S / O / Q อ้างอิง system design, ตัวเลขล้วน / E / M อ้างอิง ROADMAP", "",
         "## สมมติฐานของแผน", "",
         "- ทีมตาม ROADMAP หัวข้อ 10 **บวก BE3 (Backend Go คนที่ 3) ตั้งแต่ 1 ธ.ค.** ตามความเสี่ยง Q21 งานของ BE3 (Notification, Report, sync ระบบเดิม, ช่องทางจ่ายต่างชาติ) รวม 26 วันทำงาน ถ้าไม่เพิ่มคน TL (ว่างหลัง 30/12/26) และ BE2 (ว่างหลัง 18/01/27) รับได้พอดี M4 แต่ไม่เหลือเวลา code review และแก้บั๊ก",
         "- FE2 ยังรออนุมัติ (#25) ถ้าไม่มี งาน Admin / Gate Scanner ต้องเลื่อนหรือตัดขอบเขต",
         "- Mobile 2 คนเริ่ม 1 ธ.ค., DevOps เต็มเวลา ธ.ค.–เม.ย. ก่อนหน้านั้น Tech Lead ตั้ง Staging ชั่วคราวให้ CMS ใช้ได้ก่อน",
         "- เฟส 2–4 ใช้ทีมเดิม ระดับรายละเอียดเป็น task หลัก แตกละเอียดเพิ่มก่อนเริ่มแต่ละเฟส", "",
         "## ทีม", "", "| รหัส | บทบาท | งานหลัก |", "|---|---|---|"]
    L += [f"| {c} | {n} | {w} |" for c, n, _, w in TEAM]
    L += ["", "## Milestone", "", "| วันที่ | WBS | Milestone | ผู้รับผิดชอบ |", "|---|---|---|---|"]
    L += [f"| {dmy(r['start'])} | {r['wbs']} | {r['name']} | {r['owner']} |" for r in rows if r.get("dur") == 0]
    for r in rows:
        if r["level"] == 0:
            L += ["", f"## {r['name']}", "", f"{dmy(r['start'])} – {dmy(r['end'])}"]
        elif "dur" not in r:
            L += ["", f"### {r['wbs']} {r['name']}", "", f"{dmy(r['start'])} – {dmy(r['end'])} ({workdays(r)} วันทำงาน)", ""]
        else:
            when = dmy(r["start"]) if r["dur"] == 0 else f"{dmy(r['start'])} – {dmy(r['end'])} · {r['dur']} วัน"
            tag = "◆ " if r["dur"] == 0 else ""
            meta = [r["owner"], when]
            if r["preds"]:
                meta.append("ต่อจาก " + ", ".join(r["preds"]))
            if r["ref"]:
                meta.append("อ้างอิง " + r["ref"])
            L.append(f"- [ ] **{tag}{r['wbs']} {r['name']}** · " + " · ".join(meta))
            if r["detail"]:
                L.append(f"  - {r['detail']}")
    path = OUT / "tasks.md"
    path.write_text("\n".join(L) + "\n", encoding="utf-8")
    return path


def plan_of(r):
    if r["wbs"] in PLAN_LINK:
        link, part = PLAN_LINK[r["wbs"]]
        return f"[{Path(link).stem[11:]} · {part}]({link})", True
    if r["owner"] not in DEV_OWNERS or r["phase"] != "1":
        return "ไม่ต้องมี (ไม่ใช่งานโค้ด หรือเฟสถัดไป)", None
    group = ".".join(r["wbs"].split(".")[:2])
    n = PLAN_INDEX.get(group)
    return (f"ยังไม่เขียน · Plan {n} ใน[ดัชนี]({PLANS}2026-11-11-phase1-index.md)" if n else "ยังไม่เขียน"), False


def progress(items):  # ถ่วงด้วยจำนวนวันทำงาน
    tasks = [r for r in items if r.get("dur", 0) > 0]
    w = sum(r["dur"] for r in tasks)
    return round(sum(r["dur"] * r["pct"] for r in tasks) / w) if w else 0


def bar(p): return "█" * (p // 10) + "░" * (10 - p // 10) + f" {p}%"


def write_task_files(rows):
    d = OUT.parent / "tasks"
    d.mkdir(exist_ok=True)
    for old in d.glob("[0-9]*.md"):
        old.unlink()
    by = {r["wbs"]: r for r in rows}
    succ = {}
    for r in rows:
        for p in r.get("preds", []):
            succ.setdefault(p, []).append(r["wbs"])
    leaves = [r for r in rows if "dur" in r]
    for r in leaves:
        group = by[".".join(r["wbs"].split(".")[:2])]
        plan, ready = plan_of(r)
        ms = r["dur"] == 0
        when = dmy(r["start"]) if ms else f"{dmy(r['start'])} – {dmy(r['end'])} ({r['dur']} วันทำงาน)"
        link = lambda ws: ", ".join(f"[{w} {by[w]['name']}]({w}.md)" for w in ws) or "–"
        L = [f"# {r['wbs']} {'◆ ' if ms else ''}{r['name']}", "",
             f"> สร้างอัตโนมัติจาก `docs/plan/build_plan.py` ห้ามแก้ไฟล์นี้ตรง ๆ · [กลับไปรายการ task](README.md)", "",
             "| | |", "|---|---|",
             f"| สถานะ | **{r['status']}** · {r['pct']}%{' · ' + r['note'] if r['note'] else ''} |",
             f"| เฟส / กลุ่ม | เฟส {r['phase']} · {group['wbs']} {group['name']} |",
             f"| ประเภท | {'Milestone' if ms else 'Task'} |",
             f"| ผู้รับผิดชอบ | {r['owner']} |",
             f"| กำหนดการ | {when} |",
             f"| ต้องรอ | {link(r['preds'])} |",
             f"| งานที่รอ task นี้ | {link(succ.get(r['wbs'], []))} |",
             f"| อ้างอิง | {r['ref'] or '–'} |",
             f"| แผน implementation | {plan} |", "",
             "## รายละเอียด", "", r["detail"] or "–", "",
             "## เกณฑ์เสร็จ (Definition of Done)", ""]
        if ms:
            L += ["- เกณฑ์ผ่านตามรายละเอียดข้างบนครบ และ PM บันทึกผลในที่ประชุมรายสัปดาห์"]
        elif r["owner"] in DEV_OWNERS:
            L += ["- ทำตามแผน implementation ครบทุก step และ test ที่เกี่ยวข้องผ่าน (`make test`, `make test-int`)",
                  "- **build และ test ใน Docker เท่านั้น** (`make test`, `make test-int`, `make build`) ห้ามรัน `go` บนเครื่องโดยตรง",
                  "- ผ่าน code review (subagent reviewer หรือ TL) และ merge เข้า main",
                  "- deploy ขึ้น Staging ได้ และ OpenAPI / proto / event schema อัปเดตตามที่เปลี่ยน",
                  "- ทำตาม Global Constraints ของ system design (1 schema ต่อ service, outbox, consumer idempotent, 7 ภาษา)"]
            if r["owner"] in ("FE1", "FE2", "MB1", "MB2", "UX"):
                L.append("- UI ตรงกับ [DESIGN.md](../design/DESIGN.md) และผ่าน WCAG 2.1 AA บนหน้าที่เกี่ยวข้อง")
        elif r["owner"].startswith("QA"):
            L += ["- มีรายงานผลทดสอบ (ผ่าน / ไม่ผ่าน / บั๊กที่เปิด) แนบในระบบติดตามงาน",
                  "- ไม่มีบั๊ก Critical / High ที่ยังไม่ได้เปิด ticket"]
        else:
            L += ["- ส่งมอบตามรายละเอียดข้างบน และ PM ยืนยันว่าได้รับครบ"]
        if ready is not None:
            L += ["", "## สำหรับ subagent", ""]
            if ready:
                L += ["- พร้อมเริ่ม: ใช้ skill `superpowers:subagent-driven-development` กับแผนที่ลิงก์ไว้ ทำเฉพาะส่วนที่ระบุ",
                      "- อ่าน spec ที่แผนอ้างถึง: [system-design.md](../system-design.md)" + (" และ [DESIGN.md](../design/DESIGN.md)" if r["owner"] in ("FE1", "FE2", "MB1", "MB2") else "")]
            else:
                L += ["- **ยังไม่พร้อม:** ต้องเขียนแผน implementation ด้วย skill `superpowers:writing-plans` ก่อนส่งให้ subagent"]
            L += ["- งานเสร็จแล้วให้อัปเดต `STATUS` ใน `docs/plan/build_plan.py` แล้วรัน `python docs/plan/build_plan.py`"]
        (d / f"{r['wbs']}.md").write_text("\n".join(L) + "\n", encoding="utf-8")

    # README
    subs = [r for r in leaves if r["dur"] > 0]
    cnt = lambda s: sum(r["status"] == s for r in subs)
    p1dev = [r for r in subs if r["phase"] == "1" and r["owner"] in DEV_OWNERS]
    ready = sum(plan_of(r)[1] is True for r in p1dev)
    R = ["# รายการ Task ทั้งหมด", "",
         f"> สร้างอัตโนมัติจาก `docs/plan/build_plan.py` · อัปเดตล่าสุด {dmy(date.today())} · แผนเริ่ม 13/11/26",
         "> Gantt: [Nongnooch_Dev_Plan.xlsx](../plan/Nongnooch_Dev_Plan.xlsx) · Design: [DESIGN.md](../design/DESIGN.md) · Spec: [system-design.md](../system-design.md)", "",
         "## ภาพรวมความคืบหน้า", "",
         f"**ทั้งโครงการ:** `{bar(progress(subs))}` (ถ่วงด้วยวันทำงาน)", "",
         f"| Task ทั้งหมด | เสร็จ | กำลังทำ | ติดปัญหา | ยังไม่เริ่ม | Milestone |",
         "|---|---|---|---|---|---|",
         f"| {len(subs)} | {cnt('เสร็จ')} | {cnt('กำลังทำ')} | {cnt('ติดปัญหา')} | {cnt('ยังไม่เริ่ม')} | {len(leaves) - len(subs)} |", "",
         f"**พร้อมส่ง subagent (มีแผน implementation แล้ว):** {ready} จาก {len(p1dev)} task โค้ดของเฟส 1 · ที่เหลือต้องเขียนแผนก่อน ตามลำดับใน[ดัชนีแผน]({PLANS}2026-11-11-phase1-index.md)", "",
         "### งานเตรียมการที่ทำแล้ว", "", "| งาน | สถานะ |", "|---|---|"]
    R += [f"| [{n}]({l}) | {s} |" for n, s, l in PREWORK]
    R += ["", "### ความคืบหน้ารายกลุ่ม", "", "| กลุ่ม | ช่วงเวลา | ความคืบหน้า | เสร็จ / ทั้งหมด |", "|---|---|---|---|"]
    for g in rows:
        if "dur" in g:
            continue
        kids = [k for k in subs if k["wbs"].startswith(g["wbs"] + ".")]
        name = f"**{g['name']}**" if g["level"] == 0 else f"{g['wbs']} {g['name']}"
        R.append(f"| {name} | {dmy(g['start'])} – {dmy(g['end'])} | `{bar(progress(kids))}` | "
                 f"{sum(k['status'] == 'เสร็จ' for k in kids)} / {len(kids)} |")
    R += ["", "## รายการ Task", "", "สัญลักษณ์แผน: ✅ มีแผน implementation พร้อมส่ง subagent · 📝 ต้องเขียนแผนก่อน · – ไม่ใช่งานโค้ด / เฟสถัดไป"]
    for r in rows:
        if r["level"] == 0:
            R += ["", f"### {r['name']}", ""]
        elif "dur" not in r:
            R += ["", f"#### {r['wbs']} {r['name']}", "",
                  "| WBS | งาน | ผู้รับผิดชอบ | เริ่ม | สิ้นสุด | วัน | สถานะ | % | แผน |", "|---|---|---|---|---|---|---|---|---|"]
        else:
            mark = {True: "✅", False: "📝", None: "–"}[plan_of(r)[1]]
            nm = ("◆ " if r["dur"] == 0 else "") + r["name"]
            R.append(f"| [{r['wbs']}]({r['wbs']}.md) | {nm} | {r['owner']} | {dmy(r['start'])} | {dmy(r['end'])} | "
                     f"{r['dur'] or '–'} | {r['status']} | {r['pct']}% | {mark} |")
    R += ["", "## วิธีอัปเดตความคืบหน้า", "",
          "1. แก้ `STATUS` ใน [docs/plan/build_plan.py](../plan/build_plan.py) เช่น `\"1.3.1\": (\"เสร็จ\", 100, \"merge PR #12\")`",
          "2. รัน `python docs/plan/build_plan.py`",
          "3. Excel, tasks.md, README นี้ และไฟล์ของแต่ละ task จะอัปเดตพร้อมกัน (ปิดไฟล์ Excel ก่อนรัน)"]
    (d / "README.md").write_text("\n".join(R) + "\n", encoding="utf-8")
    return d


if __name__ == "__main__":
    rows = schedule(parse())
    check(rows)
    print(write_xlsx(rows))
    print(write_md(rows))
    print(write_task_files(rows))
