const api = require('../../services/api')

Page({
  data: { list: [], keyword: '' },
  onShow() { this.load() },
  onKeyword(e) { this.setData({ keyword: e.detail.value }) },
  load() {
    let q = '?page=1&page_size=20'
    if (this.data.keyword) q += '&keyword=' + encodeURIComponent(this.data.keyword)
    api.get('/affairs' + q, false).then((d) => {
      const list = (d.list || []).map((it) => ({
        ...it,
        published_at: (it.published_at || '').replace('T', ' ').slice(0, 16)
      }))
      this.setData({ list })
    })
  },
  goDetail(e) {
    wx.navigateTo({ url: '/pages/affairs/detail?id=' + e.currentTarget.dataset.id })
  }
})