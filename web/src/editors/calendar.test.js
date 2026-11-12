import assert from 'node:assert/strict'
import test from 'node:test'
import { coversDay, dateKey, eventsOn, monthWeeks, parseKey, placeTimed, startOfWeek, timeKey, toISO } from './calendar.js'

test('month grid starts on Monday and keeps six weeks', () => {
  const weeks = monthWeeks(2026, 10)
  assert.equal(weeks.length, 6)
  assert.equal(weeks[0].length, 7)
  assert.equal(weeks[0][0].key, '2026-10-26')
  assert.equal(weeks[0][0].inMonth, false)
  assert.equal(weeks[0][6].key, '2026-11-01')
  assert.equal(weeks[0][6].inMonth, true)
  assert.equal(weeks[5][6].key, '2026-12-06')
  assert.equal(dateKey(startOfWeek(parseKey('2026-11-12'))), '2026-11-09')
  assert.equal(dateKey(startOfWeek(parseKey('2026-11-15'))), '2026-11-09')
})

test('all-day and timed events land on the calendar days they cover', () => {
  const allDay = { id: '1', all_day: true, start_on: '2026-11-12', end_on: '2026-11-13', starts_at: '2026-11-12T00:00:00Z' }
  const timed = {
    id: '2',
    all_day: false,
    starts_at: new Date(2026, 10, 12, 10, 0).toISOString(),
    ends_at: new Date(2026, 10, 12, 11, 0).toISOString(),
  }
  const overnight = {
    id: '3',
    all_day: false,
    starts_at: new Date(2026, 10, 12, 23, 0).toISOString(),
    ends_at: new Date(2026, 10, 13, 1, 0).toISOString(),
  }
  assert.equal(coversDay(allDay, '2026-11-12'), true)
  assert.equal(coversDay(allDay, '2026-11-13'), true)
  assert.equal(coversDay(allDay, '2026-11-14'), false)
  assert.equal(coversDay(timed, '2026-11-12'), true)
  assert.equal(coversDay(timed, '2026-11-13'), false)
  assert.equal(coversDay(overnight, '2026-11-12'), true)
  assert.equal(coversDay(overnight, '2026-11-13'), true)
  const ordered = eventsOn([timed, allDay], '2026-11-12').map((event) => event.id)
  assert.deepEqual(ordered, ['1', '2'])
  const back = new Date(toISO('2026-11-12', '10:30'))
  assert.equal(dateKey(back), '2026-11-12')
  assert.equal(timeKey(back), '10:30')
})

test('overlapping timed events share a column group', () => {
  const placed = placeTimed([
    { id: 'a', start: 60, end: 120 },
    { id: 'b', start: 90, end: 150 },
    { id: 'c', start: 200, end: 260 },
  ])
  const byId = Object.fromEntries(placed.map((item) => [item.id, item]))
  assert.equal(byId.a.cols, 2)
  assert.equal(byId.b.cols, 2)
  assert.notEqual(byId.a.col, byId.b.col)
  assert.equal(byId.c.cols, 1)
  assert.equal(byId.c.col, 0)
})
