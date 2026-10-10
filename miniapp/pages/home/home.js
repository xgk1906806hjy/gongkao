const api = require('../../services/api')

Page({
  data: {
    type: 'civil',
    keyword: '',
    list: []
  },
  onShow() {
    this.load()
  },
  onType(e) {
    this.setData({ type: e.currentTarget.dataset.type }, () => this.load())
  },
  onKeyword(e) {
    this.setData({ keyword: e.detail.value })
  },
  load() {
    const { type, keyword } = this.data
    let q = `?type=${type}&page=1&page_size=20`
    if (keyword) q += `&keyword=${encodeURIComponent(keyword)}`
    api.get('/announcements' + q, false).then((data) => {
      const list = (data.list || []).map((it) => ({
        ...it,
        published_at: (it.published_at || '').replace('T', ' ').slice(0, 16)
      }))
      this.setData({ list })
    }).catch(() => {
      wx.showToast({ title: '加载失败', icon: 'none' })
    })
  },
  goDetail(e) {
    wx.navigateTo({ url: '/pages/announcement/detail?id=' + e.currentTarget.dataset.id })
  }
})