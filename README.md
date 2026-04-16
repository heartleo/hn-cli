# hn

A terminal client for Hacker News.

![Go version](https://img.shields.io/badge/go-1.25%2B-blue)
[![Release](https://img.shields.io/github/v/release/heartleo/hn-cli)](https://github.com/heartleo/hn-cli/releases)
![License](https://img.shields.io/badge/license-MIT-green)

<!-- English | [中文](README.zh.md) -->

![demo](docs/demo.gif)

## Features

- 📰 **Story browser** — Top, New, Best, Ask HN, Show HN with tab switching
- 💬 **Comment threads** — navigate with `j/k`, fold/unfold, lazy-load reply trees
- 🌐 **Translation** — translate a title with `t`, all visible titles with `T`, or a selected comment
- 🔄 **Soft refresh** — refresh stories or comments without restarting
- 🎨 **Themes** — hn, mocha, dracula, tokyo, nord, gruvbox
- ⚡ **Progressive loading** — visible range loads first, more fetched as you scroll

## Install

**Prebuilt binaries** — download from [GitHub Releases](https://github.com/heartleo/hn-cli/releases):

| Platform        | Archive                             |
| --------------- | ----------------------------------- |
| Linux x86\_64   | `hn_<version>_linux_x86_64.tar.gz`  |
| Linux arm64     | `hn_<version>_linux_arm64.tar.gz`   |
| macOS x86\_64   | `hn_<version>_darwin_x86_64.tar.gz` |
| macOS arm64     | `hn_<version>_darwin_arm64.tar.gz`  |
| Windows x86\_64 | `hn_<version>_windows_x86_64.zip`   |
| Windows arm64   | `hn_<version>_windows_arm64.zip`    |

**Go install** (requires Go 1.25+):

```bash
$ go install github.com/heartleo/hn-cli/cmd/hn@latest
```

**Build from source:**

```bash
$ git clone https://github.com/heartleo/hn-cli
$ cd hn
$ go build -o hn ./cmd/hn
```

## Quick Start

```bash
$ hn        # top stories
$ hn new    # new stories
$ hn best   # best stories
```

## Commands

### Browse

![browse demo](docs/demo-browse.gif)

Opens the interactive TUI. Defaults to Top stories; switch tabs with `←/→`.

```bash
$ hn        # top stories (default)
$ hn top
$ hn new
$ hn best
$ hn ask
$ hn show
```

### Comments

![comments demo](docs/demo-comments.gif)

Press `Enter` on any story to open its comment thread.

- navigate with `↑/↓` or `k/j`
- press `Enter` to expand or collapse a reply tree
- press `Space` to fold or unfold the selected comment
- press `C` / `E` to fold or unfold all
- press `r` to jump to the root comment
- press `R` to soft refresh
- press `Esc` to go back

### Translation

Translates via any OpenAI-compatible chat completions API. See [Configuration](#translation-1) to set up an API key.

```
t   translate selected story title (toggle cached translation)
T   translate all visible titles in one batch request
t   translate selected comment (in comment view)
```

### theme

```bash
$ hn theme          # show current
$ hn theme nord     # set globally
```

Available: `hn` · `mocha` · `dracula` · `tokyo` · `nord` · `gruvbox`

## Keys

### Story List

| Key            | Action                       |
| -------------- | ---------------------------- |
| `Enter`        | Open comment thread          |
| `o`            | Open in browser              |
| `t`            | Translate selected title     |
| `T`            | Translate all visible titles |
| `←` / `→`      | Switch tab                   |
| `r`            | Refresh                      |
| `?`            | Toggle help                  |
| `q` / `Ctrl+C` | Quit                         |

### Comment Thread

| Key            | Action                         |
| -------------- | ------------------------------ |
| `j` / `↓`      | Next comment                   |
| `k` / `↑`      | Previous comment               |
| `gg`           | Back to top                    |
| `r`            | Jump to root comment           |
| `Enter`        | Expand / collapse replies      |
| `Space`        | Fold / unfold selected comment |
| `C` / `E`      | Fold / unfold all              |
| `t`            | Translate selected comment     |
| `R`            | Soft refresh                   |
| `o`            | Open story in browser          |
| `Esc`          | Back to list                   |
| `?`            | Toggle help                    |
| `Q` / `Ctrl+C` | Quit                           |

## Configuration

`hn` reads `~/.config/hn/config.json`. A `.env` file in the working directory is also loaded automatically; environment variables take precedence over the config file.

### Translation

| Variable               | Default                     | Description     |
| ---------------------- | --------------------------- | --------------- |
| `HN_TRANSLATE_API_URL` | `https://api.openai.com/v1` | API base URL    |
| `HN_TRANSLATE_API_KEY` | —                           | API key         |
| `HN_TRANSLATE_MODEL`   | `gpt-4o-mini`               | Model name      |
| `HN_TRANSLATE_LANG`    | `Chinese`                   | Target language |

Example `.env`:

```bash
HN_TRANSLATE_API_KEY=sk-...
HN_TRANSLATE_LANG=Chinese
```

### Theme

| Variable   | Description                           |
| ---------- | ------------------------------------- |
| `HN_THEME` | Override theme without editing config |

Example `~/.config/hn/config.json`:

```json
{
  "theme": "mocha",
  "translate": {
    "api_url": "https://api.openai.com/v1",
    "api_key": "sk-...",
    "model": "gpt-4o-mini",
    "language": "Chinese"
  }
}
```
