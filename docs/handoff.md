# Handoff

Updated: 2026-08-18 (Asia/Shanghai)

## Objective

Migrate the DeepSeek Harness Hacker News plugin from `../hn` into this repository, make it easy to invoke from natural language, and keep local/CI validation repeatable.

## Current Status

- The plugin lives in `plugins/hacker-news` as an independent ESM package.
- It is published to npm as [`dsh-hacker-news@0.1.0`](https://www.npmjs.com/package/dsh-hacker-news).
- The migration is committed and pushed to `main` as `9c0d6b2`.
- The community-listing submission is open as [awesome-dsh-plugin PR #1622](https://github.com/awesome-dsh-plugin/awesome-dsh-plugin/pull/1622); it is ready for review and its checks have passed.
- After installing or upgrading the plugin, restart DSH Web so it loads the new bundle.

## Implemented

### Migrated plugin

- `hn_stories`: ranked top/new/best/ask/show/job feeds.
- `hn_item`: one item plus its breadth-first reply tree.
- `hn_search`: Algolia search by relevance or date.
- `hn_user`: user profile lookup.
- Repository metadata, install URLs, and the default user agent now reference `heartleo/hn-cli`.

### Invocation experience

- A `tool:hacker-news` system-prompt section tells the model when to choose each HN tool and to prefer them over general web search for HN requests.
- Tool cards use readable titles such as `Hacker News · New` and `Hacker News · Search: sqlite`.

### Validation and CI

- `npm test` runs three offline routing, presentation, and package-metadata tests, then the live Firebase/Algolia integration check.
- The integration check validates all four tool outputs against their schemas and covers representative rejection paths.
- `npm pack --dry-run` includes the runtime, `load-check.mjs`, and routing/presentation tests.
- `.github/workflows/plugin.yml` installs the DSH plugin dependencies and runs `npm test` when plugin files change.
- Independent review found no Critical or Important issue for the migrated plugin and its natural-language routing.

## Community Listing

The submission to [awesome-dsh-plugin](https://github.com/awesome-dsh-plugin/awesome-dsh-plugin) is [PR #1622](https://github.com/awesome-dsh-plugin/awesome-dsh-plugin/pull/1622). It adds the registry metadata below and its generated READMEs. The package declares `dsh.bundle`, contains runnable code and tests, uses official DSH packages as peers, and has explicit ranges that include DSH `0.1.0-rc.6`.

The registry entry is `data/plugins/heartleo__hn-cli--plugins-hacker-news.yml`:

```yaml
url: https://github.com/heartleo/hn-cli/tree/main/plugins/hacker-news
name: heartleo/hn-cli#hacker-news
category: tools
description:
  en: Hacker News tools for feeds, discussion threads, search, and user profiles.
  zh: 用于获取 Hacker News 榜单、讨论串、搜索和用户资料的工具。
```

The registry workflow requires `npm ci` and `node scripts/generate-readme.mjs`; generated READMEs must be committed with the YAML file and not edited by hand. The current PR already follows that workflow.

## Install and Use

DSH profiles are pnpm workspace roots, so `add` must receive `-w`. Install the npm package:

```powershell
dsh plugin --profile web add -w dsh-hacker-news
```

If a configured npm mirror has not indexed the release, use the public npm registry for this shell session:

```powershell
$env:npm_config_registry = 'https://registry.npmjs.org/'
dsh plugin --profile web add -w dsh-hacker-news
```

Restart `dsh web`, then ask naturally, for example: “What’s popular on Hacker News?”, “Search HN for SQLite WASM”, or “Read HN item 49322107 and summarize the discussion.”

For development, local paths are resolved from the profile directory, so use an absolute path:

```powershell
dsh plugin --profile web add -w "D:/myspace/github/OPEN/hn-cli/plugins/hacker-news"
```

Repository-source form:

```powershell
dsh plugin --profile web add -w github:heartleo/hn-cli#path:/plugins/hacker-news
```

Confirm both the dependency and bundle layer:

```powershell
dsh plugin --profile web list --depth 0
dsh --profile web --dump-config | Select-String "dsh-hacker-news|hacker-news"
```

Observed with DSH `0.1.0-rc.6`: running `dsh plugin --profile web install` kept a linked dependency but removed `dsh-hacker-news` from `dsh.profile.bundles`. If that happens, run the relevant `add -w` command again before restarting DSH.

## Test

```powershell
cd D:/myspace/github/OPEN/hn-cli/plugins/hacker-news
npm install --no-package-lock
npm test
```

The second half of `npm test` requires internet access because it calls the public Hacker News APIs.

Repository-wide checks used for the current tree:

```powershell
go test ./...
claude plugin validate ./ --strict
claude plugin validate ./plugins/hn --strict
```

All of the above passed during the 0.1.0 release validation. `git diff --check` also passed.

## Restart DSH Web

For an interactive launch, stop the current DSH Web process after verifying its command line, then run:

```powershell
dsh web
```

The current hidden instance was started directly with:

```powershell
D:/install/nodejs/node.exe D:/install/nodejs/node_modules/@deepseek-ai/dsh/lib/bin.js web
```

Verify the restart:

```powershell
Invoke-WebRequest http://127.0.0.1:3080 -UseBasicParsing
dsh --profile web --dump-config | Select-String "dsh-hacker-news"
```

## Release State

- Repository migration: committed and pushed to `main`.
- npm package: `dsh-hacker-news@0.1.0` is published with the `latest` tag.
- Community listing: [PR #1622](https://github.com/awesome-dsh-plugin/awesome-dsh-plugin/pull/1622) is awaiting maintainer review.
- Documentation in the npm package is a release snapshot. Changes to `plugins/hacker-news/README.md` appear on npm only after publishing a new package version.

## Remaining Work

1. Monitor community-listing PR #1622 and address maintainer feedback if any.
2. When the npm README needs an update, bump the package version, run the package checks, and publish the new version; npm does not allow replacing the README of `0.1.0`.
