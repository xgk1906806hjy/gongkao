const api = require('../../../services/api')
const auth = require('../../../services/auth')

function fmt(t) {
  return String(t || '').replace('T', ' ').slice(0, 16)
}

function countdownText(dueAt) {
  const due = new Date(dueAt).getTime()
  if (!due || Number.isNaN(due)) return ''
  const diff = due - Date.now()
  if (diff <= 0) return '已到期'
  const h = Math.floor(diff / 3600000)
  const m = Math.floor((diff % 3600000) / 60000)
  if (h >= 48) return Math.floor(h / 24) + '天后'
  if (h > 0) return h + '小时' + m + '分'
  return m + '分钟'
}

function enrichTodo(t) {
  const dueMs = new Date(t.due_at).getTime()
  const soon = t.status === 'pending' && dueMs - Date.now() <= 24 * 3600000 && dueMs >= Date.now()
  return {
    ...t,
    due_raw: t.due_at,
    due_at: fmt(t.due_at),
    remind_at: fmt(t.remind_at),
    countdown: countdownText(t.due_at),
    soon
  }
}

Page({
  data: { groups: [] },
  timer: null,
  onShow() {
    this.reload()
    this.timer = setInterval(() => this.reload(false), 60000)
  },
  onHide() {
    if (this.timer) clearInterval(this.timer)
  },
  onUnload() {
    if (this.timer) clearInterval(this.timer)
  },
  reload(askSub) {
    auth.ensureLogin().then(() => api.get('/me/todos')).then((d) => {
      const groups = (d.list || []).map((g) => ({
        ...g,
        ann_title: (g.announcement && g.announcement.title) || ('公告#' + g.announcement_id),
        todos: (g.todos || []).map(enrichTodo)
      }))
      this.setData({ groups })
      if (askSub !== false) auth.requestSubscribeTodo()
    })
  },
  done(e) {
    api.patch('/me/todos/' + e.currentTarget.dataset.id, { status: 'done' }).then(() => {
      wx.showToast({ title: '已完成' })
      this.reload(false)
    })
  },
  cancel(e) {
    api.patch('/me/todos/' + e.currentTarget.dataset.id, { status: 'cancelled' }).then(() => {
      wx.showToast({ title: '已关闭' })
      this.reload(false)
    })
  }
})
