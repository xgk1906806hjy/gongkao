const api = require('../../services/api')
const auth = require('../../services/auth')

Page({
  data: {
    user: {},
    nickname: '',
    avatarUrl: ''
  },
  onShow() {
    const user = wx.getStorageSync('user') || {}
    this.setData({
      user,
      nickname: user.nickname || '',
      avatarUrl: user.avatar_url || ''
    })
  },
  login() {
    auth.ensureLogin().then((user) => {
      this.setData({
        user: user || wx.getStorageSync('user') || {},
        nickname: (user && user.nickname) || '',
        avatarUrl: (user && user.avatar_url) || ''
      })
      wx.showToast({ title: '登录成功' })
    }).catch(() => wx.showToast({ title: '登录失败', icon: 'none' }))
  },
  onChooseAvatar(e) {
    const url = e.detail.avatarUrl
    this.setData({ avatarUrl: url })
    this.saveProfile({ avatar_url: url })
  },
  onNickname(e) {
    this.setData({ nickname: e.detail.value })
  },
  saveNickname() {
    this.saveProfile({ nickname: this.data.nickname })
  },
  saveProfile(patch) {
    auth.ensureLogin().then(() => api.patch('/me', patch)).then((u) => {
      wx.setStorageSync('user', u)
      this.setData({
        user: u,
        nickname: u.nickname || '',
        avatarUrl: u.avatar_url || ''
      })
      wx.showToast({ title: '已保存' })
    }).catch(() => wx.showToast({ title: '保存失败', icon: 'none' }))
  },
  go(e) {
    auth.ensureLogin().then(() => {
      wx.navigateTo({ url: e.currentTarget.dataset.url })
    }).catch(() => wx.showToast({ title: '请先登录', icon: 'none' }))
  }
})
