import assert from 'node:assert/strict'
import test from 'node:test'
import { duplicateElement, moveItem, resizeBox, slideLayout } from './ppt.js'

test('resizing from the west edge keeps the opposite edge fixed', () => {
  const next = resizeBox({ x: 100, y: 80, w: 200, h: 120 }, 'w', 30, 0)
  assert.equal(next.x, 130)
  assert.equal(next.w, 170)
  assert.equal(next.y, 80)
})

test('layouts place a title and a subtitle or body', () => {
  const title = slideLayout('title', ['a', 'b'])
  assert.equal(title.elements.length, 2)
  assert.equal(title.elements[0].text, '点击可添加标题')
  assert.equal(title.elements[0].align, 'center')
  assert.equal(title.background, '#ffffff')
  assert.equal(title.elements[1].text, '点击可添加副标题')
  const content = slideLayout('content', ['a', 'b'])
  assert.equal(content.elements[1].text, '点击可添加正文')
  assert.equal(slideLayout('blank', ['a', 'b']).elements.length, 0)
})

test('duplicating offsets the copy and reordering moves the slide', () => {
  const copy = duplicateElement({ id: 'e1', x: 10, y: 20, w: 100, h: 40 }, 'e2')
  assert.equal(copy.id, 'e2')
  assert.equal(copy.x, 34)
  assert.equal(copy.y, 44)
  const slides = ['a', 'b', 'c']
  assert.equal(moveItem(slides, 2, -1), 1)
  assert.deepEqual(slides, ['a', 'c', 'b'])
})
