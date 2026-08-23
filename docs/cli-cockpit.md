# Astra CLI cockpit

The connected TTY is an application shell, not a log printer. `astra connect`
opens the cockpit when both stdin and stdout are terminals. It uses an alternate
screen, so leaving Astra returns the user to the previous terminal contents.

## Layout contract

- **Header** — project, provider/model, and the current status.
- **Navigation rail** — Chat, Dashboard, Workspace, Mind Palace, Sessions, Sync,
  and Agents. `Ctrl+1`–`Ctrl+7`, `Tab`, and `Shift+Tab` navigate without losing the
  chat transcript.
- **Main panel** — a scrollable transcript or a read-only project view.
- **Composer** — a dedicated multiline input area. Output cannot overwrite a
  draft because the model never writes directly to the terminal.
- **Footer** — the current keyboard contract is always visible.

## Chat behavior

`Enter` submits a request and `Ctrl-J` inserts a newline. Large prompts are
bounded at the same 12,000-character limit as the plain CLI. The transcript
keeps user messages, plans, activated tool documentation, action parameters,
action results, errors, and the final streamed response visually distinct.

The CLI remains responsive while Astra works. Each session contains durable
runs. The first submitted message opens a run and receives a stable `run_id`;
messages typed while that run is active are appended as `user_update` records
to the same run and reassessed at the next safe checkpoint. A new run starts
only after the active run finishes. Every stream event carries both
`session_id` and `run_id`, so output remains traceable without interleaving
unrelated tasks.

## Controls

| Key | Effect |
| --- | --- |
| Enter | Send the current draft |
| Ctrl-J | Insert a newline |
| Ctrl+1…Ctrl+6 | Open a view directly |
| Tab / Shift+Tab | Move between views |
| Ctrl-P / Ctrl-R | Pause / resume at agent checkpoints |
| Ctrl-X | Request safe cancellation |
| Ctrl-L | Clear the visible chat transcript |
| Ctrl-Q | Exit the cockpit |

Colon commands remain available in Chat: `:model`, `:dashboard`, `:workspace`,
`:mindpalace`, `:sessions`, `:sync`, `:agents`, `:scopes`, `:prompts`, `:attach`,
`:pause`, `:resume`, `:stop`, `:clear`, `:pwd`, and `:help`.

## Plain mode

`astra connect --plain` bypasses the alternate-screen cockpit. It is intended
for CI, shell pipes, transcript capture, and debugging terminal capability
problems. It retains the multiline editor, bracketed paste, history, queue,
and streaming event renderer.

## Workspace access preflight

Before creating a session, `astra connect` performs a small read-only check of
the connected directory. If macOS denies access with `operation not permitted`,
Astra does not start an agent or create a misleading run. It shows a recovery
screen with:

- `o` — open macOS Privacy & Security settings;
- `r` — retry after the user grants access; or
- `q` — exit without starting the session.

The Astra scope registry and macOS privacy permissions are separate. The
registry records which paths Astra is authorized to target; it cannot grant
Full Disk Access to the process that launched the CLI. Non-interactive runs
report the blocker and exit so automation cannot hang waiting for input.

## Storage and authority

Astra-owned session state defaults to `~/.astra/projects/`, outside connected
repositories. Set `ASTRA_DATA_DIR` to relocate it.

Views are intentionally read-only. They count and list the project workspace,
external Astra-managed artifacts/attachments, global user Mind Palace files, session
manifests/evidence, and local sync records. A view never implies that a file was
uploaded or changed. Mutations still require an agent action or an explicit
local command.
