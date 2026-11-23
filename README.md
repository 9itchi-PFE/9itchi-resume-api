# 9itchi-resume-api

Resume upload, structured content, versions, templates, PDF/DOCX export, ATS check.

Port `8082`, health `/health`. All endpoints are implemented except `POST /resumes/:id/ats-check`, which returns `501 not implemented`.

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
| POST | `/resumes/:id/export` | user | ExportResume |
| POST | `/resumes/:id/ats-check` | user | ATSCheck (501 not implemented) |
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
- `content` schema (`api.ResumeContent`): `contact{name,email,phone,location,links[]}`, `summary`, `experience[{company,title,start,end,description[]}]`, `education[{school,degree,start,end}]`, `skills[]`, `languages[]`. Title max 200 chars, content max 200 KB. `PUT /resumes/:id` only changes provided fields, so switching `template_id` never alters content.
- `POST /resumes/:id/versions` snapshots the content and increments `resumes.version` in one transaction; `GET` lists versions without content.
- `POST /resumes/upload`: multipart field `file`, PDF or DOCX only, max 10 MB, magic bytes checked. Creates a resume (title from filename, empty content) and stores the original in `resume_files` (bytea) in one transaction. AI parsing is not wired yet (seam in `services/resume.go`).
- Files: `GET /resumes/:id/files` (metadata only), `GET /resumes/:id/files/:fileId` (attachment download), `DELETE` returns `204`.
- `POST /resumes/:id/export` body `{"format":"pdf"}` renders a single-column ATS-friendly PDF (fpdf, core fonts; non-latin characters become `?`) and stores it as a `kind=export` file. `docx` and unknown formats return `400`. Exporters are registered in `core.DefaultExporters()`.
- `GET /internal/resumes/:id/pdf` (`x-api-key`) returns the latest exported PDF, `404` if none.
- Templates: 3 seeded (classic, modern, compact); admins create/edit/deactivate; `GET /templates` lists active ones.

## Tests

```
go test ./...
go test -tags integration -count=1 ./repositories/...   # embedded PostgreSQL, ~1 min
```
