const api = require('../../services/api')
const auth = require('../../services/auth')

Page({
  data: { categories: [], stats: null, accuracy: '-' },
  onShow() {
    api.get('/questions/categories', false).then((d) => {
      this.setData({ categories: d.list || [] })
    })
    auth.ensureLogin().then(() => api.get('/me/practice/stats')).then((stats) => {
      const accuracy = stats.total_answered ? Math.round(stats.accuracy * 100) + '%' : '-'
      this.setData({ stats, accuracy })
    }).catch(() => {})
  },
  start(e) {
    const categoryId = Number(e.currentTarget.dataset.id)
    const mode = e.currentTarget.dataset.mode
    auth.ensureLogin().then(() => {
      return api.post('/practice/sessions', {
        category_id: categoryId, mode, count: 10, show_analysis: true
      })
    }).then((data) => {
      wx.setStorageSync('practice_session', data)
      wx.navigateTo({ url: '/pages/practice/quiz/quiz' })
    }).catch((err) => {
      wx.showToast({ title: err.message || '开始失败', icon: 'none' })
    })
  }
})