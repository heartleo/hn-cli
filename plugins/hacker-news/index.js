/** DeepSeek Harness plugin exposing Hacker News as model-facing tools. @module dsh-hacker-news */

import { HnClient } from './src/client.js'
import { Config } from './src/config.js'
import { registerTools } from './src/tools.js'

export const name = 'hacker-news'

export const inject = ['tools', 'systemPrompt']

export { Config }

/**
 * @param {import('@deepseek-ai/cordis').Context} ctx
 * @param {import('./src/config.js').HackerNewsConfig} config
 */
export function apply(ctx, config) {
  ctx.systemPrompt.section({
    name: 'tool:hacker-news',
    order: 112,
    text: [
      'Use Hacker News tools for requests about Hacker News instead of general web search.',
      'Use hn_stories for ranked feeds, hn_item for a story or comment thread,',
      'hn_search for full-text search, and hn_user for user profiles.',
    ].join(' '),
  })
  registerTools(ctx, new HnClient(config), config)
}
