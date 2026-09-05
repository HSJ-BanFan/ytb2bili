# Research: biliup Capabilities and CLI/Daemon Interface

## Summary

Biliup has evolved from an earlier Python script collection into a high-performance hybrid system centered on a Rust core (`biliup` / `biliup-cli`), a native streaming downloader (`stream-gears`), an Axum-powered daemon with SQLite persistence, and a Next.js WebUI. It provides enterprise-grade Bilibili upload capabilities—including CLI and glob-based multi-P uploads, archive appending (`biliup append`), scheduled releases (`dtime`), intelligent dynamic CDN probe line selection (`AUTO`, `bda2`, `tx`, `ws`, `qn`, `cos`), chunk- and file-level breakpoint resumption with account rate-limit backoff (PR #1558), and multi-platform live stream recording with synchronized XML danmaku generation and post-processing pipelines. For `ytb2bili` (Issue #2), biliup represents an ideal external reference architecture or pluggable headless upload/recording engine that maps cleanly onto the domain concepts of "投稿引擎" (uploader engine) and "录制会话" (recording session).

---

## Findings

### 1. Architectural Evolution and System Layout

- **Rust core with hybrid bindings**: Originally developed in Python, the upload and recording engine was re-architected into Rust (`biliup-rs`), which was subsequently merged back into the primary monorepo `biliup/biliup` ([Source](https://github.com/biliup/biliup)). The modern architecture consists of:
  - `crates/biliup`: Core library containing Bilibili API clients, credential handlers, UPOS/COS line protocols, and stream extractors ([Source](https://docs.rs/biliup/latest/biliup/)).
  - `crates/biliup-cli`: Command-line interface and Axum REST API daemon service ([Source](https://github.com/biliup/biliup/tree/master/crates/biliup-cli)).
  - `crates/stream-gears`: High-performance FLV/HLS streaming downloader exposed as a native Rust crate and compiled via PyO3 as a CPython extension for backward-compatible Python invocations ([Source](https://github.com/biliup/biliup/tree/master/crates/stream-gears)).
  - `crates/danmaku`: Multi-platform live chat and bullet comment parser emitting synchronized XML files ([Source](https://github.com/biliup/biliup/commit/431d1f61ab66dc78663c5ebdb5f8c52556bac919)).
  - `web/`: Modern web management interface built with Next.js, React, TypeScript, and ByteDance Semi UI, served directly by the Axum binary ([Source](https://github.com/biliup/biliup/blob/master/README.md)).

### 2. Multi-P Upload Capabilities

- **CLI multi-argument submission**: Direct multi-part submissions can be passed as positional arguments: `biliup upload p1.mp4 p2.mp4 p3.mp4`. The CLI uploads each file sequentially or concurrently and combines them into a single archive payload (`Studio` with `videos: Vec<Video>`) before committing to the Bilibili submission API ([Source](https://biliup.github.io/biliup-rs/Guide.html)).
- **Glob pattern batching via configuration**: In `config.yaml` / `config.toml`, users can specify Unix shell-style glob patterns (e.g., `/media/**/*.mp4`). All video segments matching a single pattern rule are automatically grouped into one multi-P archive, whereas distinct streamer/pattern entries yield separate video archives. Per-part metadata (`title`, `desc`, `filename`) can be explicitly set or generated dynamically from file attributes ([Source](https://github.com/biliup/biliup/blob/master/public/config.yaml), [Source](https://biliup.github.io/biliup-rs/Guide.html)).
- **Append mode (`biliup append`)**: Biliup supports modifying existing archives by adding new video parts without re-uploading previous content via `biliup append --vid <BV/AID> [OPTIONS] [VIDEO_PATH]...`. Under the hood, biliup calls the Bilibili edit pre-check API (`archive_pre_endpoint`), extracts the existing `videos` array, appends the newly uploaded chunk descriptors, and submits the revised manifest to `/x/vu/web/edit` or the app submission endpoint ([Source](https://docs.rs/biliup/latest/biliup/video/struct.Studio.html), [Source](https://github.com/biliup/biliup-rs/issues/208)).

### 3. Upload Line Selection and Dynamic CDN Probing

- **Dual upload infrastructures (UPOS vs Cloud Object Storage)**:
  - **UPOS (`ugcupos/bup`)**: Direct Bilibili upload system. Supports domestic and international CDN routes:
    - Mainland China: `bda2` (Baidu Cloud edge), `tx` (Tencent Cloud edge), `bldsa` (Bilibili self-built DSA edge nodes) ([Source](https://biliup.github.io/upload-systems-analysis.html)).
    - Overseas: `txa` (Tencent Cloud overseas/Hong Kong), `alia` (Alibaba Cloud overseas), `bda` (Baidu overseas) ([Source](https://github.com/biliup/biliup-rs)).
    - Global: `ws` (Wangsu CDN), `qn` (Qiniu Cloud) ([Source](https://docs.rs/biliup/latest/biliup/line/index.html)).
  - **Cloud Object Storage (Direct/COS/Kodo)**: Supports Tencent Cloud `cos` and `cos-internal` (which allows intranet VPC upload without egress traffic charges when hosted on Tencent Cloud CVM) and Qiniu `kodo` ([Source](https://docs.rs/biliup/latest/biliup/uploader/cos/struct.Cos.html), [Source](https://biliup.github.io/upload-systems-analysis.html)).
- **Dynamic CDN Probing (`lines = "AUTO"`)**:
  - When line selection is configured as `AUTO` (the default), `biliup` issues a pre-upload handshake request to `https://member.bilibili.com/preupload?r=upos&profile=ugcupos/bup`. The response provides an array of candidate CDN probe URLs (e.g., `//upos-cs-upcdnbda2.bilivideo.com/OK?probe_version=20250923&upcdn=bda2&zone=cs`) ([Source](https://github.com/biliup/biliup/issues/1501)).
  - Biliup spawns concurrent probe requests against each CDN endpoint, measuring HTTP round-trip latency (`cost` in milliseconds), and dynamically binds subsequent chunk uploads to the endpoint with the lowest latency ([Source](https://docs.rs/biliup/latest/src/biliup/line.rs.html)).
- **Concurrency control**: Concurrency can be tuned via `--limit <LIMIT>` (CLI) or `threads = 3` (config), controlling the parallel chunk upload worker pool (typically 3–8 threads; exceeding 8 often triggers CDN TCP connection resets) ([Source](https://github.com/biliup/biliup/blob/master/public/config.toml)).

### 4. Breakpoint Resumption, Rate-Limit Backoff, and Fault Tolerance

- **File-level resumption and upload locking (PR #1558 / commit `175be76`)**:
  - Implemented in `crates/biliup-cli/src/uploader.rs` and `crates/biliup-cli/src/upload_lock.rs` ([Source](https://github.com/biliup/biliup/pull/1558)).
  - During multi-part uploads, biliup writes intermediate state to a persistent task lockfile. If the process is killed or network connectivity fails midway through a multi-P queue, subsequent runs inspect the state file, skip already uploaded parts, and resume with the outstanding video files ([Source](https://github.com/biliup/biliup/pull/1558)).
- **Intelligent rate-limit retry (HTTP 601 backoff)**:
  - Bilibili's upload gateway returns error code 601 (`"您上传视频过快，请您稍作休息后再继续"`) when upload frequency exceeds internal thresholds ([Source](https://github.com/biliup/biliup/issues/1565)).
  - PR #1558 added smart backoff logic in `line.rs` and `uploader.rs`: on detecting code 601, the process pauses with incremental backoff (Retry 1: 1 min, Retry 2: 2 min, Retry 3: 3 min, Retry 4: 4 min; up to 5 attempts) instead of immediately failing the entire job ([Source](https://github.com/biliup/biliup/pull/1558)).
  - File locking in `upload_lock.rs` ensures that multiple worker processes sharing identical Bilibili credentials serialize their upload handshakes, preventing simultaneous submissions from triggering 601 cascades ([Source](https://github.com/biliup/biliup/issues/1593)).
- **Chunk-level UPOS retry**: UPOS uploads slice each video into byte blocks (typically 4MB–10MB). Each block is committed with an offset and index. Transmit errors at the block level are retried individually without discarding previously transferred blocks ([Source](https://docs.rs/biliup/latest/src/biliup/uploader/kodo.rs.html), [Source](https://docs.rs/biliup/latest/src/biliup/uploader/upos.rs.html)).
- **Title auto-truncation**: To prevent upload rejections caused by Bilibili's 80-character maximum title constraint, biliup automatically truncates oversized filenames via `truncate_title` in `crates/biliup/src/uploader/bilibili.rs` ([Source](https://github.com/biliup/biliup/pull/1558)).

### 5. Scheduled Publishing (`dtime`)

- **API implementation**: Studio metadata accepts an optional `dtime: Option<u32>` field ([Source](https://docs.rs/biliup/latest/biliup/video/struct.Studio.html)).
- **Timestamp requirements**:
  - The parameter is a 10-digit Unix epoch timestamp (seconds) ([Source](https://docs.rs/crate/biliup/latest)).
  - Bilibili enforces boundary conditions: the timestamp must be scheduled at least 2 hours (or 4 hours in certain video categories) into the future, and cannot exceed 15 days from submission time ([Source](https://biliup.github.io/biliup-rs/Guide.html), [Source](https://github.com/SmallPeaches/BiliCheater)).
  - Configurable via `--dtime <TIMESTAMP>` in CLI or `dtime: <TIMESTAMP>` in configuration templates ([Source](https://github.com/biliup/biliup/blob/master/public/config.toml)).

### 6. Live Stream Recording Engine and Pipeline

- **Multi-engine downloaders**:
  - `stream-gears`: Native Rust engine specializing in HTTP-FLV and HLS/m3u8 streaming, providing minimal memory footprint and zero external dependencies ([Source](https://github.com/biliup/biliup/blob/master/public/config.toml)).
  - `ffmpeg`: Standard fallback downloader for raw stream capture ([Source](https://github.com/biliup/biliup/blob/master/public/config.toml)).
  - `streamlink`: Multi-threaded segment fetcher wrapped with ffmpeg, ideal for high-bitrate HLS streams (`hls_fmp4`, `hls_ts`) ([Source](https://github.com/biliup/biliup/blob/master/public/config.toml)).
- **Supported live platforms**: Douyu, Huya, Bilibili Live, Douyin, Kuaishou, Twitch, YouTube, AfreecaTV, TikTok, PandaTV, and custom RTSP/RTMP sources ([Source](https://github.com/biliup/biliup)).
- **Recording segmentation rules**:
  - Time-based slicing: `segment_time: '01:00:00'` slices recordings into exact time blocks ([Source](https://github.com/biliup/biliup/blob/master/public/config.toml)).
  - Size-based slicing: `segment_size` defines byte thresholds ([Source](https://github.com/biliup/biliup/blob/master/public/config.toml)).
  - Noise filter: `filtering_threshold: 20` automatically discards truncated recordings smaller than 20MB caused by live jitter or brief disconnects ([Source](https://github.com/biliup/biliup/blob/master/public/config.toml)).
- **Live danmaku capture**: The standalone `crates/danmaku` library hooks into live WebSocket/TCP broadcast rooms, captures user chats, gifts, and SC events, and generates `.xml` danmaku files aligned with recording timestamps for replay integration ([Source](https://github.com/biliup/biliup/commit/431d1f61ab66dc78663c5ebdb5f8c52556bac919)).
- **Processing hooks and automated pipeline**:
  - `segment_processor`: Executes immediately after each segment closes. PR #1626 introduced built-in `remux:mp4` to automatically fix PTS timestamp discontinuity in MPEG-TS streams before upload ([Source](https://github.com/biliup/biliup/pull/1626)).
  - `downloaded_processor`: Shell hook executed after a recording session concludes ([Source](https://github.com/biliup/biliup/blob/master/public/config.toml)).
  - `postprocessor`: Post-upload actions such as moving files (`mv: backup/`), deletion (`rm`), or custom notifications (`run: <cmd>`) ([Source](https://github.com/biliup/biliup/blob/master/public/config.toml)).
  - `UActor` (Upload Actor): Asynchronous actor that receives segment completion events via bounded async channels and triggers the upload pipeline automatically ([Source](https://github.com/biliup/biliup/commit/2a8c1d0bd68739fa72ea63305da2d4c8c09d6d37), [Source](https://github.com/biliup/biliup/commit/e70679dbd7cabb3592d313e96b778b7d2f0e173d)).

### 7. CLI Subcommands and Daemon Interface

- **CLI Subcommand Surface**:
  - `biliup login`: Authenticates via QR code, SMS, web cookies, or password; writes credentials to `cookies.json` ([Source](https://github.com/biliup/biliup/blob/master/README.md)).
  - `biliup renew`: Validates and refreshes existing session cookies ([Source](https://github.com/biliup/biliup/blob/master/README.md)).
  - `biliup upload [OPTIONS] [VIDEO_PATH]...`: Fast upload with optional flags (`--submit <web|client|app|b-cut-android>`, `-l <line>`, `--limit <concurrency>`, `--copyright <1|2>`, `--tid <id>`, `--cover`, `--title`, `--desc`, `--dynamic`, `--tag`, `--dtime`, `--dolby`, `--hires`, `--extra-fields`) ([Source](https://biliup.github.io/biliup-rs/Guide.html), [Source](https://github.com/biliup/biliup/pull/1413)).
  - `biliup append --vid <BV/AID> [OPTIONS] [VIDEO_PATH]...`: Appends video parts to an existing submission ([Source](https://github.com/biliup/biliup-rs/issues/208)).
  - `biliup show <BV/AID>`: Inspects video archive details and review status ([Source](https://github.com/biliup/biliup/blob/master/README.md)).
  - `biliup comments <BVID>` & `biliup reply`: Lists and replies to archive comments ([Source](https://github.com/biliup/biliup/commit/697a525b55a63bac7fdb7010a7d2c60bd5ad2e8b)).
  - `biliup dump-flv <FILE>`: Extracts and displays FLV video/audio header metadata ([Source](https://github.com/biliup/biliup/blob/master/README.md)).
  - `biliup download <URL>`: Direct stream download utility ([Source](https://github.com/biliup/biliup/blob/master/README.md)).
  - `biliup list`: Lists recently uploaded archives ([Source](https://github.com/biliup/biliup-rs/pull/212)).
  - `biliup server [OPTIONS]`: Starts the headless daemon and WebUI on port 19159 (`-b <bind>`, `-p <port>`, `--auth`, `-c <config.toml>`) ([Source](https://github.com/biliup/biliup/blob/master/README.md)).
- **Daemon REST API Surface (Axum router under `/v1`)**:
  - `/v1/streamers` (GET, POST, PUT, DELETE): Manage monitored streamer list and recording configurations ([Source](https://github.com/biliup/biliup/commit/fc159cd598d4f7f444d8adb6cd53127823e703d2)).
  - `/v1/upload/streamers` (GET, POST, PUT, DELETE): Manage upload templates and metadata mapping ([Source](https://github.com/biliup/biliup/commit/2f309ba8a0539e41e02b34bb9a422eaccfefd47b)).
  - `/v1/upload/records` (GET): Retrieve history and status of uploaded videos ([Source](https://github.com/biliup/biliup/commit/e41acef883868a55da7c1d7b2cc5bff4d59ea79d)).
  - `/v1/configuration` (GET, PUT): Read and update global runtime settings ([Source](https://github.com/biliup/biliup/commit/2f309ba8a0539e41e02b34bb9a422eaccfefd47b)).
  - `/v1/users` (GET, POST, DELETE): Manage Bilibili user credentials and login sessions ([Source](https://github.com/biliup/biliup/commit/2a8c1d0bd68739fa72ea63305da2d4c8c09d6d37)).
  - `/v1/bilibili/...`: Endpoints for pre-upload validation, tag searches, and archive inspection ([Source](https://github.com/biliup/biliup/commit/fc159cd598d4f7f444d8adb6cd53127823e703d2)).
  - `/v1/ws`: WebSocket endpoint streaming real-time log output and worker progress ([Source](https://github.com/biliup/biliup/commit/e41acef883868a55da7c1d7b2cc5bff4d59ea79d)).

### 8. Architectural Assessment for `ytb2bili` (Issue #2)

- **Domain alignment with `CONTEXT.md`**:
  - In `ytb2bili`, the system is structured around **搬运编排系统** (orchestration), **策略矩阵** (policy matrix), and **媒体生命周期** (recording session, archive segment, multi-P archive, uploader account, uploader engine) ([Source](D:/02_Dev/Projects/youyisi/ytb2bili/CONTEXT.md)).
  - Biliup fulfills the role of a specialized **投稿引擎** (uploader engine) and **录制会话** (recording session provider). It does not provide higher-level AI translation, Whisper transcription, or cross-account forwarding orchestration, which `ytb2bili` specializes in.
- **Integration alternatives**:
  1. **CLI subprocess delegation**: `ytb2bili` (Go backend) can invoke `biliup upload` / `biliup append` as external worker steps in `internal/chain_task/handlers/`. This completely bypasses having to re-implement Bilibili's fluctuating UPOS line algorithms, probe latencies, or code 601 backoffs in Go.
  2. **Daemon REST integration**: `ytb2bili` can drive `biliup server` in headless mode over HTTP/WebSocket via its `/v1/upload/streamers` and `/v1/upload/records` endpoints.
  3. **In-process protocol adoption**: Alternatively, `ytb2bili` can adapt biliup's proven UPOS probing protocol (`ugcupos/bup` + HEAD latency probing), title truncation (80 chars), and exponential backoff retry for code 601 directly into its Go upload client (`internal/chain_task/upload_scheduler.go`).

---

## Sources

### Kept Sources

- `biliup/biliup` GitHub Repository ([https://github.com/biliup/biliup](https://github.com/biliup/biliup)) — Primary repository source code, release tags, architecture definitions, and build manifests.
- `biliup/biliup-rs` GitHub Repository ([https://github.com/biliup/biliup-rs](https://github.com/biliup/biliup-rs)) — Authoritative source for Rust upload CLI flags, append implementation, and line configurations.
- Biliup Official Documentation ([https://biliup.github.io/biliup/](https://biliup.github.io/biliup/) and [https://biliup.github.io/biliup-rs/Guide.html](https://biliup.github.io/biliup-rs/Guide.html)) — User manuals, configuration examples, and CLI option documentation.
- Biliup Upload Systems Analysis ([https://biliup.github.io/upload-systems-analysis.html](https://biliup.github.io/upload-systems-analysis.html)) — In-depth breakdown of Bilibili UPOS, bup, bupfetch, CDN routes (`bda2`, `tx`, `ws`, `qn`, `cos`, `kodo`), and probe mechanisms.
- Docs.rs biliup Crate API Reference ([https://docs.rs/biliup/latest/biliup/](https://docs.rs/biliup/latest/biliup/)) — Type documentation for `Studio`, `Video`, `Line`, `Parcel`, `Upos`, `Cos`, and `Kodo`.
- Biliup PR #1558: 断点续传、标题自动截断及限流重试优化 ([https://github.com/biliup/biliup/pull/1558](https://github.com/biliup/biliup/pull/1558)) — Core implementation details of breakpoint resumption, upload lockfile mechanism, and code 601 backoff.
- Biliup PR #1626: 启用 segment_processor 阶段，新增 remux:mp4 内建钩子 ([https://github.com/biliup/biliup/pull/1626](https://github.com/biliup/biliup/pull/1626)) — Per-segment stream post-processing and MPEG-TS PTS fix.
- Biliup Commits `fc159cd`, `267bda7`, `2a8c1d0`, `e41acef` — Axum REST routing (`/v1/*`), actor upload pipeline, WebSocket endpoints, and data models.
- Default Configuration Reference (`public/config.toml` at `biliup/biliup`) ([https://github.com/biliup/biliup/blob/master/public/config.toml](https://github.com/biliup/biliup/blob/master/public/config.toml)) — Full schema for stream recording, downloaders, upload lines, and postprocessors.

### Dropped Sources

- Third-party Docker wrappers and generic blog guides (e.g. B站视频转移 - Lim's Blog) — Excluded as secondary and non-authoritative.
- Stale issues regarding obsolete bupfetch cloud providers (e.g., deprecated GCS / BOS lines from 2021) — Excluded to reflect modern Bilibili upload behavior.

---

## Gaps

1. **App and B-Cut Submit Private Signatures**: Biliup supports multiple submission endpoints (`--submit web`, `client`, `app`, `b-cut-android`). The exact signing algorithms (WBI, appsec, and device ticket headers) are maintained inside `crates/biliup/src/uploader/credential.rs` and change periodically as Bilibili updates its mobile apps.
2. **Dynamic Webhook Payload Contracts**: While PR #1355 introduced upload notification webhooks, comprehensive documentation on the exact JSON schema emitted for all failure and success states remains limited to the Rust source code.

Suggested next steps:

- If `ytb2bili` decides to integrate biliup as a CLI engine, write a lightweight Go wrapper (`internal/uploader/biliup_cli.go`) wrapping `biliup upload` and `biliup append` with structured JSON parsing.
- If `ytb2bili` retains its native Go Bilibili upload client, backport biliup's UPOS latency probing algorithm and the PR #1558 code 601 exponential backoff strategy into `internal/chain_task/upload_scheduler.go`.
