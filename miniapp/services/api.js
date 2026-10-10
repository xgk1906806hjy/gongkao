const app = getApp()

function getTokens() {
  return {
    access: wx.getStorageSync('access_token') || '',
    refresh: wx.getStorageSync('refresh_token') || ''
  }
}

function setTokens(access, refresh) {
  if (access) wx.setStorageSync('access_token', access)
  if (refresh) wx.setStorageSync('refresh_token', refresh)
}

function clearTokens() {
  wx.removeStorageSync('access_token')
  wx.removeStorageSync('refresh_token')
  wx.removeStorageSync('user')
}

function baseURL() {
  return (app && app.globalData && app.globalData.baseURL) || 'http://127.0.0.1:8080/api/v1'
}

function rawRequest(method, path, data, header) {
  return new Promise((resolve, reject) => {
    wx.request({
      url: baseURL() + path,
      method,
      data,
      header,
      success(res) {
        resolve(res)
      },
      fail: reject
    })
  })
}

let refreshing = null

function refreshAccess() {
  if (refreshing) return refreshing
  const { refresh } = getTokens()
  if (!refresh) {
    return Promise.reject(new Error('unauthorized'))
  }
  refreshing = rawRequest('POST', '/auth/refresh', { refresh_token: refresh }, {
    'Content-Type': 'application/json'
  }).then((res) => {
    const body = res.data || {}
    if (res.statusCode !== 200 || body.code !== 0) {
      clearTokens()
      throw new Error('unauthorized')
    }
    setTokens(body.data.access_token, body.data.refresh_token)
    return body.data.access_token
  }).finally(() => {
    refreshing = null
  })
  return refreshing
}

function request(method, path, data, auth, retried) {
  const header = { 'Content-Type': 'application/json' }
  if (auth !== false) {
    const { access } = getTokens()
    if (access) header.Authorization = 'Bearer ' + access
  }
  return rawRequest(method, path, data, header).then((res) => {
    if (res.statusCode === 401 && auth !== false && !retried) {
      return refreshAccess().then(() => request(method, path, data, auth, true))
    }
    if (res.statusCode === 401) {
      clearTokens()
      throw new Error('unauthorized')
    }
    const body = res.data || {}
    if (body.code !== 0) {
      throw new Error(body.message || 'request failed')
    }
    return body.data
  })
}

module.exports = {
  getTokens,
  setTokens,
  clearTokens,
  get: (path, auth) => request('GET', path, {}, auth),
  post: (path, data, auth) => request('POST', path, data, auth),
  patch: (path, data, auth) => request('PATCH', path, data, auth),
  del: (path, auth) => request('DELETE', path, {}, auth)
}
