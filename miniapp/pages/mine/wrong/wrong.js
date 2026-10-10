const api = require('../../../services/api')
const auth = require('../../../services/auth')

Page({
  data: { list: [] },
  onShow() {
    auth.ensureLogin().then(() => api.get('/me/wrong-questions')).then((d) => {
      this.setData({ list: d.list || [] })
    })
  },
  remove(e) {
    api.del('/me/wrong-questions/' + e.currentTarget.dataset.id).then(() => this.onShow())
  },
  rePractice() {
    auth.ensureLogin().then(() => api.post('/practice/sessions', {
      mode: 'wrong',
      count: 20,
      show_analysis: true
    })).then((sess) => {
      wx.setStorageSync('practice_session', sess)
      wx.navigateTo({ url: '/pages/practice/quiz/quiz' })
    }).catch((e) => {
      wx.showToast({ title: e.message || '无法开始', icon: 'none' })
    })
  }
})
