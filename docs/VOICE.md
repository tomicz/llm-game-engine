# Voice-to-text integration

This document describes the voice input feature: hold **Cmd+R** (Mac) or **Super+R** (Linux) to record from the microphone; on release, the audio is transcribed and the text is sent to the chat/LLM as if you had typed it.

---

## What was implemented

### 1. Voice-to-text module and library

- **Location:** `modules/voice-to-text/` (can be a Git submodule; see [Submodule](#submodule) below).
- **Standalone CLI:** `go run ./cmd/vtt` from the voice-to-text directory records until you press Enter, then transcribes with [whisper.cpp](https://github.com/ggerganov/whisper.cpp). See `modules/voice-to-text/README.md`.
- **Library (`vttlib`):** The engine uses the same logic in-process:
  - **`vttlib.NewRecorder(root)`** — `root` is the path to the voice-to-text module (e.g. `modules/voice-to-text`).
  - **`Start()`** — starts ffmpeg capturing from the default mic (no stdin wait).
  - **`Stop()`** — sends `q` to ffmpeg to finalize the WAV.
  - **`Transcribe(ctx)`** — runs whisper-cli on the WAV and returns the transcript (and writes `output.txt` in the module root).

The game resolves the module root from the working directory: `modules/voice-to-text` or `../../modules/voice-to-text` when run from `cmd/game`.

### 2. Cmd+R recording in the game

- **Key binding:** Hold **Cmd+R** (or Super+R) to start recording; **release** to stop, transcribe, and send the result to the LLM.
- **Flow:** On key down (combo just pressed) → start recording and log `Voice: recording…`. On key up → stop, log `Voice: stopped, transcribing…`, then in a goroutine run `Transcribe()`, log the transcript and “sent to chat”, and call the same handler used for terminal natural language (`OnNaturalLanguage(transcript)`).
- **Chat/terminal logs:** All steps are written to the terminal log (and shown in the chat area when the terminal is open):
  - `Voice: recording…`
  - `Voice: stopped, transcribing…`
  - `Voice (transcript): <text>` — raw output from Whisper.
  - Either `Voice (skipped, too short; not sent to chat): <text>` or `Voice (sent to chat): <text>`.
  - Then the usual `Thinking…` and agent summary when something was sent.

### 3. Short-transcript filter

- Transcripts shorter than **5 characters** are not sent to the LLM (to avoid random actions from noise or single words like “you”).
- They are still logged as transcript and “skipped, too short” so you can see what was said.

### 4. Recording indicator (UI)

- When the **chat/terminal is closed** and voice is **recording**, a small indicator is drawn at the **bottom-left**:
  - Red dot with darker outline.
  - “Recording...” text in red (uses UI font when loaded).
- The indicator is **hidden** when the terminal is open or when not recording.

### 5. Ollama model lock

- When the LLM provider is **Ollama** (no Groq/Cursor/OpenAI keys set), the **`cmd model`** command is disabled: it returns an error and does not change the model. This prevents the LLM or voice from accidentally switching the Ollama model via natural language or `run_cmd`.
- Default Ollama model when none is set (or when the saved config is a cloud model name) is **`qwen3-coder:30b`**. Cloud-style names like `llama-3.3-70b-versatile` or `gpt-4o-mini` in config are overridden to this when using Ollama, and the updated config is saved.

### 6. Go module wiring

- **`go.mod`:** The game depends on `github.com/tomicz/speak-to-agent` with a `replace` to `./modules/voice-to-text`, so the engine uses the local `vttlib` package.
- **Imports:** `cmd/game/main.go` imports `github.com/tomicz/speak-to-agent/vttlib` for the recorder.

---

## Code locations (summary)

| What | Where |
|------|--------|
| vttlib API (Start/Stop/Transcribe) | `modules/voice-to-text/vttlib/vtt.go` |
| CLI that uses vttlib | `modules/voice-to-text/cmd/vtt/main.go` |
| Cmd+R handling, transcript → chat, logs | `cmd/game/main.go` (update loop and draw) |
| Recording indicator draw | `cmd/game/main.go` (draw function, when `!term.IsOpen() && voiceRecording`) |
| Ollama model lock and default | `cmd/game/main.go` (model command + Ollama branch of client switch) |

---

## Submodule

`modules/voice-to-text` can be a **Git submodule** pointing at its own repo (e.g. `git@github.com:tomicz/voice-to-text.git`). The `replace` in `go.mod` still points at `./modules/voice-to-text`; once that path is the submodule checkout, nothing else in the engine needs to change.

- Clone with submodules: `git clone --recurse-submodules <game-engine-url>` or after clone run `git submodule update --init --recursive`.
- See the earlier “Plan: Push voice-to-text and make it a submodule” for step-by-step conversion.

---

## Prerequisites for voice

- **ffmpeg** (e.g. `brew install ffmpeg` on macOS).
- **whisper.cpp** built under `modules/voice-to-text/third_party/whisper` and a model (e.g. large-v3-turbo) in `.../whisper/models/`. See `modules/voice-to-text/README.md`.
