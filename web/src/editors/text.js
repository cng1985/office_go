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

export function mergeRuns(runs) {
  const out = []
  for (const run of runs) {
    if (!run.text) continue
    const prev = out[out.length - 1]
    if (prev && prev.bold === run.bold && prev.italic === run.italic && prev.underline === run.underline) {
      prev.text += run.text
    } else {
      out.push({ ...run })
    }
  }
  return out.length ? out : [{ text: '', bold: false, italic: false, underline: false }]
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

function marksOf(node, marks) {
  const style = node.style || {}
  const decoration = style.textDecoration || ''
  return {
    bold: marks.bold || style.fontWeight === '700' || style.fontWeight === 'bold' || node.tagName === 'B' || node.tagName === 'STRONG',
    italic: marks.italic || style.fontStyle === 'italic' || node.tagName === 'I' || node.tagName === 'EM',
    underline: marks.underline || decoration.includes('underline') || node.tagName === 'U',
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
  for (const child of element.childNodes) walk(child, { bold: false, italic: false, underline: false })
  if (runs.length || !segments.length) flush()
  return segments
}

export function newId() {
  if (globalThis.crypto?.randomUUID) return crypto.randomUUID().replace(/-/g, '').slice(0, 16)
  return Math.random().toString(36).slice(2, 12)
}
