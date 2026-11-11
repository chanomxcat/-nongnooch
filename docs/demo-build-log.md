# Demo build log (SDD ledger, 11–12 พ.ย. 2026)

คัดลอกจาก ledger ของ subagent-driven development · ทุกบรรทัด `Ruling:` คือการตัดสินใจที่ทำแทนระหว่างคืน


Branch: demo-slice (base 59ad125). Spec: docs/system-design.md + contracts v2 (reachable).
Deadline: demo Thu 2026-11-12 11:00 ICT. User asleep; continuous execution.

## Pre-flight scan

| Pair / Task | Produces → Consumes | Finding |
|---|---|---|
| T1 → all | chassis config/db/events/outbox/consumer/server, make targets | foundation; all later tasks depend |
| T2 → T3, T6 | mockext /sms/send, /turnstile, /pg/* | T3 notify uses SMS; T6 uses PG + webhook signature X-Mock-Signature HMAC KBANK_WEBHOOK_SECRET |
| T3 ↔ T5 | identity MemberLookup interface; T5 swaps impl to gRPC GetMemberByContact | consistent (T3 stub returns "") |
| T3 ↔ T9 | T9 adds /auth/staff/login to identity | consistent |
| T4 → T5,T6,T7,T9 | authn.Principal + gateway headers | consistent with contracts §3 |
| T5 → T6 | gRPC GetCompanion(member_id,"me") for thai price | ruling R2 |
| T6 → T7 | order.paid payload contracts §5 | consistent |
| T7 → T8,T9 | QR string NN1., GET /tickets/me, POST /gate/scans | consistent |
| T6,T7,T5 → T9 | GET /admin/stats/today per service | consistent |
| T10 | POST /internal/seed per service | each task adds its seed |
| Each task self-consistency | tests vs code within brief | no contradictions found |

Ruling R1: demo has no agent service; member validates invite codes against a seeded demo_invite_codes table behind an InviteValidator interface (contracts §4 says gRPC ValidateInvite) — keeps the demo flow without building Plan 09 — cost if wrong: swap the InviteValidator impl later (small).
Ruling R2: GetCompanion with companion_id "me" returns the member's own holder data (contracts doesn't define "me") — needed for buying a Thai-price ticket for yourself — cost if wrong: rename a sentinel.
Ruling R3: notify sends SMS only (no email/Mailpit) and dashboard aggregates per-service /admin/stats/today instead of a report service — time budget — cost if wrong: Plans 10/11 implement later as written.
Ruling R4: tasks run sequentially per SDD; reviewers use sonnet; implementers sonnet except T8 (web visuals) opus; final review opus — deadline — cost if wrong: slower/costlier runs.
Task 1: dispatched (BASE 59ad125, implementer sonnet)
Task 1: implemented 075b893 (DONE_WITH_CONCERNS: PKG default ./pkg/... ./services/... because docs/node_modules; goose WithStore; make test-int -p 1; compose only mariadb/nats/gotest so far)
Ruling R5: run task N review concurrently with task N+1 implementer (reviewer is read-only; fixes for N are applied after N+1 commits) — 33-min task pace vs 11:00 deadline — cost if wrong: a fix to N may need a small rebase-free follow-up commit touching files N+1 also uses.
Task 2: dispatched (BASE 075b893, implementer sonnet); Task 1 review dispatched (sonnet)
Task 1: review spec ✅ quality Approved (0 Critical, 0 Important)
Task 1: minor (deferred): TestTwoRelaysNoDoublePublish can't detect SKIP LOCKED failure (Nats-Msg-Id dedup masks it)
Task 1: minor (deferred): consumer.Config with empty EventTypes subscribes to everything incl dlq.>
Task 1: minor (deferred): services/example drops relay/consumer goroutine errors on shutdown
Task 1: complete (commits 59ad125..075b893, review clean)
Task 2: implemented a87f4bd (DONE; server.New nil DB ready; /pg requires Bearer KBANK_API_KEY)
Task 3: dispatched (BASE a87f4bd, implementer sonnet); Task 2 review dispatched
Task 2: design hook: side-tab accent border on console.html/checkout.html — fix in Task 2 fix round (checkout shown in demo); cream-palette suppressed (brand token)
Task 2: review spec ✅ quality Approved (0 Critical, 0 Important)
Task 2: minor (deferred): kbank payments map/order list uncapped (Store capped at 200)
Task 2: minor (deferred): checkout pay handler pending-check not atomic with status change → double click could send 2 webhooks
Task 2: minor (deferred): promptpay payload format duplicated kbank.go:339/377
Task 2: minor (deferred): side-tab accent border in mockext console/checkout (design hook) — fix in final wave
Task 2: complete (commits 075b893..a87f4bd, review clean)
Task 3: implemented 1b930da (DONE; tokens JSON {access,refresh,expiresIn}; MemberLookup in identity/token.go; tests pollute shared EVENTS stream → demo-reset must down -v (Task 10))
Task 4: dispatched (BASE 1b930da, sonnet); Task 3 review dispatched
Task 3: review spec ✅ quality Approved-with-Important
Task 3: Ruling: identity trusting X-Client-IP is by design — contracts §3 services trust gateway-set headers, gateway strips client copies (Task 4 test) — cost if wrong: per-IP OTP limit bypassable if a service is exposed without the gateway.
Task 3: minor (deferred): API.Members set but unused; EmailSender interface extra (dropped with warning)
Task 3: fix round 1/5 dispatched — OTP cooldown/hourly-cap check-then-act race (otp.go:785-815)
Task 3: fix round 1/5 (1 addressed, 0 open; commits 1b930da..7f9198c)
Task 3: minor (deferred): otp_rate rows grow unbounded
Task 3: complete (commits a87f4bd..7f9198c, review clean)
Task 4: implemented aaad4f8 (DONE; gateway authenticates only, services enforce via authn.Require; stale token dropped on /auth/* public routes; TRUSTED_PROXIES re-resolved 30s)
Task 5: dispatched (BASE aaad4f8, sonnet); Task 4 review dispatched
Ruling R6: Task 8 (web, apps/web only) runs concurrently with backend Tasks 5–7; web builds against contracts v2 + plan endpoints, stages only its own files, and its Playwright e2e against the full stack moves to Task 10 — deadline buffer — cost if wrong: endpoint shape mismatches fixed during Task 10 integration.
Task 8: dispatched (implementer opus, concurrent with Task 5)
Task 4: review spec ✅ (1 ⚠️) quality: 0 Critical, 1 Important (= the ⚠️)
Task 4: Ruling: keep gateway dropping an invalid/expired bearer token on /api/identity/auth/* and /.well-known/* (continue anonymous) instead of 401 — those endpoints authenticate by body (OTP / refresh token), never by access token; 401 there would block re-login with a stale token — cost if wrong: a client isn't told its stale token was ignored on auth routes (harmless).
Task 4: minor (deferred): path.Clean drops trailing slash (routes.go:963); staff roles whitelist hardcoded (jwks.go:751); trusted-proxy IPv6 not normalized (routes.go:1057); JWKS.Ready unused so healthcheck passes before keys load (jwks.go:722); no-op "unknownkid" test entry
Task 4: complete (commits 7f9198c..aaad4f8, 1 ruling)
Task 5: implemented 125fa39 (DONE; 422 id_number_invalid per contracts; refresh body key refreshToken; member lookup failure → mid empty + WARN; seed UUIDv5; make gen via deploy/gen-proto.sh)
Task 6: dispatched (BASE 125fa39, sonnet); Task 5 review dispatched
Task 5: review spec ✅ quality Approved (0 Critical, 2 Important)
Task 5: ⚠️ resolved: gateway blocks /api/<svc>/internal/* (Task 4 tests) — /internal/seed not reachable from outside.
Task 5: Ruling: identity swallowing member-lookup errors at /auth/verify (token with empty mid + WARN) is kept — login stays available when member is down, frontend calls /auth/refresh after registration — cost if wrong: during a member outage a user can't order for up to 15 min until refresh.
Task 5: Ruling: idType derived from nationality (Thai → thai_id, others → passport); request idType is ignored (decoder not strict, web sends it harmlessly) — demo scope — cost if wrong: Thai nationals cannot register with a passport.
Task 5: minor (deferred): grpc.go needless contextT alias; GetContactPrefs ignores consent query error; points() error flow awkward
Task 5: complete (commits aaad4f8..125fa39, 2 rulings)
Task 6: implemented 1d71e3f (DONE; late webhook after hold expiry → paid_unfulfilled + payment.discrepancy; KBank call inside idempotency tx; no mid → 403 member_required)
Task 7: dispatched (BASE 1d71e3f, sonnet); Task 6 review dispatched
Task 6: review spec ✅ quality Approved (0 Critical, 0 Important)
Task 6: minor (deferred): duplicate doc comment service.go:1416; KBank HTTP call inside open DB tx; idempotency key not bound to request hash; GetCompanion gRPC error → 500; webhook status written unvalidated; quota seed doesn't roll forward past 60 days
Task 6: complete (commits 125fa39..1d71e3f, review clean)
Task 7: implemented 9e8430d (DONE; gateway route not exercised with real user token; e2e scan wrong_date for tomorrow order (correct); seed no-op)
Ruling R7: OTP limits (cooldown 60 s, 5/contact/h, 20/IP/h) must be env-configurable and relaxed in demo compose (Task 10 scope) — rehearsals + repeated logins in front of the customer would hit 429 — cost if wrong: none for production defaults.
Ruling R8: Task 9 split — 9a backend (staff login in identity + member /admin/stats/today) now; 9b frontend (apps/admin gate + dashboard) after Task 8 lands its web toolchain — avoid duplicating/conflicting npm workspace + Dockerfile.web — cost if wrong: ~20 min serialisation.
Task 9a: dispatched (BASE 9e8430d, sonnet); Task 7 review dispatched
Task 7: review spec ✅ quality Approved (0 Critical, 0 Important)
Task 7: ⚠️ resolved: QR within 30 s — implementer e2e observed ~4 s (task-7-report); Task 10 e2e re-verifies via web.
Task 7: minor (deferred): invalid scans (bad sig / unknown id) recorded but no ticket.scanned event though contracts §5 lists result "invalid"; unrequested VoidTickets RPC; test nits (replay copy, && assertion in wrong_date test)
Task 7: complete (commits 1d71e3f..9e8430d, review clean)
Task 9a: implemented e187806..6d744b9 (DONE; staff login + seeds, member stats, OTP limits via env relaxed in compose)
Task 9a: review dispatched
Task 9a: review spec ✅ quality pass-with-fixes (0 Critical, 2 Important)
Task 9a: ⚠️ resolved: gateway public route + staff token acceptance verified by 9a e2e (login + 3 stats endpoints 200 via :8000)
Task 9a: minor (deferred): member/http.go imports not gofmt-sorted; seed INSERT IGNORE swallows other errors; rate-limit check-then-insert not atomic; dummyHash error ignored
Task 9a: fix round 1/5 dispatched — staff_login_fails unbounded growth; overlong email → 500
Task 9a: fix round 1/5 (2 addressed, 0 open; commits 6d744b9..42f3aef)
Task 9a: minor (deferred): staff_login_fails purge filters created_at but only (email,created_at) index → scan per failure; purge test ignores Scan errors
Task 9a: complete (commits 9e8430d..42f3aef, review clean)
Task 8: implemented b31db85 (DONE; manual browser e2e passed; real API shapes adapted in apps/web/lib/api/client.ts; e2e spec written not run → Task 10; partial JA/KO/DE/FR)
Task 9b: dispatched (BASE b31db85, sonnet); Task 8 review dispatched
Task 8: review spec ✅ (2 ⚠️) quality ❌ (0 Critical, 4 Important)
Task 8: Ruling: BFF X-Forwarded-For — never forward the client's header verbatim; when TRUST_PROXY_HEADERS=true forward only the right-most XFF entry (appended by the trusted proxy in front of web, e.g. Caddy), otherwise send none (gateway then sees the web container, covered by relaxed demo limits) — cost if wrong: in local demo all users share one IP bucket.
Task 8: Ruling: payment result page never declares failure on timeout; keep polling with a "confirming payment" state and allow retry only when the order is expired/failed; payment retry reuses a per-order Idempotency-Key — avoids double charge — cost if wrong: a user waits longer on a genuinely lost webhook (reconciliation is out of demo scope).
Task 8: minor (deferred): QR colors hard-coded (QrCode.tsx); /r/%E0 decodeURIComponent throws in middleware.ts; hard-coded localhost:8090 demo-tools link
Task 8: fix round 1/5 dispatched — BFF dot-segment traversal; XFF spoofing; payment give-up/double charge; ticket fallback match by visitDate
Task 8: fix round 1/5 (4 addressed, 1 new Important — OrderResult.tsx:156 ticket fetch ignores isPaid 'completed'; commits b31db85..e38a538)
Task 8: minor (deferred): advanceAttempt on retry creation not confirmed failure; QrCode.tsx doc comment misplaced; card-declined stays 'confirming' until hold expiry because commerce has no paymentStatus failed
Task 8: fix round 2/5 dispatched
Task 8: fix round 2/5 (1 addressed, 0 open; commits e38a538..27de9f6)
Task 8: minor (deferred): needsTickets is a one-line alias of isPaid
Task 8: complete (commits 42f3aef..27de9f6, review clean after 2 fix rounds)
Task 9b: implemented f69d44a..82f84b4 (DONE; camera path untested (no camera in headless), PASS/wrong_date panels only unit-tested; admin host 3001→3000)
Task 10: dispatched (BASE 82f84b4, opus); Task 9b review dispatched
Task 9b: review spec ✅ quality ❌ (0 Critical, 3 Important)
Task 9b: minor (deferred): decode-only JWT in admin (server-side display only); csrf trusts x-forwarded-host; unused .dot.stale; hard-coded favicon hex; no BFF route tests
Task 9b: fix round 1/5 dispatched — BFF dot-segment traversal; XFF spoofing; scan debounce recorded before outcome / held-QR flips to USED
Task 9b: fix round 1/5 (3 addressed, 0 open; commits 82f84b4..1aba427)
Task 9b: minor (deferred): sightings during an in-flight scan (>3 s) not recorded → may flip to USED; tap-to-dismiss with ticket still in view rescans; lone '%' segment → 400
Task 9b: complete (commits 27de9f6..1aba427, review clean after 1 fix round)
Task 10: implemented a3058d5..61d0f69 (DONE; reset+seed 143 s; e2e web 1/1 admin 1/1; demo-reset removes only mariadb+nats volumes; web demo flags moved to build time; copy-QR button; camera untested)
Ruling R9: controller re-runs make demo-reset CONFIRM=yes after 00:00 on 2026-11-12 so seeded 'today' data matches the demo date — cost if wrong: dashboard 'today' shows zeros.
Task 10: review dispatched
Task 10: review spec ✅ quality Approved (0 Critical, 0 Important)
Task 10: minor (deferred): volume names hardcoded in Makefile; seed history skip keyed on orders>=12 (no resume after partial failure); demo-script dashboard numbers depend on fixtures; console button labels in fallback table unverified
Task 10: complete (commits 1aba427..61d0f69, review clean)
Final review (opus): ready-with-fixes; 0 Critical, 1 Important (I-1 ports bound to 0.0.0.0 — mockext OTP, NATS, MariaDB reachable from venue Wi-Fi)
Final review triage: no deferred minor must be fixed before demo; recommended M-1 (mockext SMS times UTC), M-3 (consumer startup failure silent), M-7 (TRUSTED_PROXIES nonexistent names → DNS stalls), M-2 (ticket order random); script fixes (declined-card fallback, resend wait 60 s, restart line)
Ruling R10: final fix wave includes I-1 + M-1 + M-2 + M-3 + M-7 + script fixes in ONE dispatch — all cheap and directly affect the projected demo — cost if wrong: small extra diff to review.
Final fix wave: dispatched (BASE 61d0f69, sonnet)
Final fix wave: implemented 43eb947 (I-1, M-1, M-2 via commerce item ordering, M-3 consumer retry + readyz, M-7, script) — demo-reset + both e2e pass; scoped re-review dispatched
Final fix wave: re-review all addressed (0 new Critical/Important; minor: Ready stays true after NATS drop; gateway default TRUSTED_PROXIES still lists caddy,gate — overridden in compose)
Final review: clean after one fix wave (commits 61d0f69..43eb947)
