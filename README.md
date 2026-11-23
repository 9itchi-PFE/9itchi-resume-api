# 9itchi-resume-api

Resume upload, structured content, versions, templates, PDF/DOCX export, ATS check.

Port `8082`, health `/health`. Skeleton only: every endpoint returns `501 not implemented` until built.

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
| POST | `/resumes/:id/ats-check` | user | ATSCheck |
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
