export function plainText(runs) {
  return (runs || []).map((run) => run.text || '').join('')
}

export function sliceRuns(runs, start, end) {
  const out = []
  let cursor = 0
  for (const run of runs || []) {
    const runEnd = cursor + run.text.length
    const from = Math.max(start, cursor)
    const to = Math.min(end, runEnd)
    if (from < to) out.push({ ...run, text: run.text.slice(from - cursor, to - cursor) })
    cursor = runEnd
  }
  return out
}

export function styleOf(run = {}) {
  const size = Number(run.size) || 0
  return {
    bold: !!run.bold,
    italic: !!run.italic,
    underline: !!run.underline,
    strike: !!run.strike,
    color: String(run.color || '').toLowerCase(),
    highlight: String(run.highlight || '').toLowerCase(),
    size: size >= 8 && size <= 72 ? size : 0,
    font: ['serif', 'sans', 'mono'].includes(run.font) ? run.font : '',
  }
}

export function sameStyle(a, b) {
  const left = styleOf(a)
  const right = styleOf(b)
  return Object.keys(left).every((key) => left[key] === right[key])
}

export function compactRun(run) {
  const style = styleOf(run)
  const out = {
    text: run.text || '',
    bold: style.bold,
    italic: style.italic,
    underline: style.underline,
  }
  if (style.strike) out.strike = true
  if (style.color) out.color = style.color
  if (style.highlight) out.highlight = style.highlight
  if (style.size) out.size = style.size
  if (style.font) out.font = style.font
  return out
}

export function mergeRuns(runs) {
  const out = []
  for (const run of runs) {
    if (!run.text) continue
    const prev = out[out.length - 1]
    if (prev && sameStyle(prev, run)) {
      prev.text += run.text
    } else {
      out.push(compactRun(run))
    }
  }
  return out.length ? out : [{ text: '', bold: false, italic: false, underline: false }]
}

export function applyStyle(runs, start, end, patch) {
  const text = plainText(runs)
  const safeStart = Math.max(0, Math.min(start, text.length))
  const safeEnd = Math.max(safeStart, Math.min(end, text.length))
  if (safeStart === safeEnd) return mergeRuns(runs)
  return mergeRuns([
    ...sliceRuns(runs, 0, safeStart),
    ...sliceRuns(runs, safeStart, safeEnd).map((run) => ({ ...run, ...patch })),
    ...sliceRuns(runs, safeEnd, text.length),
  ])
}

export function clearRuns(runs) {
  return mergeRuns((runs || []).map((run) => ({ text: run.text })))
}

export function textDiff(before, after) {
  const previous = String(before)
  const next = String(after)
  let start = 0
  const limit = Math.min(previous.length, next.length)
  while (start < limit && previous[start] === next[start]) start += 1
  let previousEnd = previous.length
  let nextEnd = next.length
  while (previousEnd > start && nextEnd > start && previous[previousEnd - 1] === next[nextEnd - 1]) {
    previousEnd -= 1
    nextEnd -= 1
  }
  return { start, removed: previousEnd - start, insert: nextEnd - start }
}

export function replaceOnce(runs, start, end, replacement) {
  const text = plainText(runs)
  const base = sliceRuns(runs, start, Math.max(start, end))[0] || { text: '' }
  return mergeRuns([
    ...sliceRuns(runs, 0, start),
    { ...base, text: replacement },
    ...sliceRuns(runs, end, text.length),
  ])
}

export function findHits(blocks, query) {
  const needle = String(query || '')
  if (!needle) return []
  const key = needle.toLowerCase()
  const hits = []
  blocks.forEach((block, blockIndex) => {
    const hay = plainText(block.runs).toLowerCase()
    let from = 0
    while (from < hay.length) {
      const at = hay.indexOf(key, from)
      if (at < 0) break
      hits.push({ block: blockIndex, start: at, end: at + needle.length })
      from = at + Math.max(needle.length, 1)
    }
  })
  return hits
}

const BULLETS = ['•', '◦', '▪']

export function listMarker(blocks, index) {
  const block = blocks[index]
  if (!block || (block.type !== 'bullet' && block.type !== 'numbered')) return ''
  const level = block.indent || 0
  if (block.type === 'bullet') return BULLETS[Math.min(level, BULLETS.length - 1)]
  let count = 0
  for (let cursor = index; cursor >= 0; cursor -= 1) {
    const item = blocks[cursor]
    if (item.type !== 'numbered') break
    const indent = item.indent || 0
    if (indent < level) break
    if (indent === level) count += 1
  }
  return formatListNumber(count, level)
}

function formatListNumber(count, level) {
  const cycle = level % 3
  if (cycle === 1) return `${alpha(count)}.`
  if (cycle === 2) return `${roman(count)}.`
  return `${count}.`
}

function alpha(count) {
  let value = count
  let out = ''
  while (value > 0) {
    value -= 1
    out = String.fromCharCode(97 + (value % 26)) + out
    value = Math.floor(value / 26)
  }
  return out
}

function roman(count) {
  const pairs = [[10, 'x'], [9, 'ix'], [5, 'v'], [4, 'iv'], [1, 'i']]
  let value = count
  let out = ''
  pairs.forEach(([step, glyph]) => {
    while (value >= step) {
      out += glyph
      value -= step
    }
  })
  return out
}

export function applyMark(runs, start, end, mark) {
  const text = plainText(runs)
  const safeStart = Math.max(0, Math.min(start, text.length))
  const safeEnd = Math.max(safeStart, Math.min(end, text.length))
  if (safeStart === safeEnd) {
    const enabled = !(runs || []).every((run) => run[mark] || !run.text)
    return mergeRuns((runs || []).map((run) => ({ ...run, [mark]: enabled && run.text !== '' ? true : run.text ? !enabled && false : run[mark] })))
  }
  const selected = sliceRuns(runs, safeStart, safeEnd)
  const enabled = !selected.every((run) => run[mark])
  return mergeRuns([
    ...sliceRuns(runs, 0, safeStart),
    ...selected.map((run) => ({ ...run, [mark]: enabled })),
    ...sliceRuns(runs, safeEnd, text.length),
  ])
}

export function clipboardToLines(plain, html) {
  const fromPlain = plain != null && plain !== ''
  const source = fromPlain ? String(plain) : htmlToPlain(html)
  const lines = source.replace(/\r\n/g, '\n').replace(/\r/g, '\n').split('\n')
  if (!fromPlain && lines.length > 1 && lines[lines.length - 1] === '') lines.pop()
  return lines
}

function htmlToPlain(html) {
  return String(html || '')
    .replace(/<br\s*\/?>/gi, '\n')
    .replace(/<\/(div|p|li|h1|h2|h3|tr|blockquote)>/gi, '\n')
    .replace(/<[^>]+>/g, '')
    .replace(/&nbsp;/g, ' ')
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&amp;/g, '&')
}

const TEXT_NODE = 3
const ELEMENT_NODE = 1
const BLOCK_TAGS = new Set(['DIV', 'P', 'LI', 'H1', 'H2', 'H3', 'BLOCKQUOTE', 'UL', 'OL', 'TR'])

function cssColorToHex(value) {
  const text = String(value || '').trim()
  if (/^#[0-9a-fA-F]{6}$/.test(text)) return text.toLowerCase()
  const matched = /^rgba?\((\d+),\s*(\d+),\s*(\d+)/.exec(text)
  if (!matched) return ''
  return `#${matched.slice(1, 4).map((part) => Number(part).toString(16).padStart(2, '0')).join('')}`
}

function fontKey(family) {
  const value = String(family || '').toLowerCase()
  if (!value) return ''
  if (value.includes('mono') || value.includes('courier') || value.includes('consolas')) return 'mono'
  if (value.includes('songti') || value.includes('palatino') || value.includes('serif')) return 'serif'
  if (value.includes('sans') || value.includes('avenir') || value.includes('pingfang')) return 'sans'
  return ''
}

function marksOf(node, marks) {
  const style = node.style || {}
  const decoration = `${style.textDecoration || ''} ${style.textDecorationLine || ''}`
  const size = Math.round(parseFloat(style.fontSize) || 0)
  return {
    bold: marks.bold || style.fontWeight === '700' || style.fontWeight === 'bold' || node.tagName === 'B' || node.tagName === 'STRONG',
    italic: marks.italic || style.fontStyle === 'italic' || node.tagName === 'I' || node.tagName === 'EM',
    underline: marks.underline || decoration.includes('underline') || node.tagName === 'U',
    strike: marks.strike || decoration.includes('line-through') || node.tagName === 'S' || node.tagName === 'DEL',
    color: cssColorToHex(style.color) || marks.color || '',
    highlight: cssColorToHex(style.backgroundColor) || marks.highlight || '',
    size: (size >= 8 && size <= 72 ? size : 0) || marks.size || 0,
    font: fontKey(style.fontFamily) || marks.font || '',
  }
}

function isTrailingBreak(node) {
  let sibling = node.nextSibling
  while (sibling) {
    if (sibling.nodeType === TEXT_NODE && !String(sibling.nodeValue || '').replace(/\u00a0/g, ' ').trim()) {
      sibling = sibling.nextSibling
      continue
    }
    return false
  }
  return true
}

// segmentsFromElement 把 contenteditable 里的换行和块级标签收成段落。
// 浏览器放在行尾占位的单个 br 不会多拆出空段。
export function segmentsFromElement(element) {
  const segments = []
  let runs = []
  const flush = () => {
    segments.push(mergeRuns(runs))
    runs = []
  }
  const pushText = (value, marks) => {
    const text = String(value || '').replace(/\u00a0/g, ' ').replace(/\r?\n/g, '')
    if (text) runs.push({ text, ...marks })
  }
  const walk = (node, marks) => {
    if (node.nodeType === TEXT_NODE) {
      pushText(node.nodeValue, marks)
      return
    }
    if (node.nodeType !== ELEMENT_NODE) return
    if (node.tagName === 'BR') {
      if (!isTrailingBreak(node) || runs.length || segments.length) flush()
      return
    }
    if (BLOCK_TAGS.has(node.tagName)) {
      if (runs.length) flush()
      const start = segments.length
      const next = marksOf(node, marks)
      for (const child of node.childNodes) walk(child, next)
      if (runs.length || segments.length === start) flush()
      return
    }
    const next = marksOf(node, marks)
    for (const child of node.childNodes) walk(child, next)
  }
  for (const child of element.childNodes) walk(child, styleOf({}))
  if (runs.length || !segments.length) flush()
  return segments
}

export function newId() {
  if (globalThis.crypto?.randomUUID) return crypto.randomUUID().replace(/-/g, '').slice(0, 16)
  return Math.random().toString(36).slice(2, 12)
}
