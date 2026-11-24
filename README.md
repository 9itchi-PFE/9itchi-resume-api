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
| GET | `/resumes/:id/versions/:versionId` | user | GetVersion (with content) |
| POST | `/resumes/:id/versions/:versionId/restore` | user | RestoreVersion |
| POST | `/resumes/:id/duplicate` | user | DuplicateResume |
| PUT | `/resumes/:id/photo` | user | UploadPhoto (multipart `file`) |
| GET | `/resumes/:id/photo` | user | GetPhoto |
| DELETE | `/resumes/:id/photo` | user | DeletePhoto |
| POST | `/resumes/:id/export` | user | ExportResume |
| POST | `/resumes/:id/ats-check` | user | ATSCheck (the 24 CV rules) |
| GET | `/internal/resumes/:id/pdf` | internal | InternalResumePDF |
| GET | `/templates` | user | ListTemplates |
| GET | `/templates/:id` | user | GetTemplate |
| POST | `/admin/templates` | admin | CreateTemplate |
| PUT | `/admin/templates/:id` | admin | UpdateTemplate |
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
- `POST /resumes/upload`: multipart field `file`, PDF or DOCX only, max 10 MB, magic bytes checked. Creates a resume (title from filename, empty content) and stores the original in `resume_files` (bytea) in one transaction. Then the CV is read in the background: text extracted (`core.ExtractText`), structured by ai-api (`POST /ai/resume/parse`), stored. `parse_status` on the resume goes `pending` → `done` or `failed` (poll `GET /resumes/:id`); a user edit while pending wins and the late AI result is discarded. `language` (`fr|en|ar`) is set once read.
- `content` also holds `certifications[{name,issuer,year}]`.
- `POST /resumes/:id/versions/:versionId/restore` first saves the current content as a new version, then puts the old one back, in one transaction, so a restore can itself be undone. `POST /resumes/:id/duplicate` (`{"title"}`) copies content and template into a new CV and counts against the `resumes` quota (`402`).
- The CV photo is one JPEG or PNG per CV (checked by its bytes, max 2 MB, `413` beyond); a new upload replaces the old one. `GET /resumes/:id` says `has_photo`. After an AI read, `parse_report` holds `{pages,seconds,rewritten,removed[]}` for the « Ce qu'on a changé » box; `removed` only lists `age`, `birth_date`, `nationality`, `marital_status`, `address`.
- Plan limits come from payments-api (`GET /internal/quotas/:userId`): `resumes` caps how many CVs a user may have (create and upload answer `402` beyond it; versions are never limited) and `cv_downloads` caps downloads, counted in `resume_downloads` so deleting a CV does not reset them (`402` beyond it). Free-plan exports carry the "CV créé avec 9itchi" mark. A missing key means no limit; when payments-api cannot answer, these routes return `503`.
- `POST /resumes/:id/ats-check` (optional body `{"target_language":"fr|en|ar"}`) runs the 24 rules of screen 4.2 and returns `{passed,total,to_fix,rules:[{id,group,status,where,was,now}]}` with `status` `ok|fix|tip`. Rule ids, in screen order: `contact_details, no_personal_details, profile_links, photo, summary, action_verbs, quantified_results, short_bullets, no_first_person, number_style, reverse_chronological, graduation_year, gpa, local_diplomas, no_skill_ratings, relevant_skills, certifications, one_page, fonts, colors, bold_titles, file_name, pdf_size, company_language`. `where` is a JSON path (`experience[0].description[1]`) to highlight, `was` the current text; rewrites come from ai-api `POST /ai/resume/rewrite`. The texts of each rule live in the client (fr, en, ar). The page and size rules render the CV with its template.
- Files: `GET /resumes/:id/files` (metadata only), `GET /resumes/:id/files/:fileId` (attachment download), `DELETE` returns `204`.
- `POST /resumes/:id/export` body `{"format":"pdf"|"docx"}` renders the CV with its template's layout and stores it as a `kind=export` file named `prenom_nom.pdf|docx`. The three layouts follow screen 4.3: `classic` (Source Sans 3, round photo, blue small-caps headline, blue uppercase titles over a rule), `modern` (dark `#1F2A44` side column with photo, contact, skills and languages; Source Serif 4 titles) and `compact` (black and white, centred name, never a photo). The PDF embeds Latin subsets of Source Sans 3 / Source Serif 4 (SIL OFL, `core/fonts`); characters outside them print as `?`. The Word file keeps the same layout with Calibri/Cambria and is for subscribers only (`402` on the free plan). Headings are French, English for English CVs. Unknown formats return `400`. Exporters are registered in `core.DefaultExporters()`.
- `GET /internal/resumes/:id/pdf` (`x-api-key`) returns the latest exported PDF, `404` if none.
- Templates: 3 seeded (classic, modern, compact); admins create/edit/deactivate; `GET /templates` lists active ones.

## Tests

```
go test ./...
go test -tags integration -count=1 ./repositories/...   # embedded PostgreSQL, ~1 min
```
