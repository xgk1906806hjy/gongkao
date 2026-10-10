const api = require('../../../services/api')
const auth = require('../../../services/auth')

function statusText(v) {
  if (v === 'accept') return '已授权'
  if (v === 'reject') return '已拒绝 / 未授权'
  return '未设置'
}

Page({
  data: {
    todoStatusText: '加载中…',
    affairsStatusText: '加载中…'
  },
  onShow() {
    auth.ensureLogin().then(() => api.get('/me/subscribe-prefs')).then((p) => {
      this.setData({
        todoStatusText: statusText(p.todo),
        affairsStatusText: statusText(p.affairs)
      })
    }).catch(() => {
      this.setData({ todoStatusText: '请先登录', affairsStatusText: '请先登录' })
    })
  },
  subTodo() {
    auth.requestSubscribe(['todo']).then(() => {
      wx.showToast({ title: '已发起授权', icon: 'none' })
      this.onShow()
    })
  },
  subAffairs() {
    auth.requestSubscribe(['affairs']).then(() => {
      wx.showToast({ title: '已发起授权', icon: 'none' })
      this.onShow()
    })
  }
})
