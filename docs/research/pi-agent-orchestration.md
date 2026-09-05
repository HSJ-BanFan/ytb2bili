# Research: Pi Coding Agent Harness Subprocess Invocation, Headless Execution, and Go Orchestrator Integration

## Summary
The Pi coding agent (`@earendil-works/pi-coding-agent`, formerly `@mariozechner/pi-coding-agent`, CLI: `pi`) supports two headless, non-interactive subprocess integration patterns over standard I/O: one-shot execution via `pi -p --mode json "<prompt>"` and long-lived bidirectional execution via `pi --mode rpc`. Both headless modes emit LF-delimited (`\n`) JSONL frames without requiring a pseudo-terminal (PTY), but RPC mode adds full duplex command-and-event multiplexing (with request IDs, mid-flight steering, aborts, and state queries) along with an extension UI RPC sub-protocol (`extension_ui_request` / `extension_ui_response`). For an external Go orchestrator service, the recommended architecture uses long-lived `pi --mode rpc` worker processes managed via `os/exec.CommandContext`, an LF-delimited scanner with a large buffer (>=16MB to avoid `bufio.ErrTooLong` on file payloads), an in-memory request-response correlation map, automated handlers for blocking `extension_ui_request` dialogs, and explicit `--approve` or `--no-session` flags to bypass interactive project trust checks.

---

## Findings

1. **Invocation Modes and Headless CLI Topology** — Pi natively provides three distinct headless integration modes alongside its default terminal UI (TUI):
   - **One-Shot Print Mode (`pi -p "<prompt>"` / `--print`)**: Executes the prompt, writes the final text response to stdout, and exits. It merges piped standard input into the prompt context if present. [Source](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/README.md)
   - **One-Shot JSON Event Stream Mode (`pi -p --mode json "<prompt>"`)**: Executes the prompt non-interactively while streaming every granular agent lifecycle event (tokens, tool executions, turns) as newline-delimited JSON (JSONL) to stdout. [Source](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/json.md)
   - **Stateful RPC Mode (`pi --mode rpc [options]`)**: Spawns a persistent, headless agent process that listens for JSON commands on `stdin` and writes JSON command responses and asynchronous event frames to `stdout`. This mode is designed for IDE extensions, external daemons, and foreign-language orchestrators. [Source](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/rpc.md)
   - None of the headless modes require a pseudo-terminal (PTY) or virtual terminal emulation; standard pipe redirection (`os/exec.Cmd.StdinPipe` / `StdoutPipe`) is fully supported. [Source](https://piagent.fyi/guides/rpc-and-json-mode/)

2. **JSONL Framing and Request Multiplexing Protocol** — Communication in RPC and JSON mode strictly adheres to line-delimited JSON (LF `\n` framing):
   - **Command Ingestion (stdin)**: Commands are single-line JSON objects. Every command may supply an optional `id` string (e.g. `{"id": "req-1", "type": "prompt", "message": "..."}`).
   - **Command Acknowledgment/Response (stdout)**: Commands are acknowledged with a response frame containing the correlated `id`:
     ```json
     {"id": "req-1", "type": "response", "command": "prompt", "success": true}
     ```
     On failure, responses return `{"id": "req-1", "type": "response", "command": "...", "success": false, "error": "<message>"}`.
   - **Asynchronous Event Stream (stdout)**: Unsolicited lifecycle events do not contain a command `id` (with rare exceptions such as bash command updates referencing their trigger). The external orchestrator must differentiate response frames (`type: "response"`) from streaming event frames. [Source](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/src/modes/rpc/rpc-types.ts)

3. **RPC Command Surface for External Schedulers** — The RPC command protocol provides comprehensive control over the agent loop:
   - **Prompt Execution**: `{"type": "prompt", "message": string, "images"?: ImageContent[], "streamingBehavior"?: "steer" | "followUp"}`.
   - **Mid-Turn Interventions**:
     - `{"type": "steer", "message": string}`: Injects instructions delivered immediately after the active tool call finishes.
     - `{"type": "follow_up", "message": string}`: Queues instructions delivered only after the current agent turn fully completes.
     - `{"type": "clear_queue"}`: Clears queued steering and follow-up instructions.
     - `{"type": "abort"}`: Cancels active streaming, tool execution, or model inference immediately. [Source](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/rpc.md)
   - **State Inspection & Introspection**:
     - `{"type": "get_state"}`: Returns `RpcSessionState` (`model`, `thinkingLevel`, `isStreaming`, `isCompacting`, `sessionId`, `sessionFile`, `messageCount`, `pendingSteeringMessages`, etc.).
     - `{"type": "get_messages"}`: Returns historical session conversation turns.
     - `{"type": "get_entries"}` & `{"type": "get_tree"}`: Exposes the internal session tree DAG nodes.
     - `{"type": "get_commands"}`: Enumerates available prompt templates, skills, and extension slash commands. [Source](https://github.com/earendil-works/pi/blob/v0.80.3/packages/coding-agent/docs/rpc.md)
   - **Runtime Context Operations**:
     - `{"type": "set_model", "provider": string, "model": string}`: Dynamically switches the active LLM backend without restarting the process.
     - `{"type": "set_thinking_level", "level": "off" | "low" | "medium" | "high"}`: Adjusts reasoning effort.
     - `{"type": "compact", "customInstructions"?: string}`: Triggers history summarization and compaction.
     - `{"type": "new_session", "parentSession"?: string}` / `{"type": "switch_session", "sessionFile": string}`: Replaces or forks session state in the same daemon. [Source](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/src/modes/rpc/rpc-types.ts)

4. **Event Streaming Schema & Granularity** — All events conform to the `AgentSessionEvent` hierarchy emitted on stdout:
   - Lifecycle: `agent_start`, `agent_end`, `turn_start`, `turn_end`.
   - Streaming text: `message_start`, `message_update`, `message_end`. Note: `message_update` delivers lightweight incremental chunk deltas rather than quadratic cumulative text buffers.
   - Tool execution: `tool_call_start` (contains `toolName`, `toolCallId`, and parsed `parameters`), `tool_call_end` (contains `toolCallId`, `content`, `isError`, and optional execution `details`).
   - Compaction: `compaction_start` (`reason: "manual" | "threshold" | "overflow"`), `compaction_end`. [Source](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/json.md)

5. **Bidirectional Extension UI Protocol (Deadlock Hazard)** — Extensions and custom skills can prompt the user for input or approval. In RPC mode, Pi converts these calls into a blocking RPC sub-protocol over stdio:
   - **Dialog Methods (`select`, `confirm`, `input`, `editor`)**: Pi emits an `extension_ui_request` on `stdout`:
     ```json
     {"type": "extension_ui_request", "id": "uuid-1", "method": "confirm", "title": "Run Script", "message": "Execute database migration?"}
     ```
     **Critical Integration Rule**: The Pi agent process *blocks execution completely* until the host writes a corresponding `extension_ui_response` on `stdin`:
     ```json
     {"type": "extension_ui_response", "id": "uuid-1", "value": true}
     ```
     If an external orchestrator treats stdout as a write-only log or fails to handle `extension_ui_request`, any extension invoking user interaction will cause a permanent deadlock.
   - **Fire-and-Forget Methods (`notify`, `setStatus`, `setWidget`, `setTitle`, `set_editor_text`)**: Pi emits `extension_ui_request` for status/notification updates without expecting a reply. [Source](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/rpc.md)

6. **Tool Contracts and Tool Control** — By default, Pi equips the model with four primary tools: `read`, `write`, `edit`, and `bash` (plus `grep`, `find`, `ls`, and platform-specific shells like `powershell` on Windows):
   - **Tool Definition Contract**: Custom tools are registered via extensions using `pi.registerTool({ name, label, description, parameters, execute })` where `parameters` uses TypeBox JSON Schema definition, and `execute` returns `{ content: [{ type: "text", text: string }], details?: any }`. [Source](https://pi.dev/docs/latest/extensions)
   - **CLI Filtering Flags**:
     - `--tools <t1,t2>`: Allowlist specific tools.
     - `--exclude-tools <t1,t2>`: Blocklist tools.
     - `--no-builtin-tools` (`-nbt`): Strips built-in filesystem and shell execution tools while retaining extension tools.
     - `--no-tools` (`-nt`): Disables all tools for pure text reasoning. [Source](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/usage.md)

7. **Project Trust, Security, and Headless Permissions** — Pi enforces a project trust security boundary:
   - In interactive TUI mode, Pi prompts the user before loading `.pi/settings.json`, custom skills, packages, and local extensions from untrusted working directories.
   - In non-interactive modes (`-p`, `--mode json`, `--mode rpc`), **Pi never prompts interactively**. Without an existing trust record in `~/.pi/agent/trust.json`, it falls back to `defaultProjectTrust` (`"ask"` or `"never"`), silently ignoring project-local configurations, extensions, and skills.
   - **Orchestrator Best Practice**: External orchestrators running in controlled worker workspaces must explicitly pass the `--approve` (or `-a`) CLI flag to trust project-local resources, or `--no-approve` (`-na`) to enforce strict lockdown. [Source](https://pi.dev/docs/latest/settings)

8. **Session Management and Tree Persistence** — Pi stores conversation DAGs in JSONL session files:
   - Header frame: `{"type":"session","version":3,"id":"...","timestamp":"...","cwd":"..."}` followed by tree entries (`id`, `parentId`, `turn`).
   - `--session <path>`: Specifies the exact `.jsonl` session file path to load or create. If the file does not exist, Pi initializes it.
   - `--no-session`: Runs in an ephemeral in-memory state without reading or writing session history files to disk, ideal for stateless pipeline workers.
   - `--session-dir <dir>`: Sets custom session storage directory. [Source](https://piagent.fyi/guides/rpc-and-json-mode/)

---

## Go Orchestrator Integration Architecture

An external Go orchestrator service (such as a task dispatcher, CI/CD worker, or video pipeline controller) should implement the following architectural design:

```
+-------------------------------------------------------------------------+
|                        Go Orchestrator Service                          |
|                                                                         |
|  +------------------------+             +----------------------------+  |
|  |   Session / Task       |             |   Request Correlation Map  |  |
|  |   Pipeline Manager     |             |   sync.Map[reqID -> chan]  |  |
|  +-----------+------------+             +--------------+-------------+  |
|              |                                         |                |
|              v Prompt / Command                        | Dispatch       |
|  +------------------------+                            | Response       |
|  |     JSONL Encoder      |                            |                |
|  +-----------+------------+                            |                |
|              | stdin (LF-delimited)                    |                |
+--------------|-----------------------------------------|----------------+
               |                                         |
               v                                         |
+--------------------------------------------------------+----------------+
|  pi --mode rpc --approve --provider openai --model gpt-4o               |
|  Subprocess (Node.js runtime, headless, no PTY)                        |
|                                                                         |
|  stdout stream (LF-delimited JSONL):                                    |
|   ├── {"id": "1", "type": "response", ...}   -----> [Correlation Map]  |
|   ├── {"type": "extension_ui_request", ...}  -----> [Auto-Approval Bus] |
|   └── {"type": "tool_call_start"|"turn_end"} -----> [Event Bus / Logs]  |
+-------------------------------------------------------------------------+
```

### 1. Process Lifecycle & Process Group Management
- **Spawn via `exec.CommandContext`**: Always tie the process execution to a Go `context.Context` for timeout and cancellation handling.
- **Process Group Isolation (Tree Termination)**: When Pi runs `bash` or `powershell` commands, subshells are spawned. On cancellation, terminating only the parent Node.js process leaves orphaned tool processes.
  - **Linux/macOS**: Set `SysProcAttr: &syscall.SysProcAttr{Setpgid: true}` and terminate the entire group with `syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)`.
  - **Windows**: Use Job Objects via `golang.org/x/sys/windows` or fall back to `exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))`.

### 2. Large Buffer I/O Handling (Avoiding `bufio.ErrTooLong`)
- Go's default `bufio.Scanner` has a maximum line token limit of 64 KB (`bufio.MaxScanTokenSize = 64 * 1024`).
- Pi outputs large payloads in single lines (e.g. file contents from `read`, unified diffs in `edit`, or base64 data). A default scanner will immediately fail with `bufio.ErrTooLong`.
- **Requirement**: Use `scanner.Buffer(make([]byte, 64*1024), 32*1024*1024)` (allocating up to 32MB max buffer) or read bytes continuously with `bufio.Reader.ReadBytes('\n')`.

### 3. Concrete Go Client Implementation Example

```go
package piorchestrator

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// Inbound and Outbound RPC Wire Types
type RpcCommand struct {
	ID                string `json:"id,omitempty"`
	Type              string `json:"type"`
	Message           string `json:"message,omitempty"`
	StreamingBehavior string `json:"streamingBehavior,omitempty"`
	Provider          string `json:"provider,omitempty"`
	Model             string `json:"model,omitempty"`
}

type RpcResponse struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Command string          `json:"command"`
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   string          `json:"error,omitempty"`
}

type ExtensionUIRequest struct {
	Type    string   `json:"type"`
	ID      string   `json:"id"`
	Method  string   `json:"method"`
	Title   string   `json:"title,omitempty"`
	Message string   `json:"message,omitempty"`
	Options []string `json:"options,omitempty"`
}

type ExtensionUIResponse struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Value any    `json:"value"`
}

type AgentEvent struct {
	Type       string          `json:"type"`
	ToolName   string          `json:"toolName,omitempty"`
	ToolCallID string          `json:"toolCallId,omitempty"`
	Content    json.RawMessage `json:"content,omitempty"`
	Delta      string          `json:"delta,omitempty"`
}

// Client wraps the running Pi subprocess
type PiSubprocessClient struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	reqSeq    uint64
	pending   sync.Map // reqID (string) -> chan RpcResponse
	eventChan chan AgentEvent
	errChan   chan error
}

func StartPiSubprocess(ctx context.Context, workDir, provider, model string) (*PiSubprocessClient, error) {
	args := []string{
		"--mode", "rpc",
		"--approve", // Bypass project trust prompt in headless mode
	}
	if provider != "" {
		args = append(args, "--provider", provider)
	}
	if model != "" {
		args = append(args, "--model", model)
	}

	cmd := exec.CommandContext(ctx, "pi", args...)
	cmd.Dir = workDir

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe error: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe error: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start pi: %w", err)
	}

	client := &PiSubprocessClient{
		cmd:       cmd,
		stdin:     stdin,
		stdout:    stdout,
		eventChan: make(chan AgentEvent, 200),
		errChan:   make(chan error, 1),
	}

	go client.listenStdout()
	return client, nil
}

func (c *PiSubprocessClient) listenStdout() {
	reader := bufio.NewReaderSize(c.stdout, 32*1024*1024)

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				c.errChan <- err
			}
			close(c.eventChan)
			return
		}

		if len(line) == 0 || line[0] != '{' {
			continue
		}

		// Peek message type
		var peek struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		}
		if err := json.Unmarshal(line, &peek); err != nil {
			continue
		}

		switch peek.Type {
		case "response":
			var resp RpcResponse
			if err := json.Unmarshal(line, &resp); err == nil {
				if chVal, ok := c.pending.LoadAndDelete(resp.ID); ok {
					ch := chVal.(chan RpcResponse)
					ch <- resp
					close(ch)
				}
			}
		case "extension_ui_request":
			// Handle blocking extension UI confirmation to prevent subprocess deadlock
			var req ExtensionUIRequest
			if err := json.Unmarshal(line, &req); err == nil {
				go c.autoAnswerExtensionUI(req)
			}
		default:
			// Regular agent event
			var ev AgentEvent
			if err := json.Unmarshal(line, &ev); err == nil {
				select {
				case c.eventChan <- ev:
				default:
				}
			}
		}
	}
}

func (c *PiSubprocessClient) autoAnswerExtensionUI(req ExtensionUIRequest) {
	// For dialog methods, reply automatically according to orchestrator security policy
	switch req.Method {
	case "confirm":
		resp := ExtensionUIResponse{Type: "extension_ui_response", ID: req.ID, Value: true}
		_ = c.writeJSON(resp)
	case "select":
		// Select first available option as default fallback
		val := ""
		if len(req.Options) > 0 {
			val = req.Options[0]
		}
		resp := ExtensionUIResponse{Type: "extension_ui_response", ID: req.ID, Value: val}
		_ = c.writeJSON(resp)
	case "input", "editor":
		resp := ExtensionUIResponse{Type: "extension_ui_response", ID: req.ID, Value: ""}
		_ = c.writeJSON(resp)
	}
}

func (c *PiSubprocessClient) writeJSON(v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	_, err = c.stdin.Write(payload)
	return err
}

func (c *PiSubprocessClient) Prompt(ctx context.Context, message string) (*RpcResponse, error) {
	reqID := fmt.Sprintf("req-%d", atomic.AddUint64(&c.reqSeq, 1))
	respChan := make(chan RpcResponse, 1)
	c.pending.Store(reqID, respChan)

	cmd := RpcCommand{
		ID:      reqID,
		Type:    "prompt",
		Message: message,
	}

	if err := c.writeJSON(cmd); err != nil {
		c.pending.Delete(reqID)
		return nil, fmt.Errorf("write prompt failed: %w", err)
	}

	select {
	case <-ctx.Done():
		c.pending.Delete(reqID)
		_ = c.Abort()
		return nil, ctx.Err()
	case resp := <-respChan:
		if !resp.Success {
			return nil, fmt.Errorf("pi rpc error: %s", resp.Error)
		}
		return &resp, nil
	}
}

func (c *PiSubprocessClient) Abort() error {
	return c.writeJSON(RpcCommand{Type: "abort"})
}

func (c *PiSubprocessClient) Events() <-chan AgentEvent {
	return c.eventChan
}
```

---

## Comparison Matrix: Headless Execution Approaches

| Feature / Dimension | One-Shot Print (`-p`) | JSON Stream (`--mode json -p`) | Persistent RPC (`--mode rpc`) |
| :--- | :--- | :--- | :--- |
| **Invocation Model** | Transient CLI subprocess | Transient CLI subprocess | Stateful daemon subprocess |
| **Output Channel** | `stdout` raw string | `stdout` JSONL event stream | `stdout` JSONL responses & events |
| **Input Channel** | CLI argument / stdin pipe | CLI argument / stdin pipe | `stdin` JSONL command protocol |
| **Mid-Turn Control** | None (Wait for exit) | None (Wait for exit) | `steer`, `follow_up`, `abort` |
| **Dynamic Model Switch** | Requires re-spawn | Requires re-spawn | Supported (`set_model`) |
| **Extension UI Dialogs** | Fails / Defaulted | Fails / Defaulted | Handled via `extension_ui_request` |
| **Session Tree Inspection**| External file read only | External file read only | Dynamic (`get_tree`, `get_state`) |
| **Process Overhead** | Node VM startup per task | Node VM startup per task | Amortized across tasks |
| **Ideal Use Case** | Shell scripts & batch runs | Single-task CI execution | External Go Orchestrator Daemon |

---

## Sources

- **Kept**: [Pi Official RPC Mode Documentation](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/rpc.md) (`https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/rpc.md`) — Authoritative specification of the JSON RPC protocol, command payload definitions, and bidirectional extension UI handshake.
- **Kept**: [Pi JSON Event Mode Specification](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/json.md) (`https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/json.md`) — Official event hierarchy and JSONL schema for headless event consumption.
- **Kept**: [Pi RPC Types Source Code](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/src/modes/rpc/rpc-types.ts) (`https://github.com/earendil-works/pi/blob/main/packages/coding-agent/src/modes/rpc/rpc-types.ts`) — Upstream TypeScript source containing exact wire definitions for `RpcCommand`, `RpcResponse`, `RpcSessionState`, and `RpcExtensionUIRequest`.
- **Kept**: [Pi Project Settings & Trust Documentation](https://pi.dev/docs/latest/settings) (`https://pi.dev/docs/latest/settings`) — Primary rules on project trust behavior in non-interactive/headless environments (`--approve`, `defaultProjectTrust`).
- **Kept**: [Pi CLI Usage & Tool Options](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/usage.md) (`https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/usage.md`) — Details flags for tool allowlisting, blocklisting, and disabling built-in tools.
- **Kept**: [Pi Extensions System Specification](https://pi.dev/docs/latest/extensions) (`https://pi.dev/docs/latest/extensions`) — Specifications for custom tool registration contracts (`pi.registerTool`) and lifecycle hooks.
- **Kept**: [Piagent RPC & JSON Mode Deep Dive](https://piagent.fyi/guides/rpc-and-json-mode/) (`https://piagent.fyi/guides/rpc-and-json-mode/`) — Practical host integration patterns, framing gotchas (LF delimiter rules), and command surfaces.
- **Dropped**: Generic npm registry landing pages (`https://www.npmjs.com/package/@mariozechner/pi-coding-agent`) — Redundant version metadata without architectural or protocol specifics.
- **Dropped**: Unofficial forks and re-exports without unique documentation — Excluded to preserve fidelity to upstream `@earendil-works/pi` contracts.

---

## Gaps

1. **Native Tool Interception in RPC Mode**: The upstream RPC protocol provides commands to run shell tasks (`bash`) and inspect tools, but does not provide an RPC command to register arbitrary Go-native tool functions on the fly over stdin/stdout.
   - *Workaround/Next Step*: To provide Go-backed tools to the Pi agent, write a thin TypeScript extension placed in `.pi/extensions/` or passed via `-e` that registers tools with `pi.registerTool()`. The extension can either execute local helper scripts or forward calls over a local UNIX domain socket / HTTP loopback to the Go orchestrator.
2. **OS Sandbox Boundaries**: While `--approve` bypasses interactive trust checks, it does not sandbox the underlying filesystem or `bash` execution.
   - *Suggested Next Step*: If running untrusted agent workloads, run the Pi subprocess inside a container (Docker/Podman) or a Linux network/mount namespace (bubblewrap).
