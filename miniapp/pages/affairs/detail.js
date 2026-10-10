const api = require('../../services/api')
const auth = require('../../services/auth')

Page({
  data: { id: 0, item: null },
  onLoad(q) {
    this.setData({ id: Number(q.id || 0) })
    auth.ensureLogin().catch(() => null).finally(() => {
      api.get('/affairs/' + this.data.id).then((item) => {
        item.published_at = (item.published_at || '').replace('T', ' ').slice(0, 16)
        this.setData({ item })
      })
    })
  },
  fav() {
    auth.ensureLogin().then(() => api.post('/me/affairs/favorites', { article_id: this.data.id }))
      .then(() => wx.showToast({ title: '已收藏' }))
      .catch((e) => wx.showToast({ title: e.message || '失败', icon: 'none' }))
  }
})