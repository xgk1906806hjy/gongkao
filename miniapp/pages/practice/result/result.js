Page({
  data: { result: {}, accuracy: '-' },
  onLoad() {
    const result = wx.getStorageSync('practice_result') || {}
    const accuracy = result.total ? Math.round(result.accuracy * 100) + '%' : '-'
    this.setData({ result, accuracy })
  },
  back() {
    wx.switchTab({ url: '/pages/practice/practice' })
  },
  wrong() {
    wx.navigateTo({ url: '/pages/mine/wrong/wrong' })
  }
})