# 9itchi Resume API
CVs: upload + background parse, editor content, versions/history, templates, photo, PDF/DOCX export, ATS check (24 rules), AI rewrite proxy, weekly CV tips, quota paywall. Port 8082, DB `resumes`.

## Conventions (shared by every 9itchi Go service)
- `handlers/` thin (validate, authorize, call service, JSON) -> `services/` all logic, returns `(T, *models.ApiError)` -> `repositories/` only Postgres, plain errors (`ErrNotFound`, `ErrConflict`). Error body `{"error": "..."}`.
- DB calls use `repositories.QueryTimeout`; multi-table writes `Store.WithTx`; never select `resume_files.data` in list queries.
- Schema: add `migrations/NNNN_name.sql` (goose, embedded, runs at startup). Never edit a merged migration.
- Auth: `Authorization: Bearer` AES-GCM-wrapped HS256 JWT (claims `id`, `email`, `role`, `typ`, `exp`; access only). `/admin/*` = `RequireRole("admin")`. `/internal/*` = `x-api-key` (`INTERNAL_API_KEY`), blocked by nginx.
- Events: exchange `9itchi.events` (topic, routing key = event name), names in `core/events.go`; empty `RABBITMQ_URL` = `LogPublisher`.
- Env only, validated in `utils/config.go`. Common: `PORT`, `ENVIRONMENT`, `DATABASE_URL`, `ITCHI_SECRET_KEY`, `ITCHI_ENCRYPTION_KEY`, `INTERNAL_API_KEY`, `INTERNAL_API_KEY_REQUIRED`. Never log tokens, passwords, PII.
- Tests: `{resource}_test.go`, testify, func-field fakes (no testify/mock), success + error paths. Repo tests: `//go:build integration`, embedded Postgres.
- Check: `gofmt -l .; go build ./... && go vet ./... && go test ./...`; `go test -tags integration -count=1 ./repositories/...`; `golangci-lint run`.

## Routes
- User: `POST /resumes/upload`; `GET|POST /resumes`; `GET|PUT|DELETE /resumes/:id`; `GET|POST /resumes/:id/versions`, `GET /resumes/:id/versions/:versionId`, `POST .../:versionId/restore`; `POST /resumes/:id/rewrite|duplicate|export|ats-check`; `PUT /resumes/:id/editable`; `GET|PUT|DELETE /resumes/:id/photo`; `GET /resumes/:id/files`, `GET|DELETE /resumes/:id/files/:fileId`; `GET /me/downloads`; `GET /templates`, `/templates/:id`.
- Admin: `GET|POST /admin/templates`, `PUT /admin/templates/:id`; `GET /admin/users/:id/cv|downloads`.
- Internal: `GET /internal/resumes/:id/pdf|original` (email-api), `POST /internal/tips/send` (schedule-api), `DELETE /internal/users/:id`.

## Data, peers, events
- Tables: `templates`, `resumes`, `resume_versions`, `resume_files` (bytea `data`; `kind` original | export | photo), `resume_downloads`, `cv_tips`, `ai_rewrites`. Latest `0008_two_column_templates.sql`.
- Peers: ai (`/ai/resume/parse`, `/ai/resume/rewrite`), payments (`/internal/quotas/:userId`).
- Publishes `cv.first_imported`, `cv.tips`. Consumes nothing.
- `core/`: `PDFExporter` (go-pdf/fpdf, embedded `core/fonts`, `stamp_mark.png`), hand-built DOCX exporter, PDF text (`ledongthuc/pdf`), `photoextract.go`, `ats.go` rule engine.
- Env: `RABBITMQ_URL`, `AI_API_URL`, `ACCOUNT_API_URL`, `PAYMENTS_API_URL`; `UNLIMITED_DOWNLOADS=true` (read via `os.Getenv` in `services/quota.go`, test envs only).

## Domain rules
- 402 from payments quotas: new CV beyond `resumes` (versions never limited); export when `cv_downloads` used up (DOCX subscribers only); rewrite beyond `ai_rewrites_per_month` (UTC month); any edit when `cv_edit` = 0.
- Read-only CVs (`services/locked.go`): `cv_edit` = 0 locks all; over the plan's CV limit only the chosen (`PUT /editable`) or latest-modified stays editable. Nothing deleted. payments-api down = nothing locked.
- Exports without a plan carry the 9itchi stamp; filename `prenom_nom`. PDF attached to applications is generated on demand, unstamped, not counted.
- Rewrite cached in `ai_rewrites` (same text = no new count) with ai-api `warnings` (`new_numbers`, `too_long`; never blocking, `[]` when none), also on cache hits. Restore saves the replaced content first (undoable). Duplicate counts against the CV limit. Photo max 2 MB JPEG/PNG.
- Six layouts: `classic`, `modern`, `compact` and the two-column `sidebar`, `split`, `banner` (`core/two_column.go`: a spec per layout in `twoColSpecs`, shared with modern; both columns break pages; DOCX as a borderless table with shaded cells).
- Style (`content.style`, `api.CVStyle`, all optional, unset = template default) is read by PDF, DOCX and the web preview alike. `entry_layout` (`stacked|inline`) and `entry_order` (`title_first|subtitle_first`) lay out experience/education title vs company/school in every template (`core/entry_layout.go`); an unset one keeps that template's own choice, both unset keeps the legacy code path. Inline rows read "first, second" (second in the subtitle style). `education_order` (overrides `entry_order` for education), `heading_rule` (`none|thin|thick`), `section_gap` (mm before headings), `skills_style` (`comma|bullet|bar`), `languages_style` (`rows|inline`) and `group_roles` (consecutive same-company roles under one company line) live in `core/style_extras.go`. New CVs (create, upload, parse) without a style start with `api.DefaultCVStyle()` (Amiri 9 pt, navy `#355C7D` accent, inline company-first entries, dates and location on the right); older CVs keep theirs.
- Upload parse in background (`parse_status`); never overwrites user edits; photo-only CVs read by ai-api vision; embedded photo imported unless one is set.
- Tips: max 1/week per student, idle CVs first; resume only emits `cv.tips`. Other users' resumes = 404.

## Gotchas
- `pkg/account` client is an unused stub.
