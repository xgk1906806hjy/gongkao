const api = require('../../services/api')

function fmt(t) {
  return String(t || '').replace('T', ' ').slice(0, 16)
}

function nodeSummary(it) {
  if (it.register_end) return '报名截止 ' + fmt(it.register_end)
  if (it.register_start) return '报名开始 ' + fmt(it.register_start)
  if (it.confirm_end) return '确认截止 ' + fmt(it.confirm_end)
  if (it.exam_at) return '考试 ' + fmt(it.exam_at)
  return ''
}

Page({
  data: {
    type: 'civil',
    keyword: '',
    region: '',
    publishedFrom: '',
    publishedTo: '',
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
  onRegion(e) {
    this.setData({ region: e.detail.value })
  },
  onFrom(e) {
    this.setData({ publishedFrom: e.detail.value })
  },
  onTo(e) {
    this.setData({ publishedTo: e.detail.value })
  },
  load() {
    const { type, keyword, region, publishedFrom, publishedTo } = this.data
    let q = `?type=${type}&page=1&page_size=20`
    if (keyword) q += `&keyword=${encodeURIComponent(keyword)}`
    if (region) q += `&region=${encodeURIComponent(region)}`
    if (publishedFrom) q += `&published_from=${publishedFrom}`
    if (publishedTo) q += `&published_to=${publishedTo}`
    api.get('/announcements' + q, false).then((data) => {
      const list = (data.list || []).map((it) => ({
        ...it,
        published_at: fmt(it.published_at),
        node_summary: nodeSummary(it)
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
