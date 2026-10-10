const api = require('../../../services/api')
const auth = require('../../../services/auth')

Page({
  data: { list: [] },
  onShow() {
    auth.ensureLogin().then(() => api.get('/me/affairs/favorites')).then((d) => {
      this.setData({ list: d.list || [] })
    })
  },
  go(e) {
    wx.navigateTo({ url: '/pages/affairs/detail?id=' + e.currentTarget.dataset.id })
  }
})