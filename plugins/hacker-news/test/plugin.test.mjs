import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

import { apply, Config, inject } from '../index.js'

const packageJson = JSON.parse(
  await readFile(new URL('../package.json', import.meta.url), 'utf8'),
)

function createContext() {
  const promptSections = []
  const tools = []
  const ctx = {
    tools: { register: (tool) => tools.push(tool) },
    systemPrompt: { section: (section) => promptSections.push(section) },
  }
  return { ctx, promptSections, tools }
}

test('plugin guides the model to select every Hacker News tool from natural language', () => {
  const harness = createContext()

  apply(harness.ctx, Config({}))

  assert.deepEqual(inject, ['tools', 'systemPrompt'])
  assert.equal(harness.promptSections.length, 1)
  assert.equal(harness.promptSections[0].name, 'tool:hacker-news')
  assert.match(harness.promptSections[0].text, /hn_stories/)
  assert.match(harness.promptSections[0].text, /hn_item/)
  assert.match(harness.promptSections[0].text, /hn_search/)
  assert.match(harness.promptSections[0].text, /hn_user/)
})

test('tool calls use readable Hacker News card titles', () => {
  const harness = createContext()
  apply(harness.ctx, Config({}))
  const tool = (name) => harness.tools.find((entry) => entry.name === name)

  assert.equal(tool('hn_stories').presentCall({ feed: 'new' }).title, 'Hacker News · New')
  assert.equal(tool('hn_item').presentCall({ id: 42 }).title, 'Hacker News · Item 42')
  assert.equal(tool('hn_search').presentCall({ query: 'sqlite' }).title, 'Hacker News · Search: sqlite')
  assert.equal(tool('hn_user').presentCall({ id: 'dang' }).title, 'Hacker News · User: dang')
})

test('plugin declares a DSH bundle and prerelease-compatible peer dependencies', () => {
  assert.equal(packageJson.dsh.bundle.patch, './cordis.patch.yml')
  assert.equal(packageJson.peerDependencies['@deepseek-ai/cordis'], '>=4.0.1 <5.0.0')
  assert.equal(packageJson.peerDependencies['@deepseek-ai/schemastery'], '>=3.18.1 <4.0.0')
  assert.equal(
    packageJson.peerDependencies['@deepseek-ai/dsh-system-prompt'],
    '>=0.1.0-rc.1 <0.2.0-0',
  )
  assert.equal(
    packageJson.peerDependencies['@deepseek-ai/dsh-tools'],
    '>=0.1.0-rc.1 <0.2.0-0',
  )
})
