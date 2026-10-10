const api = require('../../../services/api')
const auth = require('../../../services/auth')

Page({
  data: { list: [] },
  onShow() {
    auth.ensureLogin().then(() => api.get('/me/wrong-questions')).then((d) => {
      this.setData({ list: d.list || [] })
    })
  },
  remove(e) {
    api.del('/me/wrong-questions/' + e.currentTarget.dataset.id).then(() => this.onShow())
  }
})