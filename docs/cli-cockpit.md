# Astra CLI cockpit

The connected TTY is an application shell, not a log printer. `astra connect`
opens the cockpit when both stdin and stdout are terminals. It uses an alternate
screen, so leaving Astra returns the user to the previous terminal contents.

## Layout contract

- **Header** — project, provider/model, and the current status.
- **Navigation rail** — Chat, Dashboard, Workspace, Mind Palace, Sessions, and
  Sync. `Ctrl+1`–`Ctrl+6`, `Tab`, and `Shift+Tab` navigate without losing the
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

Requests may run concurrently. Each request owns its own stream buffer; a
finished response is committed as one assistant entry instead of emitting a
new terminal line for every token. This is what prevents the interleaving seen
in the older prompt-plus-ANSI implementation.

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
`:mindpalace`, `:sessions`, `:sync`, `:attach`, `:pause`, `:resume`, `:stop`,
`:clear`, `:pwd`, and `:help`.

## Plain mode

`astra connect --plain` bypasses the alternate-screen cockpit. It is intended
for CI, shell pipes, transcript capture, and debugging terminal capability
problems. It retains the multiline editor, bracketed paste, history, queue,
and streaming event renderer.

## Storage and authority

Astra-owned session state defaults to `~/.astra/projects/`, outside connected
repositories. Set `ASTRA_DATA_DIR` to relocate it.

Views are intentionally read-only. They count and list the project workspace,
external Astra-managed artifacts/attachments, global user Mind Palace files, session
manifests/evidence, and local sync records. A view never implies that a file was
uploaded or changed. Mutations still require an agent action or an explicit
local command.
