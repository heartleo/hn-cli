/** Model-facing Hacker News tools. @module dsh-hacker-news/tools */

import { defineTool } from '@deepseek-ai/dsh-tools'
import { FEED_NAMES, itemUrl, userUrl } from './client.js'
import { clip, htmlToText, relativeAge, renderCommentBlock, renderStoryLine } from './text.js'

const nullableString = { oneOf: [{ type: 'string' }, { type: 'null' }] }

const storySchema = {
  type: 'object',
  additionalProperties: false,
  properties: {
    id: { type: 'integer' },
    type: { type: 'string' },
    title: { type: 'string' },
    url: nullableString,
    hnUrl: { type: 'string' },
    by: { type: 'string' },
    score: { type: 'integer' },
    comments: { type: 'integer' },
    time: { type: 'integer' },
    text: { type: 'string' },
  },
}

/** Clamp a model-supplied count into `[1, max]`, falling back to `fallback`. */
function boundedCount(value, fallback, max) {
  if (value === undefined) return fallback
  if (!Number.isInteger(value) || value < 1) {
    throw new Error(`Expected a positive integer, received ${JSON.stringify(value)}.`)
  }
  return Math.min(value, max)
}

/** Project a raw item into the canonical story shape returned by the tools. */
function toStory(item, maxTextLength) {
  return {
    id: item.id,
    type: item.type ?? 'story',
    title: item.title ?? '(untitled)',
    url: item.url ?? null,
    hnUrl: itemUrl(item.id),
    by: item.by ?? 'unknown',
    score: item.score ?? 0,
    comments: item.descendants ?? 0,
    time: item.time ?? 0,
    text: clip(htmlToText(item.text ?? ''), maxTextLength),
  }
}

/**
 * Register every Hacker News tool on `ctx`. Registration is effect-based, so
 * unloading the plugin unregisters the tools.
 *
 * @param {import('@deepseek-ai/cordis').Context} ctx
 * @param {import('./client.js').HnClient} client
 * @param {import('./config.js').HackerNewsConfig} config
 */
export function registerTools(ctx, client, config) {
  ctx.tools.register(defineTool({
    name: 'hn_stories',
    description: [
      'List Hacker News stories from one ranked feed.',
      `Feeds: ${FEED_NAMES.join(', ')}. Returns ids usable with hn_item.`,
    ].join(' '),
    parameters: {
      feed: { type: 'string', enum: FEED_NAMES, description: 'Which ranked feed to read. Defaults to top.' },
      limit: { type: 'integer', description: `How many stories to return (max ${config.maxStoryLimit}).` },
      offset: { type: 'integer', description: 'How many ranked entries to skip, for paging. Defaults to 0.' },
    },
    output: {
      schema: {
        type: 'object',
        additionalProperties: false,
        properties: {
          feed: { type: 'string' },
          offset: { type: 'integer' },
          total: { type: 'integer' },
          stories: { type: 'array', items: storySchema },
        },
      },
      render: (_args, value) => [{
        type: 'text',
        text: value.stories.length === 0
          ? `No stories in the ${value.feed} feed at offset ${value.offset}.`
          : [
              `Hacker News ${value.feed} stories ${value.offset + 1}-${value.offset + value.stories.length} of ${value.total}:`,
              ...value.stories.map((story, index) => renderStoryLine(story, value.offset + index + 1)),
            ].join('\n'),
      }],
    },
    presentCall: (args) => {
      const feed = args.feed ?? 'top'
      return { card: 'generic', title: `Hacker News · ${feed[0].toUpperCase()}${feed.slice(1)}`, kind: 'search', rawInput: args }
    },
    async execute(args, exec) {
      const feed = args.feed ?? 'top'
      const limit = boundedCount(args.limit, config.defaultStoryLimit, config.maxStoryLimit)
      const offset = args.offset === undefined ? 0 : args.offset
      if (!Number.isInteger(offset) || offset < 0) {
        throw new Error(`offset must be a non-negative integer, received ${JSON.stringify(args.offset)}.`)
      }
      const ids = await client.feedIds(feed, exec.signal)
      const items = await client.items(ids.slice(offset, offset + limit), exec.signal)
      return {
        feed,
        offset,
        total: ids.length,
        stories: items.map((item) => toStory(item, config.maxTextLength)),
      }
    },
  }))

  ctx.tools.register(defineTool({
    name: 'hn_item',
    description: [
      'Read one Hacker News story or comment together with its reply tree.',
      'Replies are collected breadth-first, so a small comment_limit returns the top of the thread.',
    ].join(' '),
    parameters: {
      id: { type: 'integer', required: true, description: 'Hacker News item id.' },
      comment_limit: { type: 'integer', description: `How many replies to load (max ${config.maxCommentLimit}).` },
      max_depth: { type: 'integer', description: `How deep to follow replies (max ${config.maxCommentDepth}).` },
    },
    output: {
      schema: {
        type: 'object',
        additionalProperties: false,
        properties: {
          item: storySchema,
          truncated: { type: 'boolean' },
          comments: {
            type: 'array',
            items: {
              type: 'object',
              additionalProperties: false,
              properties: {
                id: { type: 'integer' },
                by: { type: 'string' },
                time: { type: 'integer' },
                depth: { type: 'integer' },
                parent: { type: 'integer' },
                hnUrl: { type: 'string' },
                text: { type: 'string' },
              },
            },
          },
        },
      },
      render: (_args, value) => [{
        type: 'text',
        text: [
          `${value.item.title} (${value.item.type})`,
          `${value.item.score} points by ${value.item.by} | ${value.item.comments} comments | ${relativeAge(value.item.time)}`,
          value.item.url ? value.item.url : null,
          value.item.hnUrl,
          value.item.text ? `\n${value.item.text}` : null,
          value.comments.length === 0 ? '\nNo replies loaded.' : `\nReplies (${value.comments.length}${value.truncated ? ', truncated' : ''}):`,
          ...value.comments.map(renderCommentBlock),
        ].filter((line) => line !== null).join('\n'),
      }],
    },
    presentCall: (args) => ({ card: 'generic', title: `Hacker News · Item ${args.id}`, kind: 'read', rawInput: args }),
    async execute(args, exec) {
      const commentLimit = boundedCount(args.comment_limit, config.defaultCommentLimit, config.maxCommentLimit)
      const maxDepth = boundedCount(args.max_depth, config.defaultCommentDepth, config.maxCommentDepth)
      const item = await client.item(args.id, exec.signal)
      if (!item) throw new Error(`Hacker News item ${args.id} does not exist.`)
      const { comments, truncated } = await client.commentTree(
        item,
        { maxComments: commentLimit, maxDepth },
        exec.signal,
      )
      return {
        item: toStory(item, config.maxTextLength),
        truncated,
        comments: comments.map((comment) => ({
          id: comment.id,
          by: comment.by ?? 'unknown',
          time: comment.time ?? 0,
          depth: comment.depth,
          parent: comment.parent ?? item.id,
          hnUrl: itemUrl(comment.id),
          text: clip(htmlToText(comment.text ?? ''), config.maxTextLength),
        })),
      }
    },
  }))

  ctx.tools.register(defineTool({
    name: 'hn_search',
    description: [
      'Search Hacker News through the Algolia index.',
      'Use tags to restrict the result kind, for example story, comment, ask_hn, show_hn, or author_pg.',
    ].join(' '),
    parameters: {
      query: { type: 'string', required: true, description: 'Full-text query.' },
      sort: { type: 'string', enum: ['relevance', 'date'], description: 'Ranking. Defaults to relevance.' },
      tags: { type: 'string', description: 'Algolia tag filter, for example "story" or "story,author_pg".' },
      limit: { type: 'integer', description: `How many hits to return (max ${config.maxStoryLimit}).` },
    },
    output: {
      schema: {
        type: 'object',
        additionalProperties: false,
        properties: {
          query: { type: 'string' },
          sort: { type: 'string' },
          tags: nullableString,
          hits: {
            type: 'array',
            items: {
              type: 'object',
              additionalProperties: false,
              properties: {
                id: { type: 'integer' },
                title: { type: 'string' },
                url: nullableString,
                hnUrl: { type: 'string' },
                by: { type: 'string' },
                score: { type: 'integer' },
                comments: { type: 'integer' },
                time: { type: 'integer' },
                text: { type: 'string' },
              },
            },
          },
        },
      },
      render: (_args, value) => [{
        type: 'text',
        text: value.hits.length === 0
          ? `No Hacker News results for ${JSON.stringify(value.query)}.`
          : [
              `Hacker News results for ${JSON.stringify(value.query)} (sorted by ${value.sort}):`,
              ...value.hits.map((hit, index) => renderStoryLine(hit, index + 1)),
            ].join('\n'),
      }],
    },
    presentCall: (args) => ({ card: 'generic', title: `Hacker News · Search: ${args.query}`, kind: 'search', rawInput: args }),
    async execute(args, exec) {
      const query = args.query.trim()
      if (query === '') throw new Error('query must not be empty.')
      const sort = args.sort ?? 'relevance'
      const limit = boundedCount(args.limit, config.defaultStoryLimit, config.maxStoryLimit)
      const hits = await client.search({ query, sort, tags: args.tags, limit }, exec.signal)
      return {
        query,
        sort,
        tags: args.tags ?? null,
        hits: hits.map((hit) => {
          const id = Number(hit.objectID)
          return {
            id,
            title: hit.title ?? hit.story_title ?? '(comment)',
            url: hit.url ?? hit.story_url ?? null,
            hnUrl: itemUrl(id),
            by: hit.author ?? 'unknown',
            score: hit.points ?? 0,
            comments: hit.num_comments ?? 0,
            time: hit.created_at_i ?? 0,
            text: clip(htmlToText(hit.comment_text ?? hit.story_text ?? ''), config.maxTextLength),
          }
        }),
      }
    },
  }))

  ctx.tools.register(defineTool({
    name: 'hn_user',
    description: 'Read a Hacker News user profile: karma, account age, bio, and submission count.',
    parameters: {
      id: { type: 'string', required: true, description: 'Hacker News username, case sensitive.' },
    },
    output: {
      schema: {
        type: 'object',
        additionalProperties: false,
        properties: {
          id: { type: 'string' },
          found: { type: 'boolean' },
          karma: { type: 'integer' },
          created: { type: 'integer' },
          submitted: { type: 'integer' },
          about: { type: 'string' },
          hnUrl: { type: 'string' },
        },
      },
      render: (_args, value) => [{
        type: 'text',
        text: value.found
          ? [
              `${value.id} | ${value.karma} karma | joined ${relativeAge(value.created)} | ${value.submitted} submissions`,
              value.hnUrl,
              value.about ? `\n${value.about}` : null,
            ].filter((line) => line !== null).join('\n')
          : `No Hacker News user named ${value.id}.`,
      }],
    },
    presentCall: (args) => ({ card: 'generic', title: `Hacker News · User: ${args.id}`, kind: 'read', rawInput: args }),
    async execute(args, exec) {
      const id = args.id.trim()
      if (id === '') throw new Error('id must not be empty.')
      const user = await client.user(id, exec.signal)
      return {
        id,
        found: Boolean(user),
        karma: user?.karma ?? 0,
        created: user?.created ?? 0,
        submitted: user?.submitted?.length ?? 0,
        about: clip(htmlToText(user?.about ?? ''), config.maxTextLength),
        hnUrl: userUrl(id),
      }
    },
  }))
}
