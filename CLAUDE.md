# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

```bash
# Build everything (frontend + backend)
make build

# Build only frontend
make build-web

# Build only backend (quick iteration)
make build-api
# Or use Go directly:
go build -o ytb2bili.exe .

# Clean all build artifacts
make clean

# Run tests
make test
# Or:
go test -v ./...

# Development mode with hot reload (requires air)
make dev

# Format code
make fmt

# Lint code
make lint

# Quick build (Go only, no frontend changes)
make quick-build

# Production build (optimized)
make build-prod
```

**Note**: The `web/` directory contains the Next.js frontend. When building, static files are copied to `internal/web/bili-up-web/` and embedded into the Go binary.

## Architecture Overview

### Multi-User System Architecture

This is a **multi-user system** with complete user isolation at three layers:

1. **API Layer** (`/api/v1/videos/*` routes)
   - All video routes protected by JWT authentication middleware
   - Every request must include `Authorization: Bearer <token>` header
   - Frontend uses `authFetch()` from `web/src/lib/authFetch.ts` to automatically attach tokens
   - Key methods with user isolation:
     - `getVideoList()` → `GetVideosPaginatedForUser(offset, limit, userID)`
     - `getVideoDetail()` → `GetVideoByIDForUser(id, userID)` / `GetVideoByVideoIDForUser(videoID, userID)`
     - `deleteVideo()` → `DeleteVideoForUser(id, userID)`
     - `retryTaskStep()` → `GetVideoByIDForUser()` / `GetVideoByVideoIDForUser()`

2. **File System Layer**
   - Files stored per-user: `{FileUpDir}/user_{userID}/{date}/{videoID}/`
   - `StateManager` in `internal/chain_task/manager/state.go` requires `userID` parameter
   - Prevents file conflicts between users downloading the same video

3. **Background Task Layer**
   - Global task queue processes all users' tasks
   - `TbVideo` model includes `UserID` field for task context
   - Task chain handlers preserve `userID` through entire pipeline

**Critical**: Never bypass user isolation. Always use `*ForUser` service methods when dealing with user data.

### Task Processing Pipeline

The system uses a **two-phase architecture**:

#### Phase 1: Preparation (Real-time Processing)
**Handler**: `ChainTaskHandler` in `internal/chain_task/chain_task_handler.go`
- Executes immediately when video URL is submitted
- Runs 4 steps in parallel: Subtitle Generation → Cover Download → Translation → Metadata Generation
- Scheduling: Cron job every 5 seconds, max 10 concurrent workers
- Concurrency control via `workerPool` channel

#### Phase 2: Upload (Scheduled Execution)
**Handler**: `UploadScheduler` in `internal/chain_task/upload_scheduler.go`
- Video upload: 1 per hour (configurable)
- Subtitle upload: 1 hour after successful video upload
- Separate scheduling to avoid Bilibili rate limits

**Key Insight**: Tasks are stateful. Status is persisted in `task_steps` table, allowing recovery from restarts.

### State Management

**StateManager** (`internal/chain_task/manager/state.go`):
- Central state container for task execution
- Stores: file paths, context data, cache
- **Must be initialized with**: `NewStateManager(id, userID, videoID, projectRoot, createdAt)`
- File paths are computed based on user directory: `{projectRoot}/user_{userID}/{date}/{videoID}/`

### Database Models

**Key Tables**:
- `cw_saved_videos`: Main video storage with `user_id` column for multi-tenancy
- `cw_task_steps`: Task execution state, linked via `video_id`
- `cw_users`: User accounts and Bilibili authentication

**Important**: `SavedVideo` (pkg/store/model/models.go) is the primary model used throughout the codebase.

### Dependency Injection

Uses **Uber FX** (`go.uber.org/fx`) for declarative dependency injection:
- `main.go` defines the dependency graph
- Services are provided as constructor functions
- Lifecycle hooks: `OnStart`, `OnStop`

### External Service Integration

**yt-dlp Integration**:
- Binary path: configurable in `config.toml` or auto-downloaded
- Wrapper: `pkg/utils/ytdlp_manager.go`
- Supports: YouTube, TikTok, Twitter, Instagram, and more

**Bilibili API**:
- Auth: TV QR code login flow
- Upload: Official SDK with multi-account support
- Implementation: `internal/bili_account/` and `internal/chain_task/handlers/upload_*.go`

**AI Services**:
- Whisper AI: Subtitle generation (local execution)
- Baidu Translate: `pkg/translator/baidu_translator.go`
- DeepSeek AI: `internal/chain_task/handlers/generate_metadata.go`

### Authentication Flow

1. **Frontend**: User enters credentials → stored in localStorage as `jwt_token`
2. **API Request**: `authFetch()` attaches `Authorization: Bearer <token>` header
3. **Backend**: `authMiddleware.JWTAuth()` validates token, extracts `userID`
4. **Handler**: Calls `auth.GetUserID(c)` to get `userID` for database queries

**JWT Implementation**: `internal/auth/jwt.go`

## Configuration

**Main Config**: `config.toml`
- `listen`: Server port (default: `:8096`)
- `FileUpDir`: Root directory for file storage (per-user subdirectories created automatically)
- `[database]`: Connection details (MySQL/PostgreSQL/SQLite)
- `[TenCosConfig]`: Tencent Cloud COS for cloud storage
- `[GeminiConfig]`: AI service for metadata generation

**Dynamic Config**: Runtime settings via `/api/v1/config` (Baidu Translate, DeepSeek, etc.)

## Important Patterns

### Error Handling in Task Chain

Tasks return `bool` success status. Errors are stored in `context["error"]`:
```go
if err != nil {
    context["error"] = err.Error()
    return false
}
return true
```

### User Permission Checks

Always verify user ownership before operations:
```go
userID, exists := auth.GetUserID(c)
if !exists || userID == 0 {
    c.JSON(401, gin.H{"message": "未登录"})
    return
}
video, err := service.GetVideoByIDForUser(id, userID)
if err != nil {
    c.JSON(404, gin.H{"message": "视频不存在或无权操作"})
    return
}
```

### Adding New Task Handlers

1. Create handler in `internal/chain_task/handlers/`
2. Implement `Task` interface from `internal/core/types/task_interface.go`:
   - `GetName() string`
   - `Execute(ctx map[string]interface{}) bool`
   - `InsertTask() error`
   - `UpdateStatus(status, message string) error`
3. Register in task chain: `chain.AddTask(handler)`
4. Wrap with step tracking: `h.wrapTaskWithStepTracking(handler, videoID)`

## Common Issues

### File Path Calculations
- Always use `StateManager.CurrentDir` as base path
- Never hardcode paths - they include user ID and date components
- File naming: `{videoID}.mp4`, `en.srt`, `zh.srt`, `cover.jpg`

### Multi-User Context
- Background tasks (cron, scheduler) process all users' data
- User-triggered operations (API calls) MUST be scoped to `userID`
- When in doubt: check if method has `userID` parameter or uses `*ForUser` service method

### Task Status Management
- Don't manually set task status - use `TaskStepService`
- Status flow: `pending` → `running` → `completed`/`failed`/`skipped`
- Recovery: `ResetFailedStepsToPending()` on application startup

## Testing

**Manual Testing**:
```bash
# Health check
curl http://localhost:8096/health

# Auth status
curl http://localhost:8096/api/v1/auth/status

# Video list (requires token)
curl -H "Authorization: Bearer <token>" http://localhost:8096/api/v1/videos?page=1&limit=10
```

**Unit Tests**: Located with source files (e.g., `*_test.go`)
```bash
go test -v ./internal/core/services/...
```

## Project-Specific Conventions

- **Video IDs**: YouTube video IDs are stored as-is, used as unique identifiers
- **Status Codes**:
  - `001`: Pending preparation
  - `002`: Processing
  - `200`: Ready for upload
  - `300`: Video uploaded
  - `400`: Completed
  - `999`: Failed
- **File Naming**: Use `videoID` as base filename (not database ID)
- **Logging**: Use `h.App.Logger` (structured logger with zap)
- **Concurrency**: Use channels for worker pools, never unbounded goroutines

## Agent skills

### Issue tracker

Issues and specs live in GitHub Issues for `HSJ-BanFan/ytb2bili`; use the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Use the default triage labels: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, and `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

This is a single-context repository using root `CONTEXT.md` and `docs/adr/`. See `docs/agents/domain.md`.
