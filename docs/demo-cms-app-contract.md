# Demo CMS + App: shared contract (12 Nov 2026)

Slice of Plan 04 (CMS), Plan 14 Tasks 2–3 (admin), and Plan 15 (mobile), built for the 11:00 demo.
The CMS and App teams build in parallel. Both follow this file.

## Public content API (no JWT)

`GET /api/content/public/items?type=<type>&lang=<th|en|zh|ja|ko|de|fr>` returns published items, sorted by `attrs.order` ascending:

```json
{"items":[{"id":"uuid","type":"hero_slide","attrs":{"order":1,"imageUrl":"/images/act-thai-cultural-show.webp","ctaHref":"/checkout"},
           "texts":{"title":"...","subtitle":"...","ctaLabel":"..."}}]}
```

- `texts` is already resolved to `lang`, one field at a time. A field with no text in that language falls back to `en`, then `th`.
- An unknown `lang` is treated as `en`. An unknown `type` returns `400 {"error":"unknown_type"}`.
- `imageUrl` is either a path relative to the website (`http://localhost:3000`) or an absolute URL.

| type | attrs | texts (translatable) |
|---|---|---|
| `hero_slide` | `order` int, `imageUrl`, `ctaHref` | `title`, `subtitle`, `ctaLabel` |
| `show` | `order` int, `times` string[] `"HH:MM"` (Asia/Bangkok), `imageUrl` | `name`, `venue`, `description` |
| `faq` | `order` int | `question`, `answer` |

`GET /api/content/public/preview/{token}?lang=` has the same response shape. It returns the draft for one item inside the list of published items of that type, and adds `"preview":true`. The token expires after 30 minutes, after which the endpoint returns `404 {"error":"preview_expired"}`.

## Demo accounts

- Tourist with tickets for today: phone `+66811111111` (enter `0811111111`). The OTP shows at http://localhost:8090.
- Staff use the password `Demo1234!`:
  - `editor@demo.nongnooch.local` edits content.
  - `approver@demo.nongnooch.local` approves content.
  - `admin@demo.nongnooch.local` has every role, but four-eyes still applies.
