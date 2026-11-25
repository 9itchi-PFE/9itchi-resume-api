# 9itchi-resume-api

Resume upload, structured content, versions, templates, PDF/DOCX export, ATS check.

Port `8082`, health `/health`. All endpoints are implemented.

## Endpoints

| Method | Path | Access | Handler |
|---|---|---|---|
| POST | `/resumes/upload` | user | UploadResume |
| POST | `/resumes` | user | CreateResume |
| GET | `/resumes` | user | ListResumes |
| GET | `/resumes/:id` | user | GetResume |
| PUT | `/resumes/:id` | user | UpdateResume |
| DELETE | `/resumes/:id` | user | DeleteResume |
| GET | `/resumes/:id/versions` | user | ListVersions |
| POST | `/resumes/:id/versions` | user | CreateVersion |
| POST | `/resumes/:id/rewrite` | user | Rewrite (`{section: bullet|summary, text, language, job_title}` → `{suggestions, limit, used}`: a better version of one line or of the summary, by the AI) |
| GET | `/resumes/:id/versions/:versionId` | user | GetVersion (with content) |
| POST | `/resumes/:id/versions/:versionId/restore` | user | RestoreVersion |
| POST | `/resumes/:id/duplicate` | user | DuplicateResume |
| PUT | `/resumes/:id/editable` | user | ChooseEditable |
| PUT | `/resumes/:id/photo` | user | UploadPhoto (multipart `file`) |
| GET | `/resumes/:id/photo` | user | GetPhoto |
| DELETE | `/resumes/:id/photo` | user | DeletePhoto |
| POST | `/resumes/:id/export` | user | ExportResume |
| POST | `/resumes/:id/ats-check` | user | ATSCheck (the 24 CV rules) |
| GET | `/internal/resumes/:id/pdf` | internal | InternalResumePDF |
| GET | `/internal/resumes/:id/original` | internal | InternalResumeOriginal |
| POST | `/internal/tips/send` | internal | SendTips: `{"sent": N, "failed": N}` |
| GET | `/me/downloads` | user | Downloads (`{"used", "limit"}`: downloads in the current period of the subscription and what the plan allows; `limit` is null without a limit, 0 without a subscription) |
| GET | `/templates` | user | ListTemplates |
| GET | `/templates/:id` | user | GetTemplate |
| GET | `/admin/templates` | admin | ListAllTemplates (every template, published or not, with `resumes`: how many CVs use it) |
| POST | `/admin/templates` | admin | CreateTemplate |
| PUT | `/admin/templates/:id` | admin | UpdateTemplate (409 when it would withdraw the last published template) |
| GET | `/admin/users/:id/downloads` | admin | UserDownloads (for the team: how many CV downloads a student made and when the last one was, to tell whether a payment was used; nothing of which CV) |
| GET | `/admin/users/:id/cv` | admin | UserCV (for the team: how many CVs a student has and how the one changed last does with the 24 rules; nothing of what it says) |
| GET | `/resumes/:id/files` | user | ListFiles |
| GET | `/resumes/:id/files/:fileId` | user | DownloadFile |
| DELETE | `/resumes/:id/files/:fileId` | user | DeleteFile |

## Run

```
cp .env.example .env
go run main.go
```

## Behaviour notes

- Resumes are scoped by the JWT `id`; another user's resume is `404`. `GET /resumes` takes `page`, `page_size` (default 20, max 100) and returns `{items,total,page,page_size}` without content.
- `content` schema (`api.ResumeContent`): `contact{name,headline,email,phone,location,links[]}`, `summary`, `experience[{company,title,kind,location,start,end,description[]}]`, `education[{school,degree,location,start,end}]`, `projects[{name,tech,year,description[]}]`, `skills[]`, `skill_groups[{label,items[]}]`, `languages[]` (a level may follow a colon: `"Arabe: Langue maternelle"`), `certifications[]`, `extra[{title,items[]}]` (sections added with « + Ajouter une section »). The fields added in sprint 2 are optional. Title max 200 chars, content max 200 KB. `PUT /resumes/:id` only changes provided fields, so switching `template_id` never alters content.
- `POST /resumes/:id/versions` snapshots the content and increments `resumes.version` in one transaction; `GET` lists versions without content.
- `POST /resumes/upload`: multipart field `file`, PDF, DOCX, or a photo of the CV (JPG or PNG), max 10 MB, magic bytes checked. Creates a resume (title from filename, empty content) and stores the original in `resume_files` (bytea) in one transaction. Then the CV is read in the background: text extracted (`core.ExtractText`) and structured by ai-api (`POST /ai/resume/parse`), or for a photo sent as an image to the same route, then stored. `parse_status` on the resume goes `pending` → `done` or `failed` (poll `GET /resumes/:id`); a user edit while pending wins and the late AI result is discarded. `language` (`fr|en|ar`) is set once read.
- `content` also holds `certifications[{name,issuer,year}]`.
- `POST /resumes/:id/versions` takes an optional `{"label"}`, a short ASCII key (max 100) that the client translates in the history tab (`bullet_rewritten`, `template_changed`, `section_added`, `manual`...); versions return it as `label`.
- `POST /resumes/:id/versions/:versionId/restore` first saves the current content as a new version labelled `before_restore`, then puts the old one back, in one transaction, so a restore can itself be undone. `POST /resumes/:id/duplicate` (`{"title"}`) copies content and template into a new CV and counts against the `resumes` quota (`402`).
- The CV photo is one JPEG or PNG per CV (checked by its bytes, max 2 MB, `413` beyond); a new upload replaces the old one. `GET /resumes/:id` says `has_photo`. After an AI read, `parse_report` holds `{pages,seconds,rewritten,removed[]}` for the « Ce qu'on a changé » box; `removed` only lists `age`, `birth_date`, `nationality`, `marital_status`, `full_address`.
- Plan limits come from payments-api (`GET /internal/quotas/:userId`): `resumes` caps how many CVs a user may have (create and upload answer `402` beyond it; versions are never limited) and `cv_downloads` caps downloads, counted in `resume_downloads` so deleting a CV does not reset them (`402` beyond it). Free-plan exports carry the "CV créé avec 9itchi" mark. A missing key means no limit; when payments-api cannot answer, these routes return `503`.
- `POST /resumes/:id/ats-check` (optional body `{"target_language":"fr|en|ar"}`) runs the 24 rules of screen 4.2 and returns `{passed,total,to_fix,rules:[{id,group,status,where,was,now}]}` with `status` `ok|fix|tip`. Rule ids, in screen order: `contact_details, no_personal_details, profile_links, photo, summary, action_verbs, quantified_results, short_bullets, no_first_person, number_style, reverse_chronological, graduation_year, gpa, local_diplomas, no_skill_ratings, relevant_skills, certifications, one_page, fonts, colors, bold_titles, file_name, pdf_size, company_language`. `where` is a JSON path (`experience[0].description[1]`) to highlight, `was` the current text; rewrites come from ai-api `POST /ai/resume/rewrite`. The texts of each rule live in the client (fr, en, ar). The page and size rules render the CV with its template.
- Files: `GET /resumes/:id/files` (metadata only), `GET /resumes/:id/files/:fileId` (attachment download), `DELETE` returns `204`.
- `POST /resumes/:id/export` body `{"format":"pdf"|"docx"}` renders the CV with its template's layout and stores it as a `kind=export` file named `prenom_nom.pdf|docx`. The three layouts follow screen 4.3: `classic` (Source Sans 3, round photo, blue small-caps headline, blue uppercase titles over a rule), `modern` (dark `#1F2A44` side column with photo, contact, skills and languages; Source Serif 4 titles) and `compact` (black and white, centred name, never a photo). The PDF embeds Latin subsets of Source Sans 3 / Source Serif 4 (SIL OFL, `core/fonts`); characters outside them print as `?`. The Word file keeps the same layout with Calibri/Cambria and is for subscribers only (`402` on the free plan). Headings are French, English for English CVs. Unknown formats return `400`. Exporters are registered in `core.DefaultExporters()`.
- `GET /internal/resumes/:id/pdf` (`x-api-key`) renders the CV as it is now (template, language, photo) and returns the PDF; `404` when the resume does not exist. Nothing is stored and it is not counted as a download, so an application always carries the current CV.
- `GET /internal/resumes/:id/original` (`x-api-key`) returns the file the student imported for this CV, byte for byte, with its own name and type (PDF or DOCX); `404` when the CV was not started from a file. An application sent without a subscription carries this file.
- Templates: 3 seeded (classic, modern, compact); admins create/edit/deactivate; `GET /templates` lists active ones.

## CV tips (sprint 6)

`POST /internal/tips/send` raises the weekly advice of « Paramètres → Alertes → Conseils CV ». schedule-api calls it during the day in Tunis.

- A call claims at most 50 students who were never tipped or were last tipped about a week ago (`cv_tips`, one row per student, written before anything is sent, so several instances never tip the same student twice).
- For each, the CV they changed last is checked with the 24 rules. A CV still being read, or changed in the last 3 days (the student is working on it), is left alone.
- When points are left to correct, the event `cv.tips` is published (`user_id`, `resume_id`, `count`, `rule`: the first rule to correct). With nothing to correct nothing is raised, and the student is looked at again a week later.
- resume-api only raises the event: notification-api reads the student's settings and makes it a notification, an email, both or nothing.
- An event that could not be published, or a CV that could not be read, gives its claim back and is counted in `failed`: the next call tries again.

## Events (RabbitMQ)

Optional. With `RABBITMQ_URL` set, `cv.tips` is published on the topic exchange `9itchi.events` (routing key = event name, persistent JSON, publisher confirms). Without it the event name is logged and nothing is published. The URL is never logged.

Broker tests: `RABBITMQ_TEST_URL=amqp://guest:guest@localhost:5672/ go test -tags integration ./core/` (skipped without the variable).

## Tests

```
go test ./...
go test -tags integration -count=1 ./repositories/...   # embedded PostgreSQL, ~1 min
```

## What a plan allows (no free download)

A student with no subscription writes and checks their CV but cannot download it; a plan sets how many downloads a period of the subscription allows and which groups of the diagnosis are shown. The limits come from payments-api (`GET /internal/quotas/:userId`, with `period_start`):

- `cv_downloads`: downloads per period. `0` refuses every download with 402 (`subscription required to download your CV`); a number is counted from `period_start` and refused beyond it with 402 (`download limit of your plan reached for this period`); no key means no limit.
- `ats_<group>` set to `0` hides a group of the diagnosis (`contact`, `summary`, `experience`, `education`, `skills`, `layout`, `file`). `POST /resumes/:id/ats-check` then returns the rules of that group with `status: "locked"` and nothing of what they found, and lists the group in `locked_groups`. `passed`, `total` and `to_fix` still count the whole CV. No key shows the group.
- `resumes`: how many CVs (unchanged).

When payments-api cannot be reached, downloads and the diagnosis answer 503 rather than show what the plan may not allow. The CV sent with an application (`/internal/resumes/:id/pdf`) is not a download.

## Account deletion

`DELETE /internal/users/:id` (`x-api-key`) is called by account-api before it deletes an account. Here it concerns the CVs with their versions, files and photo, the record of the downloads and the weekly advice. It runs in one transaction, answers `204`, and can be called again without harm (account-api retries until every service has answered). Nothing about the user is logged.

## First CV

When a student imports their first CV (`POST /resumes/upload` leaves them with exactly one), the event `cv.first_imported` (`user_id`) is published: statistics-api counts it as the step « first CV » between signing up and subscribing. A student who deletes their CV and imports another is counted again. A failed count or publish never fails the import.

## AI rewrites

`POST /resumes/:id/rewrite` asks ai-api (internal route, internal key) for a better version of one line of the student's own CV, or of its summary. What the AI proposed is kept in `ai_rewrites` (migration `0005`) under a digest of the section, text, language and offer title: the same text comes back without asking the AI nor counting it. The plan's `ai_rewrites_per_month` limits new rewrites per calendar month (absent: no limit; `0`: 402 « part of the subscription »; reached: 402 « monthly limit »). Another student's CV is a 404. The rewrites are erased with the account.

## CVs beyond the plan's limit (read-only)

Nothing is deleted when a subscription ends. When a student has more CVs than the plan's `resumes` limit (at least one stays editable), the first ones in this order stay editable: the CV chosen with `PUT /resumes/:id/editable` (`resumes.pinned_at`, migration `0006`), then the last modified. The others carry `"locked": true` in `GET /resumes` and `GET /resumes/:id`; they can be read, downloaded (within the download limit) and deleted, but a change (`PUT /resumes/:id`, versions, restore, photo) answers `402 {"error":"subscribe to edit your other CVs"}`. A plan that allows them again unlocks them at once. If payments-api cannot be reached, nothing is locked.
