import { post } from './http'

export function register(body) {
  return post('/api/account/register', body)
}

export function login(body) {
  return post('/api/account/login', body)
}

export function logout() {
  return post('/api/account/logout', {})
}

export function me() {
  return post('/api/account/me', {})
}
