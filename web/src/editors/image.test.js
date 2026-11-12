import assert from 'node:assert/strict'
import test from 'node:test'
import { fittedSize } from './image.js'

test('fitted size keeps the picture inside the slide and readable', () => {
  const large = fittedSize(4000, 3000, 480, 300)
  assert.equal(large.w, 400)
  assert.equal(large.h, 300)
  const tiny = fittedSize(16, 16, 480, 300)
  assert.equal(tiny.w, 200)
  assert.equal(tiny.h, 200)
  assert.deepEqual(fittedSize(0, 10, 480, 300), { w: 160, h: 120 })
})
