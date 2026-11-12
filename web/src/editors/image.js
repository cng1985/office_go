const maxBytes = 240 * 1024

// fileToJpeg 把本地图片画到白底上，并压成可以保存的 JPEG。
export function fileToJpeg(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onerror = () => reject(new Error('无法读取这张图片'))
    reader.onload = () => {
      const img = new Image()
      img.onload = () => {
        const naturalWidth = img.naturalWidth
        const naturalHeight = img.naturalHeight
        if (!naturalWidth || !naturalHeight) {
          reject(new Error('无法读取这张图片'))
          return
        }
        const canvas = document.createElement('canvas')
        const ctx = canvas.getContext('2d')
        let width = naturalWidth
        let height = naturalHeight
        const edge = Math.min(1, 1600 / Math.max(width, height))
        width = Math.max(1, Math.round(width * edge))
        height = Math.max(1, Math.round(height * edge))
        let quality = 0.82
        let data = ''
        for (let step = 0; step < 8; step += 1) {
          canvas.width = width
          canvas.height = height
          ctx.fillStyle = '#ffffff'
          ctx.fillRect(0, 0, width, height)
          ctx.drawImage(img, 0, 0, width, height)
          data = canvas.toDataURL('image/jpeg', quality)
          const bytes = Math.ceil((data.length - data.indexOf(',') - 1) * 0.75)
          if (bytes <= maxBytes) {
            resolve({ url: data, width: naturalWidth, height: naturalHeight })
            return
          }
          if (quality > 0.5) quality -= 0.12
          else {
            width = Math.max(32, Math.round(width * 0.72))
            height = Math.max(32, Math.round(height * 0.72))
          }
        }
        reject(new Error('图片过大，无法插入'))
      }
      img.onerror = () => reject(new Error('无法读取这张图片'))
      img.src = String(reader.result || '')
    }
    reader.readAsDataURL(file)
  })
}

export function fittedSize(width, height, maxWidth, maxHeight) {
  if (!width || !height || !maxWidth || !maxHeight) return { w: 160, h: 120 }
  const fit = Math.min(maxWidth / width, maxHeight / height)
  const scale = Math.max(width, height) <= 240 ? Math.min(fit, 200 / Math.max(width, height)) : fit
  return {
    w: Math.max(24, Math.min(maxWidth, Math.round(width * scale))),
    h: Math.max(24, Math.min(maxHeight, Math.round(height * scale))),
  }
}
