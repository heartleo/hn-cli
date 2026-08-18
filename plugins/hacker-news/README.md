# hacker-news — DeepSeek Harness plugin

Hacker News tools for [DeepSeek Harness](https://deepseek-harness.github.io/deepseek-harness/): browse the ranked feeds, read a thread with its reply tree, search the Algolia index, and look up a user.

Plain ESM JavaScript, no build step and no runtime dependency on the `hn` binary — it talks to the public HN APIs directly (`https://hacker-news.firebaseio.com/v0` and `https://hn.algolia.com/api/v1`).

## Tools

| Tool | Arguments | Returns |
|------|-----------|---------|
| `hn_stories` | `feed` (`top`/`new`/`best`/`ask`/`show`/`job`), `limit`, `offset` | Ranked stories with id, score, comment count, URLs |
| `hn_item` | `id` (required), `comment_limit`, `max_depth` | The item plus its reply tree in reading order, with `depth` and `truncated` |
| `hn_search` | `query` (required), `sort` (`relevance`/`date`), `tags`, `limit` | Algolia hits normalised to the story shape |
| `hn_user` | `id` (required) | `found`, karma, account age, bio, submission count |

Comment HTML is converted to plain text (links keep their label plus target) and clipped at `maxTextLength`. Reply loading is breadth-first, so a small `comment_limit` returns the top of the thread rather than one deep branch; `truncated` reports whether anything was left out.

## Use

Ask naturally; the plugin guides the agent to the matching tool:

```text
What's popular on Hacker News right now?
Search Hacker News for SQLite WASM.
Read HN item 49322107 and summarize the discussion.
Look up the HN user dang.
```

## Install

```sh
dsh plugin --profile <name> add -w github:heartleo/hn-cli#path:/plugins/hacker-news
```

The package ships runnable JavaScript, so no `prepare` build and no pnpm `allowBuilds` entry is needed. A local checkout works too:

```sh
dsh plugin --profile <name> add -w /absolute/path/to/hn-cli/plugins/hacker-news
dsh --profile <name> --dump-config   # shows the "# == dsh-hacker-news" layer
```

To load it from source without installing, point a patch overlay at the entry file:

```yaml
- insert:
    - id: hacker-news
      name: '/absolute/path/to/hn-cli/plugins/hacker-news/index.js'
```

```sh
pnpm dsh web --patch ./scratch/hacker-news.cordis.yml
```

## Configuration

Every tunable is a config field, overridable per profile in `cordis.patch.yml`:

```yaml
- id: hacker-news
  name: dsh-hacker-news
  config:
    defaultStoryLimit: 30
    defaultCommentLimit: 100
    requestTimeoutMs: 20000
```

| Field | Default | Meaning |
|-------|---------|---------|
| `firebaseBaseUrl` | `https://hacker-news.firebaseio.com/v0` | Official HN API root |
| `algoliaBaseUrl` | `https://hn.algolia.com/api/v1` | Search API root |
| `requestTimeoutMs` | `15000` | Per-request timeout |
| `concurrency` | `20` | Maximum in-flight item fetches |
| `defaultStoryLimit` / `maxStoryLimit` | `20` / `100` | Story and search result counts |
| `defaultCommentLimit` / `maxCommentLimit` | `50` / `400` | Replies loaded per `hn_item` call |
| `defaultCommentDepth` / `maxCommentDepth` | `5` / `10` | Reply nesting followed |
| `maxTextLength` | `2000` | Clip length for story, comment, and bio text |
| `userAgent` | `dsh-hacker-news/0.1.0 …` | Sent on every request |

## Checking it locally

`npm test` first runs the offline routing and presentation tests, then `load-check.mjs` registers the tools against a stub context, runs every tool against the live API, validates each result with the registry's own output-schema validator, and exercises the rejection paths:

```sh
cd plugins/hacker-news
npm install --no-package-lock
npm test
```

The final integration phase needs network access.
