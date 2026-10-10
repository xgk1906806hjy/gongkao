const api = require('../../services/api')
const auth = require('../../services/auth')

function fmt(t) {
  if (!t) return '待定'
  return String(t).replace('T', ' ').slice(0, 16)
}

Page({
  data: { id: 0, item: null, timeline: [] },
  onLoad(q) {
    this.setData({ id: Number(q.id || 0) })
    this.load()
  },
  load() {
    api.get('/announcements/' + this.data.id, false).then((item) => {
      item.published_at = fmt(item.published_at)
      const timeline = [
        { label: '报名开始', value: fmt(item.register_start) },
        { label: '报名结束', value: fmt(item.register_end) },
        { label: '确认开始', value: fmt(item.confirm_start) },
        { label: '确认结束', value: fmt(item.confirm_end) },
        { label: '考试时间', value: fmt(item.exam_at) }
      ]
      this.setData({ item, timeline })
    })
  },
  join() {
    auth.ensureLogin().then(() => {
      return api.post('/me/exams', { announcement_id: this.data.id })
    }).then(() => {
      wx.showToast({ title: '已加入' })
      return auth.requestSubscribeTodo()
    }).catch((e) => {
      wx.showToast({ title: e.message || '失败', icon: 'none' })
    })
  }
})