export function formatBytes(value) {
  const n = Math.max(0, Number(value) || 0)
  if (n < 1024) return `${Math.round(n)} B`
  const units = ['KB', 'MB', 'GB']
  let size = n / 1024
  let unit = 0
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024
    unit += 1
  }
  const text = (size >= 100 ? size.toFixed(0) : size.toFixed(1)).replace(/\.0$/, '')
  return `${text} ${units[unit]}`
}

const encodeShare = 12

export function uploadPercent(phase, ratio) {
  const clamped = Math.max(0, Math.min(1, Number(ratio) || 0))
  if (phase === 'done') return 100
  if (phase === 'read') return 0
  if (phase === 'encode') return Math.round(clamped * encodeShare)
  if (phase === 'upload') return Math.min(99, encodeShare + Math.round(clamped * (99 - encodeShare)))
  if (phase === 'import') return 99
  return 0
}

export function cancelError() {
  const error = new Error('已取消上传')
  error.name = 'AbortError'
  return error
}

export function isUploadCancel(error) {
  return error?.name === 'AbortError' || error?.message === '已取消上传'
}

export async function encodeBase64(bytes, onProgress, signal) {
  const chunk = 0x8000
  let binary = ''
  for (let index = 0; index < bytes.length; index += chunk) {
    if (signal?.aborted) throw cancelError()
    binary += String.fromCharCode(...bytes.subarray(index, index + chunk))
    const done = Math.min(index + chunk, bytes.length)
    if (onProgress && done < bytes.length && (index / chunk) % 8 === 0) {
      onProgress(done)
      await new Promise((resolve) => setTimeout(resolve, 0))
      if (signal?.aborted) throw cancelError()
    }
  }
  if (signal?.aborted) throw cancelError()
  if (onProgress) onProgress(bytes.length)
  return btoa(binary)
}
