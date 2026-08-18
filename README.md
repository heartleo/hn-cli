# hn

A terminal client for Hacker News.

![Go version](https://img.shields.io/badge/go-1.25%2B-blue)
[![CI](https://img.shields.io/github/actions/workflow/status/heartleo/hn-cli/release.yml)](https://github.com/heartleo/hn-cli/actions)
[![Release](https://img.shields.io/github/v/release/heartleo/hn-cli)](https://github.com/heartleo/hn-cli/releases)
[![Downloads](https://img.shields.io/github/downloads/heartleo/hn-cli/total)](https://github.com/heartleo/hn-cli/releases)
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

**Homebrew** (macOS / Linux):

```bash
$ brew install heartleo/tap/hn
```

**winget** (Windows):

```powershell
$ winget install heartleo.hn
```

**curl** (macOS / Linux):

```bash
$ curl -fsSL https://raw.githubusercontent.com/heartleo/hn-cli/main/install.sh | sh
```

No `sudo` needed — the binary is installed under your home directory. The install
directory is picked from the first of these that is set:

| Source           | Value                   |
| ---------------- | ----------------------- |
| `HN_INSTALL_DIR` | used as-is              |
| `XDG_BIN_HOME`   | used as-is              |
| `XDG_DATA_HOME`  | `$XDG_DATA_HOME/../bin` |
| *(default)*      | `$HOME/.local/bin`      |

To install somewhere else:

```bash
$ curl -fsSL https://raw.githubusercontent.com/heartleo/hn-cli/main/install.sh | HN_INSTALL_DIR=~/bin sh
```

The script warns if the install directory is not on your `$PATH`, and prints the
`export PATH=...` line to add to your shell profile.

The script never calls `sudo`. To install system-wide, run it as root yourself:

```bash
$ curl -fsSL https://raw.githubusercontent.com/heartleo/hn-cli/main/install.sh -o install.sh
$ sudo env HN_INSTALL_DIR=/usr/local/bin sh install.sh
```

**Prebuilt binaries** — download from [GitHub Releases](https://github.com/heartleo/hn-cli/releases)

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

## Claude Code Plugin

This repo also ships [**hn**](plugins/hn), a Claude Code plugin that brings Hacker News to Claude.

```
/plugin marketplace add heartleo/hn-cli
/plugin install hn@hn-cli
```

| Skill       | What it does                                                        |
| ----------- | ------------------------------------------------------------------- |
| `hn-digest` | Scan the front page — today's themes, hot discussions, industry mix |

Then just ask: `what's on HN today`. Ask in any language and the digest comes back in that language. See the [plugin README](plugins/hn) for details.

The digest is also published daily at [hndigest.heartleo.dev](https://hndigest.heartleo.dev).

## DeepSeek Harness Plugin

For [DeepSeek Harness](https://deepseek-harness.github.io/deepseek-harness/) there is [**hacker-news**](plugins/hacker-news), a plain-ESM plugin that talks to the public HN APIs directly — no `hn` binary needed.

```sh
dsh plugin --profile <name> add -w github:heartleo/hn-cli#path:/plugins/hacker-news
```

Ask naturally, for example: `what's popular on HN?`, `search HN for sqlite`, or `read this HN discussion`.

| Tool         | What it does                                            |
| ------------ | ------------------------------------------------------- |
| `hn_stories` | Ranked feeds — top / new / best / ask / show / job       |
| `hn_item`    | An item plus its reply tree in reading order            |
| `hn_search`  | Algolia search, by relevance or date                    |
| `hn_user`    | Karma, account age, bio, submission count               |

See the [plugin README](plugins/hacker-news) for configuration.

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

## Contributing

Issues, bug reports, feature suggestions, and general feedback are very welcome:

- 🐛 [Report a bug](https://github.com/heartleo/hn-cli/issues/new)
- 💡 [Suggest a feature](https://github.com/heartleo/hn-cli/issues/new)

> [!NOTE]
> This project is maintained in my spare time, and I currently do not have enough capacity to properly review external pull requests. See [CONTRIBUTING.md](CONTRIBUTING.md) for details. Thank you for your understanding!
