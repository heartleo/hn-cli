# hn

Hacker News for Claude.

## Install

```
/plugin marketplace add heartleo/hn-cli
/plugin install hn@hn-cli
```

## Skills

| Skill | What it does |
| --- | --- |
| [`hn-digest`](skills/hn-digest) | Scan the front page — today's themes, hot discussions, industry mix |

More to come. Skills live inside this plugin, so installing `hn` once is enough — new skills arrive with a plugin update, not a new install.

## hn-digest

Not a browser. It answers one question in one screenful: **what is HN talking about right now, and what industries is it coming from.**

Just ask:

```
what's on HN today
top HN today
scan HN
```

Output is three fixed sections:

1. **Overview** — prose. Read only this and you know what happened today.
2. **Hot discussions** — top 10 by score, every one clickable (article + comments).
3. **Industry mix** — a monospace bar chart of the 30 front-page stories.

Ask in any language and the digest comes back in that language.

### How it works

One request:

```bash
curl -s "https://hn.algolia.com/api/v1/search?tags=front_page&hitsPerPage=30&attributesToRetrieve=title,points,num_comments,url,objectID,created_at"
```

That's the whole data layer. Algolia's `front_page` tag returns all 30 front-page stories with scores and comment counts in a single response (~17KB), so there is no parsing code, no `jq`, no MCP server, and no dependency to install. The JSON goes straight to Claude, which does what it is already good at: summarizing.

Firebase's `topstories.json` would need 1 + 30 requests for the same data.

### What it does not do

- **Read comments.** The skill never fetches a comment tree. Its only signals are title, score, and comment count — and it is [held to saying so](skills/hn-digest/SKILL.md#honesty-discipline) rather than inventing what commenters said. Mining a thread for its actual arguments is a different job, and a candidate for a future skill.
- **Filter by interest tags or score floors.** The front page is only 30 stories; filter it and there's nothing left to generalize from.
- **Scrape `hckrnews.com`.** It's JS-rendered — `curl` returns an empty shell.

### Notes

`tags=front_page` returns **the front page as it looks right now**, not "stories posted today." Long-running stories stay up for days; a story from two months ago has been observed still on the front page. This is intentional — the question is "what's on HN," not "what was posted today." For a strict 24-hour window use `tags=story&numericFilters=created_at_i>{now-86400}` instead, still one request.

Algolia's per-comment `points` field is always `null`, so "which comment scored highest" is not an available signal. Comment count over score is the only controversy proxy there is.

## Related

This plugin ships alongside [**hn**](https://github.com/heartleo/hn-cli), a terminal client for Hacker News. The two are independent — the plugin needs no binary installed, and the CLI needs no Claude.

## License

MIT.
