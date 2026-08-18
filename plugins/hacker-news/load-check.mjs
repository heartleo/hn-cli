import assert from 'node:assert/strict'
import { validateJsonSchemaValue } from '@deepseek-ai/dsh-tools'
import { apply, Config, inject, name } from './index.js'

/** Run the registry's output-schema validation the way dispatch would. */
function checkOutput(tool, value) {
  const violations = validateJsonSchemaValue(tool.output.schema, JSON.parse(JSON.stringify(value)), '')
  if (violations.length > 0) throw new Error(`${tool.name} output violates schema: ${violations.join('; ')}`)
  return value
}

const registered = []
const promptSections = []
const ctx = {
  tools: { register: (tool) => registered.push(tool) },
  systemPrompt: { section: (section) => promptSections.push(section) },
}

assert.equal(
  Config({}).userAgent,
  'dsh-hacker-news/0.1.0 (+https://github.com/heartleo/hn-cli)',
  'default user agent must identify the migrated repository',
)

const config = Config(process.env.HN_PLUGIN_CONFIG ? JSON.parse(process.env.HN_PLUGIN_CONFIG) : {})

apply(ctx, config)

assert.deepEqual(
  registered.map((tool) => tool.name),
  ['hn_stories', 'hn_item', 'hn_search', 'hn_user'],
  'plugin must register exactly the documented tools',
)
assert.equal(promptSections[0]?.name, 'tool:hacker-news')

console.log('plugin:', name, 'inject:', inject)
console.log('config:', config)
console.log('tools:', registered.map((tool) => tool.name))

const signal = new AbortController().signal
const tool = (toolName) => registered.find((entry) => entry.name === toolName)

const storiesTool = tool('hn_stories')
const storiesArgs = { feed: 'ask', limit: 3 }
const stories = checkOutput(storiesTool, await storiesTool.execute(storiesArgs, { signal }))
console.log(storiesTool.output.render(storiesArgs, stories)[0].text)
console.log('call card:', storiesTool.presentCall(storiesArgs))

const itemTool = tool('hn_item')
const itemArgs = { id: stories.stories[0].id, comment_limit: 5, max_depth: 2 }
const item = checkOutput(itemTool, await itemTool.execute(itemArgs, { signal }))
console.log('item comments:', item.comments.length, 'truncated:', item.truncated)
console.log(itemTool.output.render(itemArgs, item)[0].text.split('\n').slice(0, 6).join('\n'))

const searchTool = tool('hn_search')
const searchArgs = { query: 'sqlite', tags: 'story', sort: 'date', limit: 2 }
const search = checkOutput(searchTool, await searchTool.execute(searchArgs, { signal }))
console.log('search hits:', search.hits.map((hit) => hit.title))

const userTool = tool('hn_user')
const user = checkOutput(userTool, await userTool.execute({ id: 'dang' }, { signal }))
console.log('user:', user.id, user.karma, user.found)
const missing = checkOutput(userTool, await userTool.execute({ id: 'no-such-user-xyz-0' }, { signal }))
console.log('missing user found:', missing.found)

for (const [label, run] of [
  ['bad feed', () => storiesTool.execute({ feed: 'nope' }, { signal })],
  ['missing id', () => itemTool.execute({}, { signal })],
  ['empty query', () => searchTool.execute({ query: '  ' }, { signal })],
  ['negative limit', () => storiesTool.execute({ limit: -1 }, { signal })],
  ['unknown item', () => itemTool.execute({ id: 999999999 }, { signal })],
]) {
  let rejection
  try {
    await run()
  } catch (error) {
    rejection = error
  }
  assert.ok(rejection, `${label} should have failed`)
  console.log(`${label} rejected:`, rejection.message.split('\n')[0].slice(0, 90))
}
