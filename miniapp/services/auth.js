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

function loadClientConfig() {
  const cached = wx.getStorageSync('client_config')
  if (cached && cached.todo_template_id) {
    return Promise.resolve(cached)
  }
  return api.get('/config/client', false).then((cfg) => {
    wx.setStorageSync('client_config', cfg || {})
    return cfg || {}
  }).catch(() => cached || {})
}

function reportSubscribe(result, tmplTodo, tmplAffairs) {
  const prefs = {}
  if (tmplTodo && result[tmplTodo] !== undefined) {
    prefs.todo = result[tmplTodo] === 'accept' ? 'accept' : 'reject'
  }
  if (tmplAffairs && result[tmplAffairs] !== undefined) {
    prefs.affairs = result[tmplAffairs] === 'accept' ? 'accept' : 'reject'
  }
  if (!prefs.todo && !prefs.affairs) return Promise.resolve()
  return api.post('/me/subscribe-prefs', prefs).catch(() => {})
}

function requestSubscribe(kinds) {
  const wantTodo = !kinds || kinds.indexOf('todo') >= 0
  const wantAffairs = kinds && kinds.indexOf('affairs') >= 0
  return ensureLogin().then(() => loadClientConfig()).then((cfg) => {
    const tmplIds = []
    const todoId = cfg.todo_template_id || 'MOCK_TODO_TEMPLATE'
    const affId = cfg.affairs_template_id || 'MOCK_AFFAIRS_TEMPLATE'
    if (wantTodo) tmplIds.push(todoId)
    if (wantAffairs) tmplIds.push(affId)
    return new Promise((resolve) => {
      wx.requestSubscribeMessage({
        tmplIds,
        success(res) {
          reportSubscribe(res, wantTodo ? todoId : '', wantAffairs ? affId : '').finally(() => resolve(res))
        },
        fail(err) {
          const rejectMap = {}
          tmplIds.forEach((id) => { rejectMap[id] = 'reject' })
          reportSubscribe(rejectMap, wantTodo ? todoId : '', wantAffairs ? affId : '').finally(() => resolve(err))
        }
      })
    })
  })
}

function requestSubscribeTodo() {
  return requestSubscribe(['todo'])
}

module.exports = {
  ensureLogin,
  loadClientConfig,
  requestSubscribe,
  requestSubscribeTodo
}
