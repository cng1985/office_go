// mergeVersionPage 按请求页码合并历史版本。重复项来自其它标签新增版本后的偏移，不代表历史已经读完。
export function mergeVersionPage(existing, pageItems, total, page, pageSize) {
  const seen = new Set(existing.map((item) => item.id))
  const fresh = (pageItems || []).filter((item) => !seen.has(item.id))
  const versions = existing.concat(fresh)
  const reachedEnd = !pageItems || pageItems.length === 0 || page * pageSize >= total
  return { versions, total, page, done: reachedEnd }
}
