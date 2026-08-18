/** Plugin configuration schema. Every deployment-variable value lives here. @module dsh-hacker-news/config */

import Schema from '@deepseek-ai/schemastery'

/**
 * @typedef {object} HackerNewsConfig
 * @property {string} firebaseBaseUrl
 * @property {string} algoliaBaseUrl
 * @property {number} requestTimeoutMs
 * @property {number} concurrency
 * @property {number} defaultStoryLimit
 * @property {number} maxStoryLimit
 * @property {number} defaultCommentLimit
 * @property {number} maxCommentLimit
 * @property {number} defaultCommentDepth
 * @property {number} maxCommentDepth
 * @property {number} maxTextLength
 * @property {string} userAgent
 */

/** @type {import('@deepseek-ai/schemastery').default<HackerNewsConfig>} */
export const Config = Schema.object({
  firebaseBaseUrl: Schema.string().default('https://hacker-news.firebaseio.com/v0'),
  algoliaBaseUrl: Schema.string().default('https://hn.algolia.com/api/v1'),
  requestTimeoutMs: Schema.number().default(15000),
  concurrency: Schema.number().default(20),
  defaultStoryLimit: Schema.number().default(20),
  maxStoryLimit: Schema.number().default(100),
  defaultCommentLimit: Schema.number().default(50),
  maxCommentLimit: Schema.number().default(400),
  defaultCommentDepth: Schema.number().default(5),
  maxCommentDepth: Schema.number().default(10),
  maxTextLength: Schema.number().default(2000),
  userAgent: Schema.string().default('dsh-hacker-news/0.1.0 (+https://github.com/heartleo/hn-cli)'),
})
