import assert from 'node:assert/strict'
import test from 'node:test'
import { applySaveSnapshot } from './save.js'

test('save adopts the server document when nothing changed during the request', () => {
  const submitted = { title: '纪要', content: { blocks: [{ id: 'b1', runs: [{ text: '甲' }] }] } }
  const server = { title: '纪要', content: { blocks: [{ id: 'b1', runs: [{ text: '甲' }] }] } }
  const next = applySaveSnapshot(submitted, submitted, server)
  assert.equal(next.pendingEdits, false)
  assert.equal(next.title, '纪要')
  assert.deepEqual(next.content, server.content)
})

test('save keeps keystrokes made after the request started', () => {
  const submitted = { title: '纪要', content: { text: '甲' } }
  const current = { title: '纪要终稿', content: { text: '甲乙' } }
  const server = { title: '纪要', content: { text: '甲' }, revision: 2 }
  const next = applySaveSnapshot(current, submitted, server)
  assert.equal(next.pendingEdits, true)
  assert.equal(next.title, '纪要终稿')
  assert.deepEqual(next.content, current.content)
  assert.deepEqual(next.savedContent, submitted.content)
  assert.equal(next.savedTitle, '纪要')
})
