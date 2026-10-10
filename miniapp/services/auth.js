const api = require('./api')

function ensureLogin() {
  const { access } = api.getTokens()
  if (access) {
    return Promise.resolve(wx.getStorageSync('user') || null)
  }
  return new Promise((resolve, reject) => {
    wx.login({
      success(loginRes) {
        api.post('/auth/wechat', { code: loginRes.code || 'dev' }, false)
          .then((data) => {
            api.setTokens(data.access_token, data.refresh_token)
            wx.setStorageSync('user', data.user)
            resolve(data.user)
          })
          .catch(reject)
      },
      fail: reject
    })
  })
}

function requestSubscribeTodo() {
  // replace with real template ids after审核
  const tmplIds = ['TODO_TEMPLATE']
  return new Promise((resolve) => {
    wx.requestSubscribeMessage({
      tmplIds,
      complete: resolve
    })
  })
}

module.exports = {
  ensureLogin,
  requestSubscribeTodo
}