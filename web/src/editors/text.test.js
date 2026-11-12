import assert from 'node:assert/strict'
import test from 'node:test'
import {
  applyMark,
  applyStyle,
  clampFindIndex,
  clipboardToLines,
  collapseBackward,
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

test('backspace before a paragraph removes the image and keeps the text', () => {
  const blocks = [
    { id: 'a', type: 'image', align: 'left', image: 'data:image/jpeg;base64,aaaa', runs: [{ text: '' }] },
    { id: 'b', type: 'paragraph', align: 'left', runs: [{ text: 'Hello' }] },
  ]
  const result = collapseBackward(blocks, 1)
  assert.equal(result.index, 0)
  assert.equal(result.offset, 0)
  assert.equal(blocks.length, 1)
  assert.equal(blocks[0].type, 'paragraph')
  assert.equal(plainText(blocks[0].runs), 'Hello')
})

test('page break backspace keeps the following paragraph', () => {
  const blocks = [
    { id: 'a', type: 'paragraph', align: 'left', runs: [{ text: '上文' }] },
    { id: 'b', type: 'break', align: 'left', runs: [{ text: '' }] },
    { id: 'c', type: 'paragraph', align: 'left', runs: [{ text: '还在' }] },
  ]
  const result = collapseBackward(blocks, 2)
  assert.equal(result.index, 1)
  assert.equal(blocks.length, 2)
  assert.equal(plainText(blocks[1].runs), '还在')
  assert.equal(blocks.some((block) => block.type === 'break'), false)
})

test('a sans-serif stack is not stored as a serif font', () => {
  const sans = elementNode('DIV', [
    elementNode('SPAN', [textNode('黑体')], { fontFamily: '"PingFang SC", "Noto Sans SC", "Microsoft YaHei", sans-serif' }),
  ])
  const serif = elementNode('DIV', [
    elementNode('SPAN', [textNode('宋体')], { fontFamily: '"Songti SC", "STSong", serif' }),
  ])
  assert.equal(segmentsFromElement(sans)[0][0].font, 'sans')
  assert.equal(segmentsFromElement(serif)[0][0].font, 'serif')
})

test('find index stays inside the remaining matches', () => {
  assert.equal(clampFindIndex(2, 2), 1)
  assert.equal(clampFindIndex(-1, 3), 0)
  assert.equal(clampFindIndex(0, 0), -1)
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
