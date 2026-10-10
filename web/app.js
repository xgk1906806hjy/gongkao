const API = 'http://127.0.0.1:8080/api/v1'
const state = {
  tab: 'home',
  type: 'civil',
  keyword: '',
  token: localStorage.getItem('access_token') || '',
  user: JSON.parse(localStorage.getItem('user') || 'null'),
  view: null, // detail views
  quiz: null,
}

async function req(method, path, body, auth = true) {
  const headers = { 'Content-Type': 'application/json' }
  if (auth && state.token) headers.Authorization = 'Bearer ' + state.token
  const res = await fetch(API + path, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  })
  const json = await res.json()
  if (json.code !== 0) throw new Error(json.message || 'request failed')
  return json.data
}

async function ensureLogin() {
  if (state.token) return state.user
  const data = await req('POST', '/auth/wechat', { code: 'web_' + Date.now() }, false)
  state.token = data.access_token
  state.user = data.user
  localStorage.setItem('access_token', state.token)
  localStorage.setItem('user', JSON.stringify(state.user))
  return state.user
}

function $(sel) { return document.querySelector(sel) }
function fmt(t) { return String(t || '').replace('T', ' ').slice(0, 16) || '待定' }

function setTitle(t) { $('#navTitle').textContent = t }

async function render() {
  const app = $('#app')
  if (state.view?.name === 'annDetail') return renderAnnDetail(app)
  if (state.view?.name === 'affDetail') return renderAffDetail(app)
  if (state.view?.name === 'quiz') return renderQuiz(app)
  if (state.view?.name === 'result') return renderResult(app)
  if (state.view?.name === 'todos') return renderTodos(app)
  if (state.view?.name === 'exams') return renderExams(app)

  if (state.tab === 'home') return renderHome(app)
  if (state.tab === 'practice') return renderPractice(app)
  if (state.tab === 'affairs') return renderAffairs(app)
  return renderMine(app)
}

async function renderHome(app) {
  setTitle('考试公告')
  app.innerHTML = `
    <div class="tabs">
      <button data-type="civil" class="${state.type==='civil'?'on':''}">公务员</button>
      <button data-type="institution" class="${state.type==='institution'?'on':''}">事业单位</button>
    </div>
    <div class="row">
      <input id="kw" placeholder="搜索公告" value="${state.keyword}" />
      <button class="btn" id="search">搜索</button>
    </div>
    <div id="list" class="empty">加载中...</div>`
  app.querySelectorAll('.tabs button').forEach((b) => b.onclick = () => {
    state.type = b.dataset.type; render()
  })
  $('#search').onclick = () => { state.keyword = $('#kw').value; render() }
  try {
    let q = `?type=${state.type}&page=1&page_size=20`
    if (state.keyword) q += `&keyword=${encodeURIComponent(state.keyword)}`
    const data = await req('GET', '/announcements' + q, null, false)
    const list = data.list || []
    $('#list').innerHTML = list.length ? list.map((it) => `
      <div class="card item" data-id="${it.id}">
        <div class="title">${it.title}</div>
        <div class="muted">
          <span class="tag ${it.type==='institution'?'inst':''}">${it.type==='civil'?'公务员':'事业单位'}</span>
          ${it.region || '全国'} · ${fmt(it.published_at)}
        </div>
      </div>`).join('') : '<div class="empty">暂无公告</div>'
    app.querySelectorAll('.item').forEach((el) => el.onclick = () => {
      state.view = { name: 'annDetail', id: Number(el.dataset.id) }
      render()
    })
  } catch (e) {
    $('#list').innerHTML = `<div class="empty">加载失败：${e.message}<br/>请确认后端已启动</div>`
  }
}

async function renderAnnDetail(app) {
  setTitle('公告详情')
  app.innerHTML = `<button class="btn ghost back" id="back">返回</button><div id="box" class="empty">加载中...</div>`
  $('#back').onclick = () => { state.view = null; render() }
  try {
    const it = await req('GET', '/announcements/' + state.view.id, null, false)
    $('#box').innerHTML = `
      <div class="card">
        <div class="title">${it.title}</div>
        <div class="muted">${it.region || ''} · ${fmt(it.published_at)}</div>
        <p>${it.summary || ''}</p>
      </div>
      <div class="card">
        <div class="title">关键时间线</div>
        <div class="muted">报名：${fmt(it.register_start)} ~ ${fmt(it.register_end)}</div>
        <div class="muted">确认：${fmt(it.confirm_start)} ~ ${fmt(it.confirm_end)}</div>
        <div class="muted">考试：${fmt(it.exam_at)}</div>
      </div>
      <button class="btn" id="join" style="width:100%">加入我的考试</button>`
    $('#join').onclick = async () => {
      try {
        await ensureLogin()
        await req('POST', '/me/exams', { announcement_id: it.id })
        alert('已加入，并生成代办')
      } catch (e) { alert(e.message) }
    }
  } catch (e) {
    $('#box').innerHTML = `<div class="empty">${e.message}</div>`
  }
}

async function renderPractice(app) {
  setTitle('刷题')
  app.innerHTML = `<div id="box" class="empty">加载中...</div>`
  try {
    await ensureLogin().catch(() => null)
    const cats = await req('GET', '/questions/categories', null, false)
    let statsHtml = ''
    try {
      const stats = await req('GET', '/me/practice/stats')
      const acc = stats.total_answered ? Math.round(stats.accuracy * 100) + '%' : '-'
      statsHtml = `<div class="card">已练 ${stats.total_answered} 题 · 正确率 ${acc}</div>`
    } catch {}
    $('#box').innerHTML = statsHtml + (cats.list || []).map((c) => `
      <div class="card">
        <div class="title">${c.name}</div>
        <div class="actions">
          <button class="btn ghost" data-id="${c.id}" data-mode="sequential">顺序练</button>
          <button class="btn" data-id="${c.id}" data-mode="random">随机练</button>
        </div>
      </div>`).join('') || '<div class="empty">暂无分类</div>'
    app.querySelectorAll('button[data-id]').forEach((b) => b.onclick = async () => {
      try {
        await ensureLogin()
        const data = await req('POST', '/practice/sessions', {
          category_id: Number(b.dataset.id), mode: b.dataset.mode, count: 5
        })
        state.quiz = { ...data, index: 0, answers: {} }
        state.view = { name: 'quiz' }
        render()
      } catch (e) { alert(e.message) }
    })
  } catch (e) {
    $('#box').innerHTML = `<div class="empty">${e.message}</div>`
  }
}

function renderQuiz(app) {
  setTitle('答题')
  const q = state.quiz.questions[state.quiz.index]
  const total = state.quiz.questions.length
  app.innerHTML = `
    <div class="muted">${state.quiz.index + 1} / ${total}</div>
    <div class="card title">${q.stem}</div>
    ${(q.options || []).map((o) => `
      <div class="opt ${state.quiz.answers[q.id]===o.key?'on':''}" data-key="${o.key}">${o.key}. ${o.content}</div>
    `).join('')}
    <div class="actions">
      <button class="btn ghost" id="prev" ${state.quiz.index===0?'disabled':''}>上一题</button>
      ${state.quiz.index < total - 1
        ? '<button class="btn" id="next">下一题</button>'
        : '<button class="btn" id="submit">交卷</button>'}
    </div>`
  app.querySelectorAll('.opt').forEach((el) => el.onclick = () => {
    state.quiz.answers[q.id] = el.dataset.key
    render()
  })
  const prev = $('#prev'); if (prev) prev.onclick = () => { state.quiz.index--; render() }
  const next = $('#next'); if (next) next.onclick = () => { state.quiz.index++; render() }
  const submit = $('#submit'); if (submit) submit.onclick = async () => {
    const answers = Object.keys(state.quiz.answers).map((id) => ({
      question_id: Number(id), user_key: state.quiz.answers[id]
    }))
    try {
      const res = await req('POST', '/practice/submit', {
        session_id: state.quiz.session_id, answers
      })
      state.view = { name: 'result', data: res }
      render()
    } catch (e) { alert(e.message) }
  }
}

function renderResult(app) {
  setTitle('练习结果')
  const r = state.view.data
  app.innerHTML = `
    <div class="card">
      <div class="score">正确 ${r.correct} / ${r.total}</div>
      <div class="muted">正确率 ${Math.round((r.accuracy || 0) * 100)}%</div>
    </div>
    <button class="btn" id="back" style="width:100%">返回刷题</button>`
  $('#back').onclick = () => { state.view = null; state.tab = 'practice'; render() }
}

async function renderAffairs(app) {
  setTitle('时政')
  app.innerHTML = `<div id="list" class="empty">加载中...</div>`
  try {
    const data = await req('GET', '/affairs?page=1&page_size=20', null, false)
    const list = data.list || []
    $('#list').innerHTML = list.length ? list.map((it) => `
      <div class="card item" data-id="${it.id}">
        <div class="title">${it.title}</div>
        <div class="muted">${it.source || ''} · ${fmt(it.published_at)}</div>
        <div>${it.summary || ''}</div>
      </div>`).join('') : '<div class="empty">暂无时政</div>'
    app.querySelectorAll('.item').forEach((el) => el.onclick = () => {
      state.view = { name: 'affDetail', id: Number(el.dataset.id) }
      render()
    })
  } catch (e) {
    $('#list').innerHTML = `<div class="empty">${e.message}</div>`
  }
}

async function renderAffDetail(app) {
  setTitle('时政详情')
  app.innerHTML = `<button class="btn ghost back" id="back">返回</button><div id="box" class="empty">加载中...</div>`
  $('#back').onclick = () => { state.view = null; render() }
  try {
    await ensureLogin().catch(() => null)
    const it = await req('GET', '/affairs/' + state.view.id)
    $('#box').innerHTML = `
      <div class="card">
        <div class="title">${it.title}</div>
        <div class="muted">${it.source || ''} · ${fmt(it.published_at)}</div>
        <p style="white-space:pre-wrap">${it.content || it.summary || ''}</p>
      </div>
      <button class="btn" id="fav" style="width:100%">收藏</button>`
    $('#fav').onclick = async () => {
      try {
        await ensureLogin()
        await req('POST', '/me/affairs/favorites', { article_id: it.id })
        alert('已收藏')
      } catch (e) { alert(e.message) }
    }
  } catch (e) {
    $('#box').innerHTML = `<div class="empty">${e.message}</div>`
  }
}

async function renderMine(app) {
  setTitle('我的')
  const name = state.user?.nickname || '未登录'
  app.innerHTML = `
    <div class="card" style="display:flex;justify-content:space-between;align-items:center">
      <strong>${name}</strong>
      <button class="btn" id="login">微信登录(模拟)</button>
    </div>
    <div class="card menu" data-go="exams">我的考试</div>
    <div class="card menu" data-go="todos">我的代办</div>`
  $('#login').onclick = async () => {
    try { await ensureLogin(); render() } catch (e) { alert(e.message) }
  }
  app.querySelectorAll('[data-go]').forEach((el) => el.onclick = async () => {
    try {
      await ensureLogin()
      state.view = { name: el.dataset.go }
      render()
    } catch (e) { alert(e.message) }
  })
}

async function renderExams(app) {
  setTitle('我的考试')
  app.innerHTML = `<button class="btn ghost back" id="back">返回</button><div id="box" class="empty">加载中...</div>`
  $('#back').onclick = () => { state.view = null; render() }
  const data = await req('GET', '/me/exams')
  const list = data.list || []
  $('#box').innerHTML = list.length ? list.map((it) => `
    <div class="card">
      <div class="title">${it.announcement?.title || ('#' + it.announcement_id)}</div>
      <button class="btn ghost" data-id="${it.announcement_id}">移除</button>
    </div>`).join('') : '<div class="empty">还没有加入考试</div>'
  app.querySelectorAll('button[data-id]').forEach((b) => b.onclick = async () => {
    await req('DELETE', '/me/exams/' + b.dataset.id)
    render()
  })
}

async function renderTodos(app) {
  setTitle('我的代办')
  app.innerHTML = `<button class="btn ghost back" id="back">返回</button><div id="box" class="empty">加载中...</div>`
  $('#back').onclick = () => { state.view = null; render() }
  const data = await req('GET', '/me/todos')
  const groups = data.list || []
  $('#box').innerHTML = groups.length ? groups.map((g) => `
    <div class="card">
      <div class="title">${g.announcement?.title || ('公告#' + g.announcement_id)}</div>
      ${(g.todos || []).map((t) => `
        <div style="display:flex;justify-content:space-between;gap:8px;padding:8px 0;border-top:1px solid #f0f2f5">
          <div>
            <div>${t.title}</div>
            <div class="muted">截止 ${fmt(t.due_at)} · 提醒 ${fmt(t.remind_at)}</div>
          </div>
          ${t.status === 'pending' ? `<button class="btn ghost" data-id="${t.id}">完成</button>` : '<span class="muted">已完成</span>'}
        </div>`).join('')}
    </div>`).join('') : '<div class="empty">暂无代办</div>'
  app.querySelectorAll('button[data-id]').forEach((b) => b.onclick = async () => {
    await req('PATCH', '/me/todos/' + b.dataset.id, { status: 'done' })
    render()
  })
}

document.querySelectorAll('.tabbar button').forEach((b) => {
  b.onclick = () => {
    state.tab = b.dataset.tab
    state.view = null
    document.querySelectorAll('.tabbar button').forEach((x) => x.classList.toggle('on', x === b))
    render()
  }
})

render()