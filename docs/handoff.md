# Handoff

Updated: 2026-08-18 (Asia/Shanghai)

## Objective

Migrate the DeepSeek Harness Hacker News plugin from `../hn` into this repository, make it easy to invoke from natural language, and keep local/CI validation repeatable.

## Current Status

- The plugin lives in `plugins/hacker-news` as an independent ESM package.
- The local DSH `web` profile links to this checkout as `dsh-hacker-news`.
- DSH Web was restarted after the latest changes and was verified at `http://127.0.0.1:3080` with HTTP 200.
- At the last check, DSH Web was PID `29108`. Treat the PID as transient; re-query it before stopping or restarting the process.
- The implementation is still uncommitted in the working tree. The earlier migration design document was committed separately as `513d7a6` and has a later uncommitted correction.

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

## Community Listing Preparation

The intended community registry is [awesome-dsh-plugin](https://github.com/awesome-dsh-plugin/awesome-dsh-plugin). Its hard requirements are met locally: the package declares `dsh.bundle`, contains runnable code and tests, uses official DSH packages as peers, and has explicit ranges that include DSH `0.1.0-rc.6`. The public `heartleo/hn-cli` repository is older than one day, has more than ten commits, and now has the `dsh-plugin` GitHub topic.

After this checkout is committed and pushed to `main`, open a PR in the registry that adds `data/plugins/heartleo__hn-cli--plugins-hacker-news.yml`:

```yaml
url: https://github.com/heartleo/hn-cli/tree/main/plugins/hacker-news
name: heartleo/hn-cli#hacker-news
category: tools
description:
  en: Hacker News tools for feeds, discussion threads, search, and user profiles.
  zh: 用于获取 Hacker News 榜单、讨论串、搜索和用户资料的工具。
```

In that registry checkout, run `npm ci` followed by `node scripts/generate-readme.mjs`, then commit the generated READMEs with the YAML file. Do not edit those READMEs manually. npm publication or a GitHub Release tarball is optional because this plugin is source-installable and requires no build step.

## Install Locally

DSH profiles are pnpm workspace roots, so `add` must receive `-w`. Local paths are resolved from the profile directory, so use an absolute path.

```powershell
dsh plugin --profile web add -w "D:/myspace/github/OPEN/hn-cli/plugins/hacker-news"
```

Remote repository form:

```powershell
dsh plugin --profile web add -w github:heartleo/hn-cli#path:/plugins/hacker-news
```

Confirm both the dependency and bundle layer:

```powershell
dsh plugin --profile web list --depth 0
dsh --profile web --dump-config | Select-String "dsh-hacker-news|hacker-news"
```

Observed with DSH `0.1.0-rc.6`: running `dsh plugin --profile web install` kept the linked dependency but removed `dsh-hacker-news` from `dsh.profile.bundles`. If that happens, run the `add -w` command again before restarting DSH.

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

All of the above passed on 2026-08-17. `git diff --check` also passed.

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

## Working Tree

Last observed status:

```text
 M .github/workflows/plugin.yml
 M .gitignore
 M README.md
 M docs/superpowers/specs/2026-08-17-dsh-plugin-migration-design.md
?? docs/handoff.md
?? plugins/hacker-news/
```

All listed changes belong to the DSH plugin migration or its documentation. Do not drop the pre-existing migration changes when preparing a commit.

## Remaining Work

1. Optionally perform a manual Web UI smoke test: ask for the top three HN stories and confirm the agent calls `hn_stories`.
2. Commit the migration and invocation changes when ready.
