# Fllint

**Local AI for everyone — not just for engineers.**

Fllint is an open-source application that runs large language models entirely on your Mac. No cloud, no API keys, no telemetry. Just AI that works — out of the box, without a terminal, and without scattering files all over your system.

[fllint.io](https://fllint.io)

---

## Why Fllint?

Running AI locally should be simple. Download, open, chat — that's it. No configuration, no command line, no guesswork about which model to pick or which settings to change. Fllint ships everything you need, so there's nothing to set up after installing.

At the same time, local AI tools shouldn't treat your computer like their own. Fllint lives in a single folder. Every model, every conversation, every config file — one folder. Delete it and Fllint is gone. Completely. No leftover daemons, no hidden caches, no ghost processes. Run it from an external SSD and there's zero footprint on the host machine.

And for those who *do* want full control: Pro Mode exposes every parameter, every backend setting, every knob. Fllint scales from "I just want to chat" to "I need to fine-tune inference behavior for my research workflow."

---

## Features

- **Three model tiers** — Lite, Standard, and Pro, downloadable from within the app
- **Custom models** — add any GGUF model with optional vision projection (mmproj) support
- **External providers** — connect to Ollama or other OpenAI-compatible servers
- **Image upload** — send images to vision-capable models directly in the chat
- **Document upload** — attach PDFs with optional OCR text extraction via GLM-OCR
- **Conversation history** — automatic titles, persistent storage, full CRUD
- **Single-folder architecture** — everything in one place, clean uninstall guaranteed
- **External disk support** — run entirely from an external SSD with zero trace on the host
- **Pro Mode** — full control over model parameters, context size, GPU layers, and backend settings
- **Multi-model loading** — load multiple models simultaneously and switch between them per tab
- **Helper models** — dedicated small models for conversation summaries and OCR
- **Customizable accent color** — personalize the UI with preset or custom colors
- **Auto-updates** — check for and install updates directly from the app via Sparkle

---

## How It Works

Fllint is a **macOS application** consisting of a launcher and a data folder.

Double-click the app and Fllint starts a local llama.cpp server and opens the UI in your browser — no terminal involved. A menu bar icon shows that Fllint is running and lets you quit cleanly.

The data folder contains everything else: models, conversations, config. One folder, your choice where it lives.

**Uninstall:** delete the app, delete the folder. That's it.

---

## Architecture

Under the hood, Fllint is a **localhost web application** powered by a **llama.cpp backend**.

- **Go backend** — serves a SvelteKit SPA embedded in a single binary via chi router
- **SvelteKit frontend** — Svelte 5 with runes, adapter-static in SPA mode
- **llama.cpp** — battle-tested inference engine, optimized for Apple Silicon
- **SSE streaming** — real-time token streaming from model to browser
- **Browser-based UI** — works in any modern browser, feels like a native app

---

## Hardware Requirements

| Model Tier | RAM | Best For |
|---|---|---|
| **Lite** (~2 GB) | 8 GB+ | Quick answers, lighter hardware |
| **Standard** (~9 GB) | 16 GB+ | Balanced quality and speed |
| **Pro** (~22 GB) | 32 GB+ | Maximum capability |

**Supported platform:** macOS (Apple Silicon required)

---

## Who Is This For?

**If you've never run a local model before** — Fllint is your starting point. Download, open, chat. You don't need to know what a GGUF is.

**If you want full control** — Pro Mode gives you every parameter without compromising on a clean, respectful install.

**If you care about privacy** — everything stays local. No data leaves your machine, ever.

---

<p align="center">
  <i>Local AI, without the hassle.</i>
</p>
