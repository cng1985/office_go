<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { createEvent, deleteEvent, searchEvents, updateEvent } from '../api/calendar'
import {
  EVENT_COLORS,
  addDays,
  coversDay,
  dateKey,
  eventColor,
  eventsOn,
  inkOn,
  minutesLabel,
  monthWeeks,
  parseKey,
  placeTimed,
  startOfWeek,
  timeKey,
  toISO,
} from '../editors/calendar'
import { useSessionStore } from '../stores/session'

const DOW = ['一', '二', '三', '四', '五', '六', '日']
const WEEKDAY = ['日', '一', '二', '三', '四', '五', '六']
const WEEK_NAMES = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']
const hours = Array.from({ length: 24 }, (_, hour) => `${String(hour).padStart(2, '0')}:00`)

const session = useSessionStore()
const router = useRouter()
const todayKey = computed(() => dateKey(new Date()))
const anchorKey = ref(todayKey.value)
const miniKey = ref(todayKey.value)
const selectedKey = ref(todayKey.value)
const mode = ref('month')
const events = ref([])
const draft = ref(null)
const error = ref('')
const dialogError = ref('')
const pending = ref(false)
const confirming = ref(false)
const initial = computed(() => (session.account?.display_name || session.account?.username || '云').slice(0, 1))
let loadToken = 0

const monthGrid = computed(() => {
  const anchor = parseKey(anchorKey.value)
  return monthWeeks(anchor.getFullYear(), anchor.getMonth())
})
const miniGrid = computed(() => {
  const anchor = parseKey(miniKey.value)
  return monthWeeks(anchor.getFullYear(), anchor.getMonth())
})
const week = computed(() => {
  const start = startOfWeek(parseKey(anchorKey.value))
  return Array.from({ length: 7 }, (_, index) => {
    const date = addDays(start, index)
    return { key: dateKey(date), date }
  })
})
const heading = computed(() => {
  if (mode.value === 'week') {
    const first = week.value[0].date
    const last = week.value[6].date
    if (first.getFullYear() === last.getFullYear() && first.getMonth() === last.getMonth()) {
      return `${first.getFullYear()}年${first.getMonth() + 1}月${first.getDate()}日 – ${last.getDate()}日`
    }
    const left = `${first.getFullYear()}年${first.getMonth() + 1}月${first.getDate()}日`
    const right = `${last.getFullYear()}年${last.getMonth() + 1}月${last.getDate()}日`
    return `${left} – ${right}`
  }
  const anchor = parseKey(anchorKey.value)
  return `${anchor.getFullYear()}年${anchor.getMonth() + 1}月`
})
const miniTitle = computed(() => {
  const anchor = parseKey(miniKey.value)
  return `${anchor.getFullYear()}年${anchor.getMonth() + 1}月`
})
const agenda = computed(() => eventsOn(events.value, selectedKey.value))
const selectedLabel = computed(() => {
  const date = parseKey(selectedKey.value)
  return `${date.getMonth() + 1}月${date.getDate()}日 ${WEEK_NAMES[date.getDay()]}`
})
const nowTop = computed(() => {
  const now = new Date()
  return `${((now.getHours() * 60 + now.getMinutes()) / 1440) * 100}%`
})
const weekHasToday = computed(() => week.value.some((day) => day.key === todayKey.value))

function visibleRange() {
  if (mode.value === 'week') {
    const from = startOfWeek(parseKey(anchorKey.value))
    return { from, to: addDays(from, 7) }
  }
  const weeks = monthGrid.value
  return { from: weeks[0][0].date, to: addDays(weeks[5][6].date, 1) }
}

async function load() {
  const token = ++loadToken
  const range = visibleRange()
  try {
    const result = await searchEvents({
      from: range.from.toISOString(),
      to: range.to.toISOString(),
      start_on: dateKey(range.from),
      end_on: dateKey(addDays(range.to, -1)),
    })
    if (token !== loadToken) return
    events.value = result.data?.events ?? []
    error.value = ''
  } catch (err) {
    if (token !== loadToken) return
    error.value = err.message
  }
}

function shift(dir) {
  const date = parseKey(anchorKey.value)
  if (mode.value === 'week') {
    anchorKey.value = dateKey(addDays(date, dir * 7))
  } else {
    const day = date.getDate()
    const next = new Date(date.getFullYear(), date.getMonth() + dir, 1)
    const last = new Date(next.getFullYear(), next.getMonth() + 1, 0).getDate()
    next.setDate(Math.min(day, last))
    anchorKey.value = dateKey(next)
  }
  selectedKey.value = anchorKey.value
  miniKey.value = anchorKey.value
}

function shiftMini(dir) {
  const date = parseKey(miniKey.value)
  const next = new Date(date.getFullYear(), date.getMonth() + dir, 1)
  miniKey.value = dateKey(next)
}

function goToday() {
  const key = dateKey(new Date())
  anchorKey.value = key
  selectedKey.value = key
  miniKey.value = key
}

function select(key) {
  selectedKey.value = key
}

function pickMini(key) {
  anchorKey.value = key
  selectedKey.value = key
  miniKey.value = key
}

function blankDraft(key, options = {}) {
  return {
    id: '',
    title: '',
    allDay: options.allDay !== false,
    startOn: key,
    endOn: options.endOn || key,
    startTime: options.startTime || '09:00',
    endTime: options.endTime || '10:00',
    location: '',
    notes: '',
    color: EVENT_COLORS[0],
  }
}

function createOn(key, options = {}) {
  selectedKey.value = key
  draft.value = blankDraft(key, options)
  dialogError.value = ''
  confirming.value = false
}

function createAt(key, event) {
  const rect = event.currentTarget.getBoundingClientRect()
  const ratio = Math.min(0.999, Math.max(0, (event.clientY - rect.top) / rect.height))
  const start = Math.floor((ratio * 1440) / 30) * 30
  const end = start + 60
  createOn(key, {
    allDay: false,
    startTime: minutesLabel(start),
    endTime: minutesLabel(end),
    endOn: end >= 1440 ? dateKey(addDays(parseKey(key), 1)) : key,
  })
}

function openEvent(event) {
  const start = event.all_day ? null : new Date(event.starts_at)
  const end = event.all_day ? null : new Date(event.ends_at)
  draft.value = {
    id: String(event.id),
    title: event.title,
    allDay: Boolean(event.all_day),
    startOn: event.all_day ? event.start_on : dateKey(start),
    endOn: event.all_day ? event.end_on : dateKey(end),
    startTime: start ? timeKey(start) : '09:00',
    endTime: end ? timeKey(end) : '10:00',
    location: event.location || '',
    notes: event.notes || '',
    color: eventColor(event.color),
  }
  dialogError.value = ''
  confirming.value = false
  selectedKey.value = draft.value.startOn
}

function closeDraft() {
  if (pending.value) return
  draft.value = null
  confirming.value = false
}

function dayEvents(key) {
  return eventsOn(events.value, key)
}

function timedOn(key) {
  const origin = parseKey(key).getTime()
  const next = addDays(parseKey(key), 1).getTime()
  const items = []
  for (const event of events.value) {
    if (event.all_day) continue
    const start = new Date(event.starts_at).getTime()
    const end = new Date(event.ends_at).getTime()
    if (end <= origin || start >= next) continue
    items.push({
      event,
      start: Math.max(0, (start - origin) / 60000),
      end: Math.min(1440, (end - origin) / 60000),
    })
  }
  return placeTimed(items)
}

function allDayOn(key) {
  return events.value.filter((event) => event.all_day && coversDay(event, key))
}

function blockStyle(item) {
  const top = (item.start / 1440) * 100
  const height = (Math.min(1440 - item.start, Math.max(item.end - item.start, 24)) / 1440) * 100
  const width = 100 / item.cols
  const color = eventColor(item.event.color)
  return {
    top: `${top}%`,
    height: `${height}%`,
    left: `calc(${item.col * width}% + 2px)`,
    width: `calc(${width}% - 4px)`,
    background: color,
    color: inkOn(color),
  }
}

function chipStyle(event) {
  const color = eventColor(event.color)
  return { background: color, color: inkOn(color) }
}

function whenLabel(event) {
  if (event.all_day) {
    if (event.start_on === event.end_on) return '全天'
    return `${shortDay(event.start_on)} – ${shortDay(event.end_on)} · 全天`
  }
  const start = new Date(event.starts_at)
  const end = new Date(event.ends_at)
  if (dateKey(start) === dateKey(end)) return `${timeKey(start)} – ${timeKey(end)}`
  return `${shortDay(dateKey(start))} ${timeKey(start)} – ${shortDay(dateKey(end))} ${timeKey(end)}`
}

function shortDay(key) {
  const date = parseKey(key)
  return `${date.getMonth() + 1}月${date.getDate()}日`
}

async function save() {
  const current = draft.value
  if (!current || pending.value) return
  const title = current.title.trim()
  if (!title) {
    dialogError.value = '请填写日程标题'
    return
  }
  const body = {
    title,
    all_day: current.allDay,
    location: current.location.trim(),
    notes: current.notes.trim(),
    color: current.color,
  }
  if (current.allDay) {
    if (!current.startOn || !current.endOn || current.endOn < current.startOn) {
      dialogError.value = '结束日期不能早于开始日期'
      return
    }
    body.start_on = current.startOn
    body.end_on = current.endOn
  } else {
    const starts = new Date(toISO(current.startOn, current.startTime))
    const ends = new Date(toISO(current.endOn, current.endTime))
    if (Number.isNaN(starts.getTime()) || Number.isNaN(ends.getTime()) || ends <= starts) {
      dialogError.value = '结束时间需要晚于开始时间'
      return
    }
    body.starts_at = starts.toISOString()
    body.ends_at = ends.toISOString()
  }
  if (current.id) body.id = current.id
  pending.value = true
  dialogError.value = ''
  try {
    if (current.id) await updateEvent(body)
    else await createEvent(body)
    selectedKey.value = current.startOn
    anchorKey.value = current.startOn
    miniKey.value = current.startOn
    await load()
    draft.value = null
  } catch (err) {
    dialogError.value = err.message
  } finally {
    pending.value = false
  }
}

async function remove() {
  if (!draft.value?.id || pending.value) return
  if (!confirming.value) {
    confirming.value = true
    return
  }
  pending.value = true
  dialogError.value = ''
  try {
    await deleteEvent({ id: draft.value.id })
    await load()
    draft.value = null
    confirming.value = false
  } catch (err) {
    dialogError.value = err.message
  } finally {
    pending.value = false
  }
}

function onKey(event) {
  if (event.key === 'Escape') closeDraft()
}

async function signOut() {
  await session.logout()
  router.push('/login')
}

watch([anchorKey, mode], () => {
  load()
}, { immediate: true })

watch(mode, async (value) => {
  if (value !== 'week') return
  await nextTick()
  const node = document.querySelector('[data-testid="cal-hours"]')
  if (node) node.scrollTop = 8 * 48
})

watch(draft, async (value) => {
  if (!value) return
  await nextTick()
  document.querySelector('[data-testid="cal-field-title"]')?.focus()
})

onMounted(() => window.addEventListener('keydown', onKey))
onUnmounted(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <div class="home">
    <header class="home-bar">
      <router-link class="home-brand" to="/" data-testid="open-library">
        <span class="mark word">W</span>
        <strong>云笺</strong>
      </router-link>
      <span class="spacer"></span>
      <router-link class="home-user" to="/account" data-testid="account-link" title="个人中心">
        <i>{{ initial }}</i>
        <span data-testid="current-user">{{ session.account?.display_name }}</span>
      </router-link>
      <button class="text-button" type="button" data-testid="logout" @click="signOut">退出</button>
    </header>
    <div class="cal-page" data-testid="calendar-page">
      <aside class="cal-side">
        <button class="cal-create" type="button" data-testid="cal-create" @click="createOn(selectedKey)">创建</button>
        <div class="cal-mini-wrap">
          <div class="cal-mini-bar">
            <strong>{{ miniTitle }}</strong>
            <span>
              <button type="button" aria-label="上个月" data-testid="cal-mini-prev" @click="shiftMini(-1)">‹</button>
              <button type="button" aria-label="下个月" data-testid="cal-mini-next" @click="shiftMini(1)">›</button>
            </span>
          </div>
          <div class="cal-mini-dow"><span v-for="name in DOW" :key="name">{{ name }}</span></div>
          <div class="cal-mini">
            <button
              v-for="day in miniGrid.flat()"
              :key="day.key"
              type="button"
              :class="{ out: !day.inMonth, today: day.key === todayKey, picked: day.key === selectedKey }"
              :data-testid="`cal-mini-${day.key}`"
              @click="pickMini(day.key)"
            >
              {{ day.date.getDate() }}
              <i v-if="dayEvents(day.key).length" class="cal-dot"></i>
            </button>
          </div>
        </div>
        <section class="cal-agenda">
          <h2>{{ selectedLabel }}</h2>
          <p v-if="!agenda.length" class="muted" data-testid="cal-agenda-empty">这一天还没有日程</p>
          <button
            v-for="event in agenda"
            :key="event.id"
            type="button"
            class="cal-agenda-item"
            :data-testid="`cal-agenda-${event.id}`"
            @click="openEvent(event)"
          >
            <i :style="{ background: eventColor(event.color) }"></i>
            <span>
              <strong>{{ event.title }}</strong>
              <em>{{ whenLabel(event) }}</em>
              <em v-if="event.location">{{ event.location }}</em>
            </span>
          </button>
          <button type="button" class="cal-link" data-testid="cal-agenda-create" @click="createOn(selectedKey)">新建日程</button>
        </section>
      </aside>
      <section class="cal-main">
        <div class="cal-toolbar">
          <button type="button" class="cal-today" data-testid="cal-today" @click="goToday">今天</button>
          <div class="cal-nav">
            <button type="button" data-testid="cal-prev" aria-label="上一段" @click="shift(-1)">‹</button>
            <button type="button" data-testid="cal-next" aria-label="下一段" @click="shift(1)">›</button>
          </div>
          <h1 data-testid="cal-title">{{ heading }}</h1>
          <div class="cal-modes">
            <button type="button" data-testid="cal-mode-month" :aria-pressed="mode === 'month'" @click="mode = 'month'">月</button>
            <button type="button" data-testid="cal-mode-week" :aria-pressed="mode === 'week'" @click="mode = 'week'">周</button>
          </div>
        </div>
        <p class="error" v-if="error">{{ error }}</p>
        <div v-if="mode === 'month'" class="cal-month" data-testid="cal-month">
          <div class="cal-dow"><span v-for="name in DOW" :key="name">{{ name }}</span></div>
          <div v-for="(days, index) in monthGrid" :key="index" class="cal-week-row">
            <div
              v-for="day in days"
              :key="day.key"
              class="cal-day"
              :class="{ out: !day.inMonth, today: day.key === todayKey, picked: day.key === selectedKey }"
              :data-testid="`cal-day-${day.key}`"
              @click="select(day.key)"
              @dblclick="createOn(day.key)"
            >
              <button type="button" class="cal-num" @click.stop="select(day.key)">{{ day.date.getDate() }}</button>
              <button
                v-for="event in dayEvents(day.key).slice(0, 3)"
                :key="event.id"
                type="button"
                class="cal-chip"
                :style="chipStyle(event)"
                :title="event.title"
                :data-testid="`cal-event-${event.id}`"
                @click.stop="openEvent(event)"
                @dblclick.stop
              >{{ event.title }}</button>
              <button v-if="dayEvents(day.key).length > 3" type="button" class="cal-more" @click.stop="select(day.key)">+{{ dayEvents(day.key).length - 3 }}</button>
            </div>
          </div>
        </div>
        <div v-else class="cal-week" data-testid="cal-week">
          <div class="cal-week-head">
            <div></div>
            <div v-for="day in week" :key="day.key" class="cal-week-label">
              <span>{{ WEEKDAY[day.date.getDay()] }}</span>
              <button type="button" :class="{ today: day.key === todayKey, picked: day.key === selectedKey }" @click="select(day.key)">{{ day.date.getDate() }}</button>
            </div>
          </div>
          <div class="cal-allday">
            <div class="cal-allday-label">全天</div>
            <div v-for="day in week" :key="day.key" class="cal-allday-col" @click="createOn(day.key)">
              <button
                v-for="event in allDayOn(day.key)"
                :key="event.id"
                type="button"
                class="cal-chip"
                :style="chipStyle(event)"
                :data-testid="`cal-allday-${event.id}`"
                @click.stop="openEvent(event)"
              >{{ event.title }}</button>
            </div>
          </div>
          <div class="cal-hours" data-testid="cal-hours">
            <div class="cal-gutter"><span v-for="hour in hours" :key="hour">{{ hour }}</span></div>
            <div v-for="day in week" :key="day.key" class="cal-hour-col" :data-testid="`cal-slot-${day.key}`" @click="createAt(day.key, $event)">
              <button
                v-for="item in timedOn(day.key)"
                :key="item.event.id"
                type="button"
                class="cal-block"
                :style="blockStyle(item)"
                :data-testid="`cal-block-${item.event.id}`"
                @click.stop="openEvent(item.event)"
              >{{ item.event.title }}</button>
              <div v-if="weekHasToday && day.key === todayKey" class="cal-now" :style="{ top: nowTop }"></div>
            </div>
          </div>
        </div>
      </section>
    </div>
    <div v-if="draft" class="cal-mask" @click.self="closeDraft">
      <div class="cal-dialog" role="dialog" aria-modal="true" aria-labelledby="cal-dialog-title" data-testid="cal-dialog">
        <h2 id="cal-dialog-title">{{ draft.id ? '编辑日程' : '新建日程' }}</h2>
        <form @submit.prevent="save">
          <label>标题
            <input data-testid="cal-field-title" v-model="draft.title" maxlength="80" placeholder="添加标题" />
          </label>
          <label class="cal-check">
            <input data-testid="cal-field-all-day" type="checkbox" v-model="draft.allDay" />
            全天
          </label>
          <div class="cal-when">
            <input data-testid="cal-field-start-on" type="date" v-model="draft.startOn" required />
            <input v-if="!draft.allDay" data-testid="cal-field-start-time" type="time" v-model="draft.startTime" required />
            <span>至</span>
            <input data-testid="cal-field-end-on" type="date" v-model="draft.endOn" required />
            <input v-if="!draft.allDay" data-testid="cal-field-end-time" type="time" v-model="draft.endTime" required />
          </div>
          <label>地点
            <input data-testid="cal-field-location" v-model="draft.location" maxlength="80" placeholder="添加地点" />
          </label>
          <label>备注
            <textarea data-testid="cal-field-notes" v-model="draft.notes" maxlength="2000" rows="4" placeholder="添加备注"></textarea>
          </label>
          <div class="cal-colors" role="radiogroup" aria-label="颜色">
            <button
              v-for="color in EVENT_COLORS"
              :key="color"
              type="button"
              :style="{ background: color }"
              :aria-pressed="draft.color === color"
              :aria-label="color"
              :data-testid="`cal-color-${color.slice(1)}`"
              @click="draft.color = color"
            ></button>
          </div>
          <p class="error" v-if="dialogError">{{ dialogError }}</p>
          <div class="cal-actions">
            <button v-if="draft.id" class="danger" type="button" data-testid="cal-delete" :disabled="pending" @click="remove">{{ confirming ? '确认删除' : '删除' }}</button>
            <span class="spacer"></span>
            <button class="ghost" type="button" data-testid="cal-cancel" @click="closeDraft">取消</button>
            <button class="primary" type="submit" data-testid="cal-save" :disabled="pending">保存</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
