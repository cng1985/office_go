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
  const payload = await response.json()
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
