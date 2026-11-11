import assert from 'node:assert/strict'
import test from 'node:test'
import { applyMark, plainText } from './text.js'

test('apply bold to a middle range and keep surrounding text', () => {
  const runs = [{ text: '云笺编辑', bold: false, italic: false, underline: false }]
  const next = applyMark(runs, 1, 3, 'bold')
  assert.equal(plainText(next), '云笺编辑')
  assert.equal(next.map((run) => run.bold).join(','), 'false,true,false')
})
