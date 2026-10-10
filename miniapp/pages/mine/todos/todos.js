const api = require('../../../services/api')
const auth = require('../../../services/auth')

function fmt(t) {
  return String(t || '').replace('T', ' ').slice(0, 16)
}

Page({
  data: { groups: [] },
  onShow() {
    auth.ensureLogin().then(() => api.get('/me/todos')).then((d) => {
      const groups = (d.list || []).map((g) => ({
        ...g,
        ann_title: (g.announcement && g.announcement.title) || ('公告#' + g.announcement_id),
        todos: (g.todos || []).map((t) => ({
          ...t,
          due_at: fmt(t.due_at),
          remind_at: fmt(t.remind_at)
        }))
      }))
      this.setData({ groups })
      auth.requestSubscribeTodo()
    })
  },
  done(e) {
    api.patch('/me/todos/' + e.currentTarget.dataset.id, { status: 'done' }).then(() => {
      wx.showToast({ title: '已完成' })
      this.onShow()
    })
  }
})