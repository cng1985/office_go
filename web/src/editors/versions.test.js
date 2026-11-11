import assert from 'node:assert/strict'
import test from 'node:test'
import { mergeVersionPage } from './versions.js'

function ids(start, count) {
  return Array.from({ length: count }, (_, index) => ({ id: String(start - index) }))
}

test('an overlapping page keeps the next page number and the server total', () => {
  const first = mergeVersionPage([], ids(120, 50), 120, 1, 50)
  const second = mergeVersionPage(first.versions, ids(71, 50), 121, 2, 50)
  assert.equal(second.versions.length, 99)
  assert.equal(second.page, 2)
  assert.equal(second.total, 121)
  assert.equal(second.done, false)

  const repeated = mergeVersionPage(second.versions, ids(71, 50), 121, 2, 50)
  assert.equal(repeated.versions.length, 99)
  assert.equal(repeated.total, 121)
  assert.equal(repeated.done, false)
  assert.equal(repeated.page + 1, 3)
})

test('the following page still reaches the oldest versions', () => {
  const loaded = mergeVersionPage(ids(120, 50), ids(71, 50), 121, 2, 50)
  const tail = mergeVersionPage(loaded.versions, ids(21, 21), 121, 3, 50)
  assert.equal(tail.versions.some((item) => item.id === '1'), true)
  assert.equal(tail.done, true)
})
