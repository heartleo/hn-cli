/** HTML-to-text conversion and model-facing rendering helpers. @module dsh-hacker-news/text */

import { itemUrl } from './client.js'

const ENTITIES = {
  amp: '&',
  lt: '<',
  gt: '>',
  quot: '"',
  apos: "'",
  nbsp: ' ',
  hellip: '…',
  mdash: '—',
  ndash: '–',
}

/** Decode the numeric and named entities HN emits in comment bodies. */
export function decodeEntities(input) {
  return input.replace(/&(#x?[0-9a-fA-F]+|[a-zA-Z]+);/g, (match, body) => {
    if (body[0] === '#') {
      const code = body[1] === 'x' || body[1] === 'X'
        ? Number.parseInt(body.slice(2), 16)
        : Number.parseInt(body.slice(1), 10)
      return Number.isFinite(code) ? String.fromCodePoint(code) : match
    }
    const named = ENTITIES[body.toLowerCase()]
    return named === undefined ? match : named
  })
}

/**
 * Convert an HN HTML fragment to plain text: paragraphs become blank lines,
 * links keep their label followed by the target, remaining tags are dropped.
 */
export function htmlToText(html) {
  if (!html) return ''
  const withBreaks = html
    .replace(/<\s*br\s*\/?\s*>/gi, '\n')
    .replace(/<\s*\/?\s*p\s*>/gi, '\n\n')
    .replace(/<\s*a\b[^>]*href="([^"]*)"[^>]*>(.*?)<\s*\/\s*a\s*>/gis, (_match, href, label) => {
      const text = label.replace(/<[^>]+>/g, '').trim()
      const target = decodeEntities(href)
      return text === '' || text === target ? target : `${text} (${target})`
    })
  return decodeEntities(withBreaks.replace(/<[^>]+>/g, ''))
    .replace(/[ \t]+\n/g, '\n')
    .replace(/\n{3,}/g, '\n\n')
    .trim()
}

/** Clip long text on a whitespace boundary and mark the cut. */
export function clip(text, maxLength) {
  if (text.length <= maxLength) return text
  const head = text.slice(0, maxLength)
  const boundary = head.lastIndexOf(' ')
  return `${(boundary > maxLength * 0.6 ? head.slice(0, boundary) : head).trimEnd()}… [truncated]`
}

/** Format a Unix second timestamp as an age relative to `now`. */
export function relativeAge(time, now = Date.now()) {
  if (!time) return 'unknown time'
  const seconds = Math.max(0, Math.floor(now / 1000) - time)
  const units = [
    ['d', 86400],
    ['h', 3600],
    ['m', 60],
  ]
  for (const [suffix, size] of units) {
    if (seconds >= size) return `${Math.floor(seconds / size)}${suffix} ago`
  }
  return `${seconds}s ago`
}

/** One-line story summary used by the feed and search renderers. */
export function renderStoryLine(story, index) {
  const parts = [
    `${index}. ${story.title ?? '(untitled)'}`,
    `   ${story.score ?? 0} points by ${story.by ?? 'unknown'} | ${story.comments ?? 0} comments | ${relativeAge(story.time)} | id=${story.id}`,
  ]
  if (story.url) parts.push(`   ${story.url}`)
  parts.push(`   ${story.hnUrl ?? itemUrl(story.id)}`)
  return parts.join('\n')
}

/** Indented comment block used by the item renderer. */
export function renderCommentBlock(comment) {
  const indent = '  '.repeat(comment.depth)
  const header = `${indent}- ${comment.by ?? 'unknown'} | ${relativeAge(comment.time)} | id=${comment.id}`
  const body = comment.text
    .split('\n')
    .map((line) => (line === '' ? '' : `${indent}  ${line}`))
    .join('\n')
  return body === '' ? header : `${header}\n${body}`
}
