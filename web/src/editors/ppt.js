export const CANVAS_W = 960
export const CANVAS_H = 540

export function clamp(value, min, max) {
  return Math.min(max, Math.max(min, value))
}

export function clampBox(box) {
  const w = clamp(box.w, 24, CANVAS_W)
  const h = clamp(box.h, 24, CANVAS_H)
  return {
    ...box,
    w,
    h,
    x: clamp(box.x, -200, 1100),
    y: clamp(box.y, -200, 700),
  }
}

export function resizeBox(origin, handle, dx, dy) {
  let { x, y, w, h } = origin
  if (handle.includes('e')) w += dx
  if (handle.includes('s')) h += dy
  if (handle.includes('w')) {
    x += dx
    w -= dx
  }
  if (handle.includes('n')) {
    y += dy
    h -= dy
  }
  if (w < 24) {
    if (handle.includes('w')) x -= 24 - w
    w = 24
  }
  if (h < 24) {
    if (handle.includes('n')) y -= 24 - h
    h = 24
  }
  return clampBox({ ...origin, x, y, w, h })
}

export function slideLayout(layout, ids) {
  const [titleId, bodyId] = ids
  if (layout === 'blank') return { background: '#ffffff', elements: [] }
  if (layout === 'section') {
    return {
      background: '#1c1915',
      elements: [textElement(titleId, { x: 80, y: 200, w: 800, h: 120, text: '节标题', font_size: 48, color: '#fffdf8', bold: true, align: 'center' })],
    }
  }
  if (layout === 'content') {
    return {
      background: '#ffffff',
      elements: [
        textElement(titleId, { x: 64, y: 36, w: 832, h: 78, text: '点击可添加标题', font_size: 36, color: '#202124', bold: true, align: 'left' }),
        textElement(bodyId, { x: 64, y: 140, w: 832, h: 340, text: '点击可添加正文', font_size: 22, color: '#202124', bold: false, align: 'left' }),
      ],
    }
  }
  return {
    background: '#ffffff',
    elements: [
      textElement(titleId, { x: 68, y: 132, w: 824, h: 92, text: '点击可添加标题', font_size: 40, color: '#202124', bold: true, align: 'center' }),
      textElement(bodyId, { x: 68, y: 228, w: 824, h: 68, text: '点击可添加副标题', font_size: 22, color: '#5f6368', bold: false, align: 'center' }),
    ],
  }
}

export function textElement(id, extra) {
  return {
    id,
    kind: 'text',
    x: 80,
    y: 80,
    w: 280,
    h: 72,
    text: '文本',
    font_size: 24,
    color: '#1c1915',
    bold: false,
    align: 'left',
    ...extra,
  }
}

export function shapeElement(id, shape) {
  return {
    id,
    kind: 'shape',
    shape,
    x: 620,
    y: 360,
    w: 260,
    h: 130,
    text: '',
    font_size: 20,
    color: '#1c1915',
    bold: false,
    align: 'center',
    fill: '#f4e4c4',
  }
}

export function duplicateElement(element, id) {
  const copy = JSON.parse(JSON.stringify(element))
  copy.id = id
  copy.x = clamp(element.x + 24, -200, 1100)
  copy.y = clamp(element.y + 24, -200, 700)
  return copy
}

export function moveItem(list, index, delta) {
  const next = index + delta
  if (next < 0 || next >= list.length) return index
  const [item] = list.splice(index, 1)
  list.splice(next, 0, item)
  return next
}
