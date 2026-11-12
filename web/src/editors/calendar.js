export const EVENT_COLORS = ['#1a73e8', '#188038', '#f9ab00', '#d93025', '#a142f4', '#12b5cb', '#e37400', '#5f6368']

export function eventColor(color) {
  return EVENT_COLORS.includes(color) ? color : EVENT_COLORS[0]
}

export function inkOn(color) {
  return color === '#f9ab00' ? '#202124' : '#fff'
}

export function dateKey(date) {
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${date.getFullYear()}-${month}-${day}`
}

export function parseKey(key) {
  const [year, month, day] = String(key).split('-').map(Number)
  return new Date(year, month - 1, day)
}

export function addDays(date, days) {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate() + days)
}

export function startOfWeek(date) {
  const weekday = date.getDay()
  const delta = weekday === 0 ? -6 : 1 - weekday
  return addDays(date, delta)
}

export function monthWeeks(year, monthIndex) {
  const start = startOfWeek(new Date(year, monthIndex, 1))
  const weeks = []
  for (let week = 0; week < 6; week += 1) {
    const days = []
    for (let offset = 0; offset < 7; offset += 1) {
      const date = addDays(start, week * 7 + offset)
      days.push({ key: dateKey(date), date, inMonth: date.getMonth() === monthIndex })
    }
    weeks.push(days)
  }
  return weeks
}

export function timeKey(date) {
  return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
}

export function clockMinutes(date) {
  return date.getHours() * 60 + date.getMinutes() + date.getSeconds() / 60 + date.getMilliseconds() / 60000
}

export function minutesLabel(total) {
  const minutes = ((Math.floor(total) % 1440) + 1440) % 1440
  return `${String(Math.floor(minutes / 60)).padStart(2, '0')}:${String(minutes % 60).padStart(2, '0')}`
}

export function toISO(day, time) {
  const date = parseKey(day)
  const [hours, minutes] = String(time || '00:00').split(':').map(Number)
  date.setHours(hours || 0, minutes || 0, 0, 0)
  return date.toISOString()
}

export function coversDay(event, key) {
  if (!event) return false
  if (event.all_day) return Boolean(event.start_on && event.end_on && event.start_on <= key && event.end_on >= key)
  const start = new Date(event.starts_at)
  const end = new Date(event.ends_at)
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) return false
  const day = parseKey(key)
  return start < addDays(day, 1) && end > day
}

export function eventsOn(events, key) {
  return (events || []).filter((event) => coversDay(event, key)).sort((a, b) => {
    if (a.all_day !== b.all_day) return a.all_day ? -1 : 1
    return String(a.starts_at).localeCompare(String(b.starts_at)) || String(a.id).localeCompare(String(b.id))
  })
}

export function timedBlocks(events, key) {
  const day = parseKey(key)
  const next = addDays(day, 1)
  const items = []
  for (const event of events || []) {
    if (!event || event.all_day) continue
    const start = new Date(event.starts_at)
    const end = new Date(event.ends_at)
    if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime()) || start >= next || end <= day) continue
    items.push({
      event,
      start: start <= day ? 0 : clockMinutes(start),
      end: end >= next ? 1440 : clockMinutes(end),
    })
  }
  return placeTimed(items)
}

export function placeTimed(events) {
  const items = (events || [])
    .map((event) => ({ ...event, start: Number(event.start), end: Number(event.end) }))
    .filter((event) => event.end > event.start)
    .sort((a, b) => a.start - b.start || a.end - b.end || String(a.event?.id).localeCompare(String(b.event?.id)))
  const placed = []
  let cluster = []
  let clusterEnd = -1
  const flush = () => {
    if (!cluster.length) return
    const columns = []
    for (const event of cluster) {
      let column = columns.findIndex((until) => until <= event.start)
      if (column < 0) {
        column = columns.length
        columns.push(event.end)
      } else {
        columns[column] = event.end
      }
      event.col = column
    }
    for (const event of cluster) event.cols = columns.length
    placed.push(...cluster)
    cluster = []
    clusterEnd = -1
  }
  for (const event of items) {
    if (cluster.length && event.start >= clusterEnd) flush()
    cluster.push(event)
    clusterEnd = Math.max(clusterEnd, event.end)
  }
  flush()
  return placed
}
