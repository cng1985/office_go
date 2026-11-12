import { post } from './http'

export function searchEvents(body) {
  return post('/api/calendar/search', body)
}

export function createEvent(body) {
  return post('/api/calendar/create', body)
}

export function updateEvent(body) {
  return post('/api/calendar/update', body)
}

export function deleteEvent(body) {
  return post('/api/calendar/delete', body)
}
