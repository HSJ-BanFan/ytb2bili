# CLAUDE.md

Guidance for working on `ytb2bili`.

## Build and development

```bash
make build          # frontend + backend
make build-web      # Next.js frontend
make build-api      # Go backend
go test -v ./...
make fmt
make lint
make dev
make clean
```

`web/` is the Next.js frontend. Its production output is copied to
`internal/web/bili-up-web/` and embedded in the Go binary.

## Architecture

This is a standalone video-processing tool. Its HTTP API is public; it has no
application login, JWT, membership, payment, license, Redis, or per-user
quota system. Bilibili QR login remains because it supplies credentials for
Bilibili uploads, not because it authenticates this application.

### Task pipeline

`internal/chain_task/chain_task_handler.go` prepares submitted videos. Subtitle
generation, cover download, translation, and metadata generation run through
the global worker pool. `internal/chain_task/upload_scheduler.go` handles the
scheduled video and subtitle uploads. Task steps are persisted so work can
resume after a restart.

Use the bounded `workerPool`/`downloadWorkerPool`; do not add unbounded
goroutines or membership-based concurrency limits.

### State and files

`StateManager` in `internal/chain_task/manager/state.go` owns task paths,
context, and cache. Use `StateManager.CurrentDir` rather than rebuilding file
paths. Files use the video ID as their base name (`{videoID}.mp4`, `en.srt`,
`zh.srt`, `cover.jpg`).

### Data and services

`SavedVideo` in `pkg/store/model/models.go` is the primary video model.
`TaskStep` stores pipeline state. Bilibili credentials are stored by the
multi-account service and encrypted when possible. AI integrations include
Whisper, Baidu Translate, DeepSeek, Gemini, and OpenAI-compatible APIs.

Configuration is loaded from `config.toml`; runtime settings are exposed under
`/api/v1/config`. Keep the Baidu translation endpoint and third-party AI quota
errors intact.

### HTTP routes

Business routes are public and are registered directly by the handlers:

- `/api/v1/videos`
- `/api/v1/upload`
- `/api/v1/subtitles`
- `/api/v1/config`
- `/api/v1/tool/config`
- `/api/v1/bili-accounts`
- `/api/v1/auth` for Bilibili QR/account operations only

Do not add application authentication middleware or token headers. Keep
`Permissions-Policy` and other transport/security headers.

## Important patterns

Tasks return `bool`; put failure details in `context["error"]`:

```go
if err != nil {
    context["error"] = err.Error()
    return false
}
return true
```

Do not manually mutate task status. Use `TaskStepService` and preserve the
status flow `pending` → `running` → `completed`/`failed`/`skipped`.

For new task handlers:

1. Add a handler under `internal/chain_task/handlers/`.
2. Implement `Task` from `internal/core/types/task_interface.go`.
3. Register it with `chain.AddTask(handler)`.
4. Wrap it with step tracking.

## Testing

```bash
go test -v ./...
npm --prefix web run lint
npm --prefix web run build
```

Manual smoke checks:

```bash
curl http://localhost:8096/health
curl http://localhost:8096/api/v1/videos?page=1\&limit=10
```

## Conventions

- Video IDs are stored as-is and are the file-name base.
- Status codes: `001` pending, `002` processing, `200` ready, `300` uploaded,
  `400` complete, `999` failed.
- Use `h.App.Logger` for structured logging.
- Prefer existing helpers, the standard library, and the smallest working diff.

## Agent skills

Issue tracker and domain notes live in `docs/agents/`:

- `docs/agents/issue-tracker.md`
- `docs/agents/triage-labels.md`
- `docs/agents/domain.md`

Use the repository's default GitHub Issues labels: `needs-triage`,
`needs-info`, `ready-for-agent`, `ready-for-human`, and `wontfix`.
