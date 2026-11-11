import assert from 'node:assert/strict'
import test from 'node:test'
import {
  applyMark,
  applyStyle,
  clipboardToLines,
  findHits,
  listMarker,
  plainText,
  replaceOnce,
  textDiff,
  segmentsFromElement,
} from './text.js'

test('apply bold to a middle range and keep surrounding text', () => {
  const runs = [{ text: '云笺编辑', bold: false, italic: false, underline: false }]
  const next = applyMark(runs, 1, 3, 'bold')
  assert.equal(plainText(next), '云笺编辑')
  assert.equal(next.map((run) => run.bold).join(','), 'false,true,false')
})

test('clipboard keeps line breaks from plain text and html', () => {
  assert.deepEqual(clipboardToLines('第一行\r\n第二行', ''), ['第一行', '第二行'])
  assert.deepEqual(clipboardToLines('', '<div>甲</div><div>乙<br>丙</div>'), ['甲', '乙', '丙'])
})

function textNode(value) {
  return { nodeType: 3, nodeValue: value, nextSibling: null }
}

function elementNode(tag, children, style = {}) {
  const node = { nodeType: 1, tagName: tag, style, childNodes: children, nextSibling: null }
  children.forEach((child, index) => {
    child.nextSibling = children[index + 1] || null
  })
  return node
}

test('styles, lists and find follow the edited range', () => {
  const runs = [{ text: '云笺编辑', bold: false, italic: false, underline: false }]
  const colored = applyStyle(runs, 1, 3, { color: '#112233', strike: true })
  assert.equal(plainText(colored), '云笺编辑')
  assert.equal(colored[1].color, '#112233')
  assert.equal(colored[1].strike, true)
  assert.equal(textDiff('云笺', '云笺文档').insert, 2)
  const blocks = [
    { type: 'numbered', indent: 0, runs: [{ text: '甲' }] },
    { type: 'numbered', indent: 1, runs: [{ text: '子' }] },
    { type: 'paragraph', runs: [{ text: '隔断' }] },
    { type: 'numbered', indent: 0, runs: [{ text: '乙' }] },
  ]
  assert.equal(listMarker(blocks, 0), '1.')
  assert.equal(listMarker(blocks, 1), 'a.')
  assert.equal(listMarker(blocks, 3), '1.')
  assert.deepEqual(findHits(blocks, '乙'), [{ block: 3, start: 0, end: 1 }])
  assert.equal(plainText(replaceOnce(blocks[0].runs, 0, 1, '丙')), '丙')
})

test('soft breaks and pasted blocks become separate paragraphs', () => {
  const trailing = elementNode('DIV', [textNode('今天'), elementNode('BR', [])])
  assert.deepEqual(segmentsFromElement(trailing).map((runs) => plainText(runs)), ['今天'])

  const shifted = elementNode('DIV', [textNode('今天'), elementNode('BR', []), elementNode('BR', [])])
  assert.deepEqual(segmentsFromElement(shifted).map((runs) => plainText(runs)), ['今天', ''])

  const pasted = elementNode('DIV', [
    elementNode('DIV', [textNode('第一行')]),
    elementNode('DIV', [textNode('第二行')]),
  ])
  assert.deepEqual(segmentsFromElement(pasted).map((runs) => plainText(runs)), ['第一行', '第二行'])
})
