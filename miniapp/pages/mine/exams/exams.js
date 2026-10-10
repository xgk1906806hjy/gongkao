const api = require('../../../services/api')
const auth = require('../../../services/auth')

Page({
  data: { list: [] },
  onShow() {
    auth.ensureLogin().then(() => api.get('/me/exams')).then((d) => {
      this.setData({ list: d.list || [] })
    })
  },
  leave(e) {
    const id = e.currentTarget.dataset.id
    api.del('/me/exams/' + id).then(() => {
      wx.showToast({ title: '已移除' })
      this.onShow()
    })
  }
})