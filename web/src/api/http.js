export class ApiError extends Error {
  constructor(code, message) {
    super(message || '请求失败')
    this.code = code
  }
}

export async function post(url, body) {
  const response = await fetch(url, {
    method: 'POST',
    credentials: 'same-origin',
    headers: {
      'Content-Type': 'application/json',
      'X-Requested-With': 'yunjian',
    },
    body: JSON.stringify(body ?? {}),
  })
  return readPayload(await response.json())
}

export function postWithProgress(url, body, onProgress, signal) {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    const fail = (error) => {
      signal?.removeEventListener('abort', abort)
      reject(error)
    }
    const abort = () => xhr.abort()
    if (signal?.aborted) {
      reject(new ApiError(0, '已取消上传'))
      return
    }
    xhr.open('POST', url)
    xhr.withCredentials = true
    xhr.setRequestHeader('Content-Type', 'application/json')
    xhr.setRequestHeader('X-Requested-With', 'yunjian')
    xhr.upload.onprogress = (event) => {
      if (!onProgress) return
      onProgress({
        loaded: event.loaded,
        total: event.lengthComputable ? event.total : 0,
      })
    }
    xhr.onerror = () => fail(new ApiError(0, '网络异常'))
    xhr.onabort = () => fail(new ApiError(0, '已取消上传'))
    xhr.onload = () => {
      signal?.removeEventListener('abort', abort)
      let payload
      try {
        payload = JSON.parse(xhr.responseText)
      } catch {
        reject(new ApiError(xhr.status || 500, '响应无效'))
        return
      }
      try {
        resolve(readPayload(payload))
      } catch (error) {
        reject(error)
      }
    }
    signal?.addEventListener('abort', abort)
    xhr.send(JSON.stringify(body ?? {}))
  })
}

function readPayload(payload) {
  if (payload && typeof payload.code === 'number' && payload.code !== 200) {
    throw new ApiError(payload.code, payload.msg)
  }
  return payload
}

export async function postPage(url, body) {
  const payload = await post(url, body)
  if (!payload || !Array.isArray(payload.data)) {
    throw new ApiError(500, '分页响应无效')
  }
  return payload
}
