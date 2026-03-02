# 🔥 Fllint

**Local AI for everyone — not just for engineers.**

Fllint is an open-source application that runs large language models entirely on your Mac or Linux PC. No cloud, no API keys, no telemetry. Just AI that works — out of the box, without a terminal, and without scattering files all over your system.

🌐 [fllint.io](https://fllint.io)

---

## Why Fllint?

Running AI locally should be simple. Download, open, chat — that's it. No configuration, no command line, no guesswork about which model to pick or which settings to change. Fllint ships with a ready-to-use model included, so there's nothing to set up after installing. Open the app and start chatting.

At the same time, local AI tools shouldn't treat your computer like their own. Fllint lives in a single folder. Every model, every conversation, every config file — one folder. Delete it and Fllint is gone. Completely. No leftover daemons, no hidden caches, no ghost processes.

And for those who *do* want full control: Pro Mode exposes every parameter, every backend setting, every knob. Fllint scales from "I just want to chat" to "I need to fine-tune inference behavior for my research workflow."

---

## Features

### v1.0

- **Chat with three model tiers** — Lite, Standard, and Pro (ships with Qwen 3.5 35B A3B)
- **Lite model included** — bundled with the download, ready to use immediately
- **Image upload** — send images to vision-capable models directly in the chat
- **Single-folder architecture** — everything in one place, clean uninstall guaranteed
- **Pro Mode** — full control over model parameters and backend settings

### Roadmap

- 🌐 **Web Search** — search the internet from the chat
- 📄 **Document Upload** — upload and query PDFs, text files, and more
- 🎙️ **Voice Input** — local speech-to-text via Whisper
- 🧠 **Memory & Projects** — persistent memory with RAG, optional OCR via GLM-OCR
- 🧩 **Custom Models** — add any compatible LLM from Hugging Face
- 💾 **External Disk Mode** — zero footprint on the host machine
- 🔬 **Deep Research Agent** — multi-step research with source synthesis
- 🔌 **MCP Support** — Model Context Protocol for tool and service integrations
- 🐍 **Skills** — sandboxed Python execution with package support

---

## How It Works

Fllint consists of two parts: a **launcher** and a **data folder**.

The launcher is a lightweight app that lives in your Applications folder (macOS) or runs as a `.desktop` entry or AppImage (Linux). Double-click it and Fllint starts a local llama.cpp server and opens the UI in your browser — no terminal involved. A tray icon shows that Fllint is running and lets you quit cleanly.

The data folder contains everything else: models, conversations, config. One folder, your choice where it lives.

**Uninstall:** delete the launcher, delete the folder. That's it.

---

## Architecture

Under the hood, Fllint is a **localhost web application** powered by a **llama.cpp backend**.

- **One UI for all platforms** — no separate native apps for macOS and Linux
- **Browser-based** — works in any modern browser, feels like a native app
- **llama.cpp** — battle-tested inference, optimized for Apple Silicon and GPU on Linux

---

## Hardware Requirements

| Model Tier | VRAM | Best For |
|---|---|---|
| **Lite** | 8 GB+ | Quick answers, lighter hardware |
| **Standard** | 16 GB+ | Balanced quality and speed |
| **Pro** | 32 GB+ | Maximum capability (Qwen 3.5 35B A3B) |

**Supported platforms:** macOS (Apple Silicon required) · Linux (x86_64, dedicated GPU with 8 GB+ VRAM)

---

## Who Is This For?

**If you've never run a local model before** — Fllint is your starting point. Download, open, chat. You don't need to know what a GGUF is.

**If you want full control** — Pro Mode gives you every parameter without compromising on a clean, respectful install.

**If you care about privacy** — everything stays local. No data leaves your machine, ever.

---

<p align="center">
  <i>Local AI, without the hassle.</i>
</p>
