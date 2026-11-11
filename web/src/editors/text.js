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

export function newId() {
  if (globalThis.crypto?.randomUUID) return crypto.randomUUID().replace(/-/g, '').slice(0, 16)
  return Math.random().toString(36).slice(2, 12)
}
