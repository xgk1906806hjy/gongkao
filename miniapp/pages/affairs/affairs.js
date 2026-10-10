const api = require('../../services/api')

Page({
  data: {
    list: [],
    keyword: '',
    tag: '',
    tags: ['全部', '经济', '会议', '民生', '就业', '学习']
  },
  onShow() { this.load() },
  onKeyword(e) { this.setData({ keyword: e.detail.value }) },
  onTag(e) {
    const tag = e.currentTarget.dataset.tag
    this.setData({ tag: tag === '全部' ? '' : tag }, () => this.load())
  },
  load() {
    let q = '?page=1&page_size=20'
    if (this.data.keyword) q += '&keyword=' + encodeURIComponent(this.data.keyword)
    if (this.data.tag) q += '&tag=' + encodeURIComponent(this.data.tag)
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
