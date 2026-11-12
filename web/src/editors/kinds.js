export const KINDS = {
  word: { label: '文稿', create: '新建文稿', untitled: '无标题文档', mark: '文' },
  excel: { label: '表格', create: '新建表格', untitled: '未命名电子表格', mark: '表' },
  ppt: { label: '演示', create: '新建演示', untitled: '无标题演示文稿', mark: '演' },
  pdf: { label: 'PDF', create: '新建 PDF', untitled: '无标题 PDF', mark: 'P' },
}

export function formatTime(value) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return new Intl.DateTimeFormat('zh-CN', {
    month: 'numeric',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}
