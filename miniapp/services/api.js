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

function request(method, path, data, auth) {
  const base = (app && app.globalData && app.globalData.baseURL) || 'http://127.0.0.1:8080/api/v1'
  const header = { 'Content-Type': 'application/json' }
  if (auth !== false) {
    const { access } = getTokens()
    if (access) header.Authorization = 'Bearer ' + access
  }
  return new Promise((resolve, reject) => {
    wx.request({
      url: base + path,
      method,
      data,
      header,
      success(res) {
        if (res.statusCode === 401) {
          clearTokens()
          reject(new Error('unauthorized'))
          return
        }
        const body = res.data || {}
        if (body.code !== 0) {
          reject(new Error(body.message || 'request failed'))
          return
        }
        resolve(body.data)
      },
      fail(err) {
        reject(err)
      }
    })
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