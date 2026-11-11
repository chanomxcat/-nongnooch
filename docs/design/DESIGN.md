# DESIGN.md: ระบบดิจิทัลสวนนงนุช

**ที่มา:** Figma [Nongnooch Design 27-09-69](https://www.figma.com/design/6ko8v4FAdat6fqknJMaf7Q/Nongnooch-Design-27-09-69) ดึงเมื่อ 11 พ.ย. 2026
**ขอบเขต:** Website ใช้ **Concept A** (page " Website") · Mobile web แบบ responsive (page "Design Mobile") · Tourist App (page "App")
**ใช้คู่กับ:** [system-design.md](../system-design.md) · [ROADMAP.md](../../roadmap/ROADMAP.md) หัวข้อ 4.5 · ภาพอ้างอิงใน [ref/](ref/)

> **ไม่ใช้:** Concept B และ Concept C (Night Garden / Minimal) ที่อยู่ใน page เดียวกัน ส่วน component ที่ชื่อลงท้ายด้วย `/ Concept B` ก็ไม่ใช้เช่นกัน
> **ข้อมูลตัวอย่าง:** ราคา โปรโมชัน และวันที่ในไฟล์ Figma เป็นข้อมูลจำลอง (mock) และรูปสร้างด้วย Figma AI ข้อความทั้งหมดต้องดึงจาก CMS ห้ามฝังลงในโค้ด

ค่าทั้งหมดดึงจาก Figma variables และ design context ด้วยบัญชี Full seat เมื่อ 11 พ.ย. 2026 ส่วนค่าที่มีป้าย **[ค่าคงที่ใน Figma]** เป็นค่าที่ Designer ใส่เป็นตัวเลขตรง ๆ ไม่ได้ผูกกับ variable ให้ทำเป็น token ฝั่งโค้ด และแจ้ง Designer ให้สร้าง variable ตาม (ดูหัวข้อ 9)

---

## 1. หลักการออกแบบ

- **สวนพฤกษศาสตร์ระดับพรีเมียม:** ใช้เขียวเข้ม (`brand-primary` `#074536`) คู่กับทอง (`brand-secondary` `#D9A52E`) บนพื้นครีม / ขาว และให้ภาพสวนขนาดใหญ่เป็นตัวเล่าเรื่อง
- **มาสคอตไดโนเสาร์** (เต่า / ไดโนเสาร์สวมแว่น) โผล่ตามขอบ section เพื่อความเป็นมิตร ใช้เป็นภาพตกแต่งเท่านั้น ใส่ `alt=""`
- **มุมมน ขอบนุ่ม:** section หลักโค้งที่มุมล่าง ปุ่มเป็นทรง pill และการ์ดมีมุมโค้ง
- **การเคลื่อนไหวต้องมีจุดประสงค์** (หัวข้อ 6) และปิดทั้งหมดเมื่อผู้ใช้ตั้ง `prefers-reduced-motion`

## 2. Design tokens

ชื่อ token ตรงกับ Figma variables (`--color-*`, `--spacing-*`, `--alias-*`) ให้ใช้ชื่อนี้ทั้งใน CSS ของ Next.js และใน `ThemeData` ของ Flutter

### 2.1 สี

| Token | ค่า | ใช้กับ |
|---|---|---|
| `color/brand/primary` = `color/green/900` | `#074536` | Nav, ปุ่มหลักในแอป, การ์ดที่เลือก, หัวข้อ |
| `color/green/950` | `#00271C` | พื้น section มืด, footer, overlay บนภาพ |
| `color/brand/primary-subtle` | `#F5F7F6` | พื้นของบล็อกรอง |
| `color/brand/secondary` = `color/gold/500` | `#D9A52E` | CTA "Buy Tickets", ปุ่มลูกศร, ตัวเลขขั้นตอน, เส้น progress |
| `color/brand/secondary-hover` | `#B58612` | hover ของปุ่มทอง |
| `color/gold/400` | `#E1B865` | hover / ขอบทอง |
| `color/gold/300` | `#E9CA8E` | ข้อความทองบนพื้นเขียว (subtitle ของ hero) |
| `color/gold/100` | `#F6EAD2` | พื้นทองอ่อน |
| `color/brand/secondary-subtle` | `#FBF6EC` | พื้นครีมของ section, กล่อง "Please note" |
| `color/emerald/400` | `#7CC489` | จุดเน้นสีเขียวอ่อน |
| `color/text/default` | `#2A2E37` | ข้อความหลัก |
| `color/text/subtle` = `grey/600` | `#5A606D` | ข้อความรอง |
| `color/text/disabled` = `border/strong` | `#898E9A` | ข้อความที่ปิดใช้, เส้นขอบเข้ม |
| `color/text/brand` | `#074536` | หัวข้อสีแบรนด์ |
| `color/text/inverse`, `color/white` | `#FFFFFF` | ข้อความบนพื้นเข้ม |
| `color/surface/default` | `#FFFFFF` | การ์ด, พื้นหลังหลัก |
| `color/surface/subtle` | `#F6F7F8` | พื้นหน้า Buy Tickets, segmented control |
| `color/surface/brand` | `#074536` | พื้นแบรนด์ |
| `color/border/default` = `grey/200` | `#C3C6CC` | เส้นขอบการ์ด / input |
| `color/grey/100`, `/300`, `/500` | `#DFE1E4`, `#A6ABB3`, `#6B7280` | เส้นแบ่ง, ไอคอนที่ไม่ active |
| `color/status/success` | `#16A34A` | ป้าย FREE, เครื่องหมายถูก |
| `color/status/error` | `#DC2626` | ข้อผิดพลาด, จุดแจ้งเตือน |

**คอนทราสต์ (WCAG 2.1 AA ตาม ROADMAP E1):** ทองเป็นสีพื้นของปุ่มได้ แต่ข้อความบนปุ่มทองต้องเป็น `green/900` หรือ `green/950` ห้ามใช้ข้อความสีขาว และห้ามใช้ทองเป็นสีข้อความขนาดเล็กบนพื้นขาว ข้อความขนาด ≥ 24px หรือตัวหนา ≥ 18.66px ใช้ทองบนพื้นขาวได้

### 2.2 ตัวอักษร

ไฟล์ Figma ใช้ **Noto Sans Thai** ทุกระดับ

| Style | Desktop ขนาด / line-height | Mobile (390) | น้ำหนัก |
|---|---|---|---|
| Heading/H1 | 60 / 72 [ค่าคงที่ใน Figma] (หัว Hero) | 36 / 44 | 700 |
| Heading/H2 | 36 / 43 (หัว section เช่น "Eat, Shop & Take Home") | 28 / 36 | 700 |
| Heading/H3 | ไม่ได้ใช้ใน Concept A desktop | 24 / 32 (หัว section บนมือถือ) | 700 |

| Style | ขนาด / line-height (ทุกขนาดจอ) | น้ำหนัก |
|---|---|---|
| Heading/H4 | 28 / 33 | 600 |
| Heading/H5 | 25 / 30 | 600 |
| Heading/H6 | 22 / 33 | 600 |
| Body/Body 1 | 18 / 27 | 400 |
| Body/Body 2 | 16 / 24 | 400 |
| Body/Body 3 | 14 / 21 | 400 |
| Label/Label 1 | 14 / 21 | 500 |
| Label/Label 2 | 12 / 18 | 500 |
| Label/Label 3 | 11 / 16 | 500 |

letter-spacing เป็น 0 ทุกระดับ ใช้ H1–H3 แบบ fluid ได้ เช่น `font-size: clamp(36px, 2.5vw + 22px, 60px)` สำหรับ H1 ส่วน Label ใน Figma มีทั้ง weight 500, 600 (ปุ่ม, เมนู) และ 700 (ตัวเลข, badge) ให้ทำ modifier `--semibold` และ `--bold`

**ฟอนต์สำหรับ 7 ภาษา** (Figma ออกแบบไว้แค่ TH / EN ส่วนนี้เพิ่มเพื่อรองรับระบบจริง):

```css
--font-sans: "Noto Sans Thai", "Noto Sans", "Noto Sans SC", "Noto Sans JP", "Noto Sans KR", system-ui, sans-serif;
```

- **โฮสต์ฟอนต์เองบนโดเมนของเรา (ผ่าน Cloudflare)** ห้ามโหลดจาก Google Fonts เพราะถูกบล็อกในจีน (ROADMAP E1: "ไม่พึ่งบริการที่ถูกบล็อกในจีนบนหน้าสมัคร")
- โหลดฟอนต์ตามภาษาของหน้า เช่น `/zh` โหลด SC, `/ja` โหลด JP, `/ko` โหลด KR ใช้ `unicode-range` + `font-display: swap` เพื่อรักษา LCP ≤ 2.5 วินาที
- ภาษา CJK ให้ line-height ของ Body ไม่ต่ำกว่า 1.6 ส่วนภาษา DE ต้องรองรับคำยาว ให้ใช้ `hyphens: auto` + `overflow-wrap: anywhere` บนหัวข้อและปุ่ม (Q23)
- Flutter: ใส่ไฟล์ฟอนต์ไว้ใน assets ของแอป (HMS build ไม่มี Google services)

### 2.3 ระยะ (spacing)

Token ใน Figma (`--spacing-{n}` เป็น px): **4, 8, 10, 12, 16, 24, 32, 48, 96, 128**

| ใช้ที่ | ค่า |
|---|---|
| padding ของ section (desktop) | บน-ล่าง 128 · ซ้าย-ขวา 120 (เท่ากับ margin ของ grid) |
| ระยะระหว่างหัว section กับเนื้อหา | 40 [ค่าคงที่ใน Figma] |
| ระยะระหว่างการ์ดใน grid | 24 |
| padding ในการ์ดสินค้า | 20 [ค่าคงที่ใน Figma] · การ์ดขั้นตอน Buy Tickets 28 |
| ปุ่ม pill | 32 × 16 (ปุ่มใหญ่) · 24 × 12 (ปุ่มใน Nav) · chip 18 × 8 · tag 10 × 4 |
| Nav | padding ซ้าย-ขวา 48 (ไม่ใช่ 120) |

ค่าที่ใช้บ่อยแต่ยังไม่เป็น token: 6, 14, 20, 28, 40 ให้เพิ่มเป็น token ฝั่งโค้ด และแจ้ง Designer

### 2.4 Grid และ breakpoint (page "Layout Guide")

| Breakpoint | ความกว้าง frame | คอลัมน์ | Margin | Gutter |
|---|---|---|---|---|
| Mobile | 390 | 4 | 16 | 16 |
| Tablet | 768 | 8 | 32 | 24 |
| Desktop | 1440 | 12 | 120 | 24 |

ความกว้างของเนื้อหาบน desktop คือ 1200px (1440 − 2×120) ทุก section ใน Concept A ใช้ความกว้างนี้ ส่วน CSS breakpoint ที่ใช้ได้แก่ `< 768`, `768–1279`, `≥ 1280`

### 2.5 มุมโค้ง

| Token | ค่า | ใช้กับ |
|---|---|---|
| `radius/full` | 9999 | ปุ่ม pill, chip, tag, icon button, segmented control, stepper |
| `radius/large` | 16 | การ์ดสินค้า, แถวจำนวนบัตร, การ์ดในแอป (ใช้มากที่สุดในแอป) |
| `radius/xlarge` (เสนอ) | 24 [ค่าคงที่ใน Figma] | การ์ดกิจกรรมใน Hero, การ์ดขั้นตอนใน Buy Tickets, การ์ดเลือก pass |
| `radius/medium` (เสนอ) | 12–14 [ค่าคงที่ใน Figma] | ช่องวันที่, input, ไอคอนเมนูในแอป |
| `radius/mobile-card` (เสนอ) | 18–20 [ค่าคงที่ใน Figma] | การ์ดบน mobile web และ bottom sheet ในแอป |
| แถบ progress | 2 | Progress bar สูง 3 |

Figma ใช้ค่ามุมโค้งตรง ๆ หลายค่า (8, 10, 12, 14, 18, 20, 24) ฝั่งโค้ดให้ปัดเหลือ 4 ระดับ คือ 12 / 16 / 20 / 24 และให้ Designer ยืนยัน

### 2.6 เงา (Elevation)

| Token | ค่า |
|---|---|
| Level 1 | `0 1px 2px #071C1614` |
| Level 2 | ไม่มี variable ใน Figma ให้ใช้ `0 2px 6px #071C161A` (ค่ากลางระหว่าง Level 1 และ 3) สำหรับ Nav ตอน scroll [ต้องให้ Designer ยืนยัน] |
| Level 3 | `0 4px 8px #071C161F` |
| Level 4 | `0 6px 18.6px #071C1652` (การ์ดตอน hover) |
| Level 5 | `0 16px 32px #071C1629` |
| Glow / Gold | `0 12px 32px rgba(217,166,46,.35)` ใช้กับปุ่มทองหลักใน Buy Tickets [ค่าคงที่ใน Figma] |
| Glow / Green | `0 8px 20px rgba(8,69,54,.25)` ใช้กับการ์ด pass ที่เลือก [ค่าคงที่ใน Figma] |

สีของเงาบนเว็บใช้เขียวเข้ม `#071C16` ไม่ใช้สีดำ แต่หน้าจอ App ใน Figma ใช้เงาดำ (`rgba(0,0,0,.08)` ถึง `.28`) และ bottom nav มีเงาด้านใน (inset) สีขาว ให้ใช้ Level 1–5 ชุดเดียวกันในแอปด้วย เพื่อให้สอดคล้องกัน

**Nav:** พื้น `green/950` + `backdrop-filter: blur(12px)` · Utility bar สูง 40 พื้น `rgba(0,20,15,.45)` · Main nav สูง 88 · เมนูที่ active มีขีดทองกว้าง 24 สูง 2 ใต้ข้อความ · ปุ่ม "Buy Tickets" พื้นทอง ข้อความ `brand/primary`
**Hero overlay:** ซ้อน 3 gradient ได้แก่ ล่างจางเป็นครีม `#FBF6EC`, ซ้ายเข้ม `rgba(3,41,31,.85)` → โปร่ง และบนลงล่าง `rgba(3,33,26,.65)` → `rgba(0,38,28,.95)` เพื่อให้ข้อความขาวอ่านได้บนภาพทุกภาพ

## 3. Components

### 3.1 Website (page "Components")

| Component | Variant / State | หมายเหตุ |
|---|---|---|
| Navigation Bar / Desktop | Active = Home, Activities, Zone, Auditorium, Accommodation | สูง 128 = Utility bar 40 + Main nav 88, sticky (Motion A) |
| Footer / Desktop | – | 4 คอลัมน์ Explore · Plan · About · Contact ต้องเพิ่มลิงก์ "สำหรับ Agent" (ROADMAP 4.5) |
| Button / Pill | Style = White, Gold, Dark, Outline Light, Outline Dark × State = Default, Hover | สูง 56 |
| Icon Button / Arrow | Direction = Left / Right × Style = Light / Gold | 48×48 |
| Hero Slider / Desktop | 6 slide (Beautiful Garden … Botanical Garden) | 1440×915, card preview ด้านขวา, ตัวนับ 01 / 06 |
| Promo Slider | Desktop 1200×400 · Mobile 358×148, 3 slide | Motion J |
| Stat Item | – | ตัวเลข count-up (Motion E) |
| Activity Item | Default / Hover / Active | – |
| Bento Card | Size = Large, Wide, Small × Default / Hover | – |
| Service Card | Accommodation, MICE, Restaurant, Service × Default / Hover | 282×440 |
| News Featured Card, News List Item | Default / Hover | – |
| Exploding Gallery / Desktop | Gathered → Explode → Reveal | ตกแต่งเท่านั้น ปิดเมื่อ reduced motion |
| Gallery Spotlight Coverflow, Photo Gallery, Gallery Lightbox | Active / Image = 1–6 | Lightbox ต้องกด Esc ปิดได้ ใช้ลูกศรคีย์บอร์ดได้ และ focus ต้องถูกขังไว้ใน lightbox |
| Newsletter CTA / Desktop | – | เปลี่ยนเป็น "ชวนสมัครสมาชิก + ยินยอมรับข่าวสาร" ตาม ROADMAP 4.5 |
| Locale Popup | Tab = Languages / Currency | เฟส 1 มี 7 ภาษา **ไม่มี Russian** และใช้ THB อย่างเดียว ให้ซ่อนแท็บ Currency |
| Chatbot / Desktop | Closed / Hover / Open | **เฟส 2** ไม่ทำในเฟส 1 |

### 3.2 Buy Tickets (Concept A · Buy Tickets, ภาพ [assets/figma-buy-tickets.png](../../assets/figma-buy-tickets.png))

มี 4 ขั้นในการ์ดแยกกัน และมี Order Summary แบบ sticky กว้าง 400 ด้านขวา (desktop)

1. **Choose your pass:** การ์ดภาพ Day Pass / Annual Pass การ์ดที่เลือกมีพื้น `green/900` + ขอบทอง + ป้าย "Selected"
2. **Who's visiting?:** segmented control Thai / Foreigner
3. **Select visit date:** แถบวันเลื่อนได้ + "Choose another date"
4. **Number of tickets:** แถว Adult / Child + stepper ทรง pill (ปุ่ม + เป็นทอง) + กล่องเส้นประเขียว "Children under 90 cm … FREE"

ป้ายเลขขั้นตอนเป็นวงกลม `green/900` ตัวเลขขาว ส่วนกล่อง "Please note" ใช้พื้น `brand/secondary-subtle`

### 3.3 Tourist App (page "App" → "App / Components")

| Component | Variant | หมายเหตุ |
|---|---|---|
| Status Bar | Theme = Dark / Light | ใช้ของระบบปฏิบัติการ ไม่ต้องวาดเอง |
| Bottom Navigation | Active = Home, Explore, Book, Tickets, More | แถบลอยมุมโค้ง แท็บ active มีพื้นวงรีสีเทาอ่อน |
| Button | Type = Primary, Secondary, OnDark | 327×48 |
| Service Tab | Room, Ticket, Auditorium × Selected / Default | 104×64 |
| Language Switch | Theme × Lang = TH / EN | ต้องขยายเป็น 7 ภาษา (Figma มีแค่ TH / EN) |
| Show Card | Reminder = Off / On | ตารางการแสดง + ปุ่มตั้งเตือน (Push) |
| Chip | Selected / Default | ตัวกรอง |
| Hero Slider | 6 slide 390×330 | ใช้ภาพชุดเดียวกับเว็บ |

**หน้าจอในแอป:** 01 Onboarding (Splash / Language, Login) · 02 Home · 03 Activities (List, Detail, Shows) · 04 Accommodations · 05 Auditorium · 06 Tickets (Book Ticket, My Tickets) · 07 Explore (Map) · 08 More ส่วน "99 · Archive" ไม่ใช้
**Home** ([ref/app-home.png](ref/app-home.png)): Hero ภาพเต็มความกว้าง → การ์ด "MY TICKET" + ปุ่มทอง "Show QR" → grid ไอคอน 4×2 → Promo slider → Today's shows

### 3.4 Mobile web (page "Design Mobile")

Landing, Overlay / Mobile Menu, Activities List + Detail, Buy Tickets, Accommodations List + Detail, Auditorium List + Detail (390 กว้าง) ใช้ token ชุดเดียวกับ desktop และทำตาม Motion H (แถบ "ซื้อบัตร · Book Now" ติดล่างจอ) และ Motion L

ค่าเฉพาะของมือถือ: หัวข้อ H1 36/44, H2 28/36, H3 24/32 (หัวข้อ 2.2) · การ์ดมุมโค้ง 18–20 · grid 4 คอลัมน์ margin 16 · ไอคอนบริการ bed, dish, hall, car, ticket

## 4. โครงหน้า Landing (Concept A, [ref/web-landing-concept-a.png](ref/web-landing-concept-a.png))

1. Nav sticky: Utility bar (ชื่อสวน · เวลาเปิด · ภาษา · สกุลเงิน) + Main nav (โลโก้ · เมนู · Sign in / Register · ปุ่มทอง "Buy Tickets")
2. Hero slider + ตัวเลขสถิติ + Scroll hint
3. Promo slider 1200 ซ้อนทับขอบล่างของ hero
4. Products and Services: Service Card 4 ใบบนพื้นครีม ขอบล่างโค้ง
5. Exploding gallery บนพื้น `green/950`
6. News & Events: การ์ดเด่น 1 ใบ + รายการ 4 แถว + "View All News"
7. Gallery coverflow
8. Newsletter CTA: การ์ดเขียว + มาสคอต
9. Footer `green/950`

ตำแหน่งที่ต้องเพิ่ม "จุดชวนสมัครสมาชิก" ตาม ROADMAP 4.5 ยังไม่มีในดีไซน์ ให้ Designer กำหนด

## 5. หน้าที่ระบบต้องมี แต่ยังไม่มีใน Figma

ทีม dev ต้องรอดีไซน์หน้าต่อไปนี้ (ROADMAP WBS 1.2.1–1.2.3 ต้องเสร็จก่อน M2 27/11/26)

| หน้า | อ้างอิง | Web | App |
|---|---|---|---|
| สมัครสมาชิก: OTP, Social login (LINE / WeChat / Google / Apple), **กรอกชื่อ + เลขบัตรประชาชน / พาสปอร์ต**, PDPA, Invite code | F1, S1, S2 | ไม่มี | มีแค่หน้า Login |
| ระบุผู้ใช้บัตรรายใบ + เลือกจากรายชื่อผู้ติดตาม | F3 | ไม่มี | ไม่มี |
| Upsell บัตรห้องอาหาร / การแสดง + ที่นั่งคงเหลือ | F3 | ไม่มี | ไม่มี |
| ชำระเงิน, ผลการชำระ, หน้ารอคิว (Waiting room) | S3, S6 | ไม่มี | ไม่มี |
| บัตรของฉัน + QR รายใบ + **ส่งบัตรให้เพื่อน** | F11 | ไม่มี | มี My Tickets ยังไม่มีการส่งบัตร |
| พ้อยของฉัน ("เปิดแลกเร็ว ๆ นี้") | E9 | ไม่มี | ไม่มี |
| โปรไฟล์, ผู้ติดตาม, ขอลบบัญชี | E5 | ไม่มี | ใน More? ต้องยืนยัน |
| หน้า Agent (เว็บ) + **Agent App** ทั้งชุด | F2, E10 | ไม่มี | ไม่มี |
| ตารางการแสดง, แผนที่ภาพนิ่งแยกโซน, FAQ / เบอร์ฉุกเฉิน | E5 | ไม่มี | มี Shows, Map |
| Admin / CMS, Gate Scanner | E6, F4 | ไม่มี | – |

## 6. Motion (Interaction Spec ใน Figma, frame "Interaction Spec · Scroll & Motion")

Figma แนะนำ GSAP ScrollTrigger หรือ Framer Motion ขอแนะนำให้ใช้ **CSS + IntersectionObserver ก่อน** และใช้ GSAP เฉพาะ Hero parallax เพื่อรักษา LCP

| # | ชื่อ | สเปก |
|---|---|---|
| A | Nav sticky + shrink | scroll > 80px: พื้นโปร่ง → `brand/primary` 100% + Elevation 2, สูง 88 → 72, โลโก้ scale 0.85 · 300ms ease-out |
| B | Scroll progress | เส้น `brand/secondary` สูง 3px ใต้ Nav, scaleX 0 → 1 |
| C | Hero parallax | ภาพเลื่อน 0.4× + scale 1.10 → 1.00, ข้อความ opacity 1 → 0 และเลื่อนขึ้น 60px เมื่อ scroll ผ่าน 50% ของ hero |
| D | Hero text reveal | Chip → Headline → Subtitle → CTA: y 24 → 0, opacity 0 → 1, 700ms `cubic-bezier(.22,1,.36,1)`, stagger 90ms |
| E | Stats count-up | นับจาก 0 ใน 1.2s เมื่อเข้า viewport |
| F | Activities carousel | drag / scroll-x + snap, auto-advance 5s, progress bar, การ์ด active ขอบทอง, ลูกศรเลื่อนทีละ 1 การ์ด |
| G | Section reveal | threshold .2: header y 40 → 0, การ์ด stagger 100ms, 700ms ease-out, เล่นครั้งเดียว |
| H | Booking widget sticky (mobile) | เมื่อ scroll ผ่าน widget ให้แถบ "ซื้อบัตร · Book Now" slide-up 250ms ติดล่างจอ |
| I | Card hover | translateY −8px + Elevation 4, ภาพ zoom 1.05 · 250ms |
| J | Promo slider | autoplay 5s (หยุดเมื่อ hover), เลื่อน 600ms ease-in-out, loop, ภาพ zoom 1.00 → 1.05 |
| K | News slide-in | x +40 → 0, stagger 120ms |
| L | Products | desktop grid 4 คอลัมน์ · mobile scroll-x + snap, peek การ์ดถัดไป 40px |

**การเข้าถึง:** เมื่อผู้ใช้ตั้ง `prefers-reduced-motion: reduce` ให้ปิด C, D, E, F (auto-advance), I (เหลือแค่เงา), J (autoplay) และ K ส่วน slider ที่เล่นอัตโนมัติต้องมีปุ่มหยุด (WCAG 2.2.2)

## 7. แนวทางใช้งานสำหรับ dev

- **Web (Next.js):** แปลง token เป็น CSS custom properties ใน `:root` (ชื่อตามหัวข้อ 2) ห้ามใส่ค่าสีลงในโค้ดตรง ๆ
- **App (Flutter):** สร้าง `ThemeData` + `ThemeExtension` จาก token ชุดเดียวกัน ส่วนแถบล่างใช้ 5 แท็บตาม Bottom Navigation
- **รูปภาพ:** ใช้รูปจาก CMS แปลงเป็น WebP / AVIF (system design หัวข้อ 2) ห้ามใช้รูป Figma AI บน production
- **ข้อความทั้งหมดดึงจาก CMS / ไฟล์ i18n 7 ภาษา** ไม่ใช้ข้อความภาษาอังกฤษที่อยู่ใน Figma
- **ไอคอน:** Figma ใช้ไอคอนเส้นที่ทำเป็น component ชื่อ `Icon/<name>` (ไม่ได้อ้างถึงไลบรารีภายนอก) ขนาด 14 / 16 / 18 / 20 ชื่อที่พบ ได้แก่ clock, globe, down, coin, login, ticket, arrowL, arrowR, chevL, chevR, heart, bag, check, star, infinity, cal, adult, child, baby, minus, plus, info, bed, car, dish, hall, mail, menu, phone, pin ให้ export เป็น SVG จาก Figma แล้วทำเป็น icon set เดียว ใช้ร่วมกันทั้งเว็บและแอป (Flutter ใช้ `flutter_svg`)

## 8. ดีไซน์ที่ขัดกับระบบที่ตัดสินแล้ว (ต้องแก้ใน Figma)

| ใน Figma | ที่ตัดสินแล้ว | อ้างอิง |
|---|---|---|
| "Valid for 5 days from booking" | บัตรรายวันใช้ได้ 1 ครั้งในวันที่เลือก | ROADMAP 4.5 |
| "Open the email … at the counter" | สแกน e-Ticket QR รายใบที่ประตู | F4, #26 |
| Thai price: "Thai ID required at the counter" | ราคาไทยซื้อได้เฉพาะบัญชีที่ยืนยันเลขบัตรประชาชน และระบุผู้ใช้รายใบ | F1, F3 |
| Locale popup 8 ภาษา + 8 สกุลเงิน | เฟส 1: 7 ภาษา (ไม่มี RU) และ THB อย่างเดียว | มติ 11 พ.ย. |
| App Language Switch มีแค่ TH / EN | 7 ภาษา | มติ 11 พ.ย. |
| Chatbot บนหน้า Landing | เฟส 2 | ROADMAP 6 |
| แพ็กเกจ Garden Entry / Garden & Show / Full Day + Buffet | บัตร + Upsell ห้องอาหาร / การแสดง (แพ็กเกจรอยืนยัน) | ROADMAP 4.5 |

## 9. เรื่องที่ต้องให้ Designer ยืนยัน

ดึงจาก Figma ครบแล้ว เหลือแต่เรื่องที่ไฟล์ Figma ยังไม่สม่ำเสมอ

1. **H1 desktop (60 / 72)** ใส่เป็นตัวเลขตรง ๆ ไม่ได้ผูกกับ variable ส่วน **H3 desktop** ไม่ได้ใช้ในหน้าใดของ Concept A ถ้าต้องใช้ เสนอ 32 / 40
2. **Elevation Level 2** ถูกอ้างใน Interaction Spec แต่ยังไม่มี variable (หัวข้อ 2.6 ใช้ค่าที่เสนอไว้)
3. **มุมโค้ง:** ใช้ตัวเลขตรง ๆ 7 ค่า เสนอให้ทำ token 4 ระดับ ได้แก่ 12 / 16 / 20 / 24 + full
4. **spacing ที่ยังไม่เป็น token:** 6, 14, 20, 28, 40
5. **เงาในแอปใช้สีดำ** ส่วนเว็บใช้สีเขียวเข้ม เสนอให้ใช้ชุดเดียวกัน
6. หน้าที่ยังขาดตามหัวข้อ 5
