# docs/

| ไฟล์ | คืออะไร |
|---|---|
| [system-design.md](system-design.md) | เอกสารหลัก: overview, tech stack, flow chart, sequence และคำถามเปิด |
| `diagrams/*.mmd` | **ต้นฉบับ** ของ diagram ทุกภาพ (Mermaid) |
| `diagrams/*.svg` | ภาพที่ render แล้ว (สร้างจาก script ห้ามแก้เอง) |
| `diagrams/drafts/` | ร่างแรกที่ใช้ตกลง style (เก็บไว้อ้างอิง) |
| `superpowers/specs/` | spec ของเอกสารชุดนี้ |
| `source/roadmap-v7-text.txt` | ข้อความที่ดึงจาก Roadmap v7 |

## แก้ diagram

```bash
cd docs
npm install          # ครั้งแรกครั้งเดียว
# แก้ diagrams/<id>.mmd
npm run diagrams     # render SVG ทุกภาพ + อัปเดตบล็อก Mermaid ใน system-design.md
```

ถ้าจะเพิ่ม diagram ใหม่ ให้สร้าง `diagrams/<id>.mmd` แล้ววาง marker นี้ไว้ใน `system-design.md`

```
<!-- diagram:<id> -->
<!-- /diagram -->
```

ไอคอนใช้ Lucide ผ่าน icon shape ของ Mermaid เช่น `db@{ icon: "lucide:database", form: "square", label: "MariaDB", pos: "b" }` (ดูชื่อไอคอนได้ที่ https://lucide.dev/icons) ถ้าเปิด `.md` บน GitHub หรือใน VS Code preview ไอคอนในบล็อกโค้ดจะเป็น "?" ให้ดูภาพจาก SVG แทน

## เปิดใน draw.io

draw.io → **Arrange → Insert → Advanced → Mermaid** → วางเนื้อหาไฟล์ `.mmd` (ไอคอน Lucide จะไม่มา ส่วนอื่นแก้ต่อได้)
