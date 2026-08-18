/** Hacker News HTTP client: Firebase item/feed reads plus Algolia search. @module dsh-hacker-news/client */

/** @typedef {import('./config.js').HackerNewsConfig} HackerNewsConfig */

/**
 * @typedef {object} HnItem
 * @property {number} id
 * @property {string} [type]
 * @property {string} [by]
 * @property {number} [time]
 * @property {string} [title]
 * @property {string} [url]
 * @property {string} [text]
 * @property {number} [score]
 * @property {number} [descendants]
 * @property {number} [parent]
 * @property {number[]} [kids]
 * @property {boolean} [deleted]
 * @property {boolean} [dead]
 */

/** Feed name accepted by the tools, mapped to its Firebase endpoint. */
export const FEED_ENDPOINTS = {
  top: 'topstories',
  new: 'newstories',
  best: 'beststories',
  ask: 'askstories',
  show: 'showstories',
  job: 'jobstories',
}

/** Feed names in the order the tool description lists them. */
export const FEED_NAMES = Object.keys(FEED_ENDPOINTS)

/** Canonical web URL of an item's discussion page. */
export function itemUrl(id) {
  return `https://news.ycombinator.com/item?id=${id}`
}

/** Canonical web URL of a user profile. */
export function userUrl(id) {
  return `https://news.ycombinator.com/user?id=${encodeURIComponent(id)}`
}

/**
 * Combine a caller signal with the configured per-request timeout. The caller
 * signal stays authoritative: aborting it aborts the request either way.
 */
function requestSignal(signal, timeoutMs) {
  const timeout = AbortSignal.timeout(timeoutMs)
  return signal ? AbortSignal.any([signal, timeout]) : timeout
}

/** Run `task` over `inputs` with at most `limit` in flight, preserving order. */
export async function mapConcurrent(inputs, limit, task) {
  const results = new Array(inputs.length)
  let next = 0
  const workers = new Array(Math.max(1, Math.min(limit, inputs.length))).fill(null).map(async () => {
    for (;;) {
      const index = next++
      if (index >= inputs.length) return
      results[index] = await task(inputs[index], index)
    }
  })
  await Promise.all(workers)
  return results
}

/** Read-only Hacker News client over the public Firebase and Algolia APIs. */
export class HnClient {
  /** @param {HackerNewsConfig} config */
  constructor(config) {
    this.config = config
  }

  /**
   * Fetch one JSON document. Network, HTTP-status, and parse failures throw,
   * because they are infrastructure failures rather than domain outcomes.
   */
  async fetchJson(url, signal) {
    const response = await fetch(url, {
      signal: requestSignal(signal, this.config.requestTimeoutMs),
      headers: { accept: 'application/json', 'user-agent': this.config.userAgent },
    })
    if (!response.ok) {
      throw new Error(`Hacker News request failed: ${response.status} ${response.statusText} (${url})`)
    }
    return response.json()
  }

  /** Fetch one item; missing ids resolve to `null` on this API. */
  async item(id, signal) {
    return /** @type {Promise<HnItem | null>} */ (
      this.fetchJson(`${this.config.firebaseBaseUrl}/item/${id}.json`, signal)
    )
  }

  /** Fetch the ranked id list of one feed. */
  async feedIds(feed, signal) {
    const endpoint = FEED_ENDPOINTS[feed]
    const ids = await this.fetchJson(`${this.config.firebaseBaseUrl}/${endpoint}.json`, signal)
    return Array.isArray(ids) ? ids : []
  }

  /** Fetch a user profile; unknown users resolve to `null`. */
  async user(id, signal) {
    return this.fetchJson(`${this.config.firebaseBaseUrl}/user/${encodeURIComponent(id)}.json`, signal)
  }

  /** Fetch many items concurrently, dropping ids the API no longer serves. */
  async items(ids, signal) {
    const fetched = await mapConcurrent(ids, this.config.concurrency, (id) => this.item(id, signal))
    return /** @type {HnItem[]} */ (fetched.filter((item) => item !== null && item !== undefined))
  }

  /**
   * Search through Algolia. `sort: 'date'` uses the chronological endpoint;
   * `tags` is passed through verbatim (`story`, `comment`, `author_pg`, ...).
   */
  async search({ query, tags, sort, limit }, signal) {
    const endpoint = sort === 'date' ? 'search_by_date' : 'search'
    const url = new URL(`${this.config.algoliaBaseUrl}/${endpoint}`)
    url.searchParams.set('query', query)
    url.searchParams.set('hitsPerPage', String(limit))
    if (tags) url.searchParams.set('tags', tags)
    const payload = await this.fetchJson(url.toString(), signal)
    return Array.isArray(payload?.hits) ? payload.hits : []
  }

  /**
   * Walk an item's replies breadth-first so each level is fetched with the
   * configured concurrency, then return them in depth-first reading order.
   *
   * Truncation is breadth-first: with a small `maxComments` the result is the
   * top of the thread rather than one deep branch.
   *
   * @returns {Promise<{ comments: Array<HnItem & { depth: number }>, truncated: boolean }>}
   */
  async commentTree(root, { maxComments, maxDepth }, signal) {
    /** @type {Map<number, Array<HnItem & { depth: number }>>} */
    const childrenOf = new Map()
    // Replies of a skipped node attach to its nearest surviving ancestor.
    /** @type {Map<number, number>} */
    const liftedTo = new Map()
    const anchorFor = (id) => {
      let anchor = id
      while (liftedTo.has(anchor)) anchor = liftedTo.get(anchor)
      return anchor
    }
    let frontier = (root?.kids ?? []).map((id) => ({ id, depth: 0 }))
    let collected = 0
    let truncated = false

    while (frontier.length > 0 && collected < maxComments) {
      const room = maxComments - collected
      const batch = frontier.slice(0, room)
      if (batch.length < frontier.length) truncated = true

      const fetched = await mapConcurrent(batch, this.config.concurrency, (entry) => this.item(entry.id, signal))
      /** @type {Array<{ id: number, depth: number }>} */
      const nextFrontier = []

      for (const [index, item] of fetched.entries()) {
        if (!item) continue
        const depth = batch[index].depth
        const parent = anchorFor(item.parent ?? root.id)
        // Dead and deleted nodes stay out of the result but keep their subtree
        // reachable: HN keeps live replies under removed parents.
        if (item.deleted || item.dead) {
          liftedTo.set(item.id, parent)
        } else {
          const siblings = childrenOf.get(parent) ?? []
          siblings.push({ ...item, depth })
          childrenOf.set(parent, siblings)
          collected++
        }
        const kids = item.kids ?? []
        if (kids.length === 0) continue
        if (depth + 1 > maxDepth) {
          truncated = true
          continue
        }
        for (const kid of kids) nextFrontier.push({ id: kid, depth: depth + 1 })
      }

      frontier = nextFrontier
    }

    if (frontier.length > 0) truncated = true

    /** @type {Array<HnItem & { depth: number }>} */
    const ordered = []
    const visit = (parentId) => {
      for (const child of childrenOf.get(parentId) ?? []) {
        ordered.push(child)
        visit(child.id)
      }
    }
    visit(root.id)

    return { comments: ordered, truncated }
  }
}
