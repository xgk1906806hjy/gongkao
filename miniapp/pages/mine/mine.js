const auth = require('../../services/auth')

Page({
  data: { user: {} },
  onShow() {
    this.setData({ user: wx.getStorageSync('user') || {} })
  },
  login() {
    auth.ensureLogin().then((user) => {
      this.setData({ user: user || wx.getStorageSync('user') || {} })
      wx.showToast({ title: '登录成功' })
    }).catch(() => wx.showToast({ title: '登录失败', icon: 'none' }))
  },
  go(e) {
    auth.ensureLogin().then(() => {
      wx.navigateTo({ url: e.currentTarget.dataset.url })
    }).catch(() => wx.showToast({ title: '请先登录', icon: 'none' }))
  },
  subscribe() {
    auth.requestSubscribeTodo().then(() => {
      wx.showToast({ title: '已发起订阅', icon: 'none' })
    })
  }
})