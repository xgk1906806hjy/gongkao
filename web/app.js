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
  if (state.view?.name === 'ai') return renderAI(app)

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
    <div class="card menu" data-go="ai"><strong>AI 备考助手</strong><div class="muted">简历选岗 · 复习计划 · 备考问答</div></div>
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

function esc(s) {
  return String(s ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]))
}

function list(items, mark = '·') {
  return (items || []).map((x) => `<div>${mark} ${esc(x)}</div>`).join('')
}

async function renderAI(app) {
  setTitle('AI 备考助手')
  const aiTab = state.view.tab || 'resume'
  const status = await req('GET', '/ai/status').catch(() => null)
  const profile = await req('GET', '/me/ai/profile').catch(() => ({}))
  app.innerHTML = `
    <button class="btn ghost back" id="back">返回</button>
    <div class="tabs">
      <button data-ai="resume" class="${aiTab === 'resume' ? 'on' : ''}">简历选岗</button>
      <button data-ai="plan" class="${aiTab === 'plan' ? 'on' : ''}">复习计划</button>
      <button data-ai="ask" class="${aiTab === 'ask' ? 'on' : ''}">备考问答</button>
    </div>
    <div class="muted">${status ? (status.llm_enabled ? `AI 模型：${esc(status.model)} · 今日 ${status.used_today}/${status.daily_limit}` : '规则引擎模式（未配置大模型）') : ''}</div>
    <div id="aiBox"></div>
    <div id="aiOut"></div>`
  $('#back').onclick = () => { state.view = null; render() }
  app.querySelectorAll('[data-ai]').forEach((b) => b.onclick = () => { state.view = { name: 'ai', tab: b.dataset.ai }; render() })
  const box = $('#aiBox')
  const out = $('#aiOut')
  const run = async (fn) => {
    out.innerHTML = '<div class="empty">AI 分析中...</div>'
    try { out.innerHTML = await fn() } catch (e) { out.innerHTML = `<div class="empty">${esc(e.message)}</div>` }
  }

  if (aiTab === 'resume') {
    box.innerHTML = `
      <div class="card">
        <textarea id="resume" rows="8" style="width:100%;box-sizing:border-box" placeholder="粘贴简历：学历、专业、毕业时间、政治面貌、证书、意向地区…">${esc(profile.resume_text || '')}</textarea>
        <input id="regions" placeholder="意向地区，如 湖南,广东" value="${esc(profile.target_regions || '')}" style="width:100%;margin-top:8px;box-sizing:border-box">
        <button class="btn" id="go" style="margin-top:8px">分析简历 · 推荐岗位</button>
      </div>`
    $('#go').onclick = () => run(async () => {
      const r = await req('POST', '/ai/resume/analyze', {
        resume_text: $('#resume').value,
        profile: { target_regions: $('#regions').value },
      })
      return `
        ${r.notice ? `<div class="card muted">${esc(r.notice)}</div>` : ''}
        <div class="card"><div class="title">综合评估</div><div>${esc(r.summary)}</div></div>
        <div class="card"><div class="title">报考优势</div>${list(r.strengths, '✓')}</div>
        <div class="card"><div class="title">注意事项</div>${list(r.weaknesses, '!')}</div>
        <div class="card"><div class="title">岗位方向</div>${(r.directions || []).map((d) => `<div><strong>${esc(d.name)}</strong> <span class="muted">${esc(d.reason)}</span></div>`).join('')}</div>
        <div class="card"><div class="title">推荐考试</div>${(r.recommendations || []).map((it) => `
          <div style="padding:8px 0;border-top:1px solid #f0f2f5">
            <div><strong>${esc(it.title)}</strong> <span class="muted">匹配度 ${it.score}</span></div>
            <div class="muted">${esc(it.region || '')} ${it.register_end ? '· 报名截止 ' + fmt(it.register_end) : ''}</div>
            ${list(it.reasons, '✓')}${list(it.cautions, '!')}
          </div>`).join('') || '<div class="muted">暂无匹配公告</div>'}</div>
        <div class="card"><div class="title">下一步</div>${list(r.next_steps)}</div>`
    })
  } else if (aiTab === 'plan') {
    box.innerHTML = `
      <div class="card">
        <label>考试日期 <input type="date" id="exam" value="${esc((profile.exam_date || '').slice(0, 10))}"></label>
        <label style="margin-left:8px">每天小时 <input type="number" id="hours" min="1" max="16" value="${profile.daily_hours || 3}" style="width:60px"></label>
        <input id="focus" placeholder="补充诉求（可选）" style="width:100%;margin-top:8px;box-sizing:border-box">
        <button class="btn" id="go" style="margin-top:8px">生成复习计划</button>
      </div>`
    $('#go').onclick = () => run(async () => {
      const p = await req('POST', '/ai/study-plan', { exam_date: $('#exam').value, daily_hours: Number($('#hours').value), focus: $('#focus').value })
      return `
        ${p.notice ? `<div class="card muted">${esc(p.notice)}</div>` : ''}
        <div class="card"><div class="title">距考试 ${p.days_left} 天（${p.exam_date}）</div><div class="muted">${esc(p.exam_date_note || '')}</div><div>${esc(p.summary)}</div></div>
        <div class="card"><div class="title">能力诊断</div>${(p.diagnosis || []).map((d) => `<div>${esc(d.category)} <span class="muted">${d.answered} 题 · ${d.answered ? Math.round(d.accuracy * 100) + '%' : '-'} · ${esc(d.level)}</span></div>`).join('')}</div>
        <div class="card"><div class="title">每日模板</div>${(p.daily_template || []).map((b) => `<div><strong>${esc(b.module)}</strong> ${b.minutes} 分钟 <span class="muted">${esc(b.task)}</span></div>`).join('')}</div>
        <div class="card"><div class="title">阶段</div>${(p.phases || []).map((ph) => `<div><strong>${esc(ph.name)}</strong> ${ph.start} ~ ${ph.end} <span class="muted">${esc(ph.goal)}</span></div>`).join('')}</div>
        <div class="card"><div class="title">周计划</div>${(p.weekly || []).map((w) => `<div style="padding:6px 0"><strong>第 ${w.week} 周</strong> <span class="muted">${w.start} ~ ${w.end} · ${esc(w.focus)}</span>${list(w.tasks)}</div>`).join('')}</div>
        <div class="card"><div class="title">建议</div>${list(p.tips)}</div>`
    })
  } else {
    box.innerHTML = `
      <div class="card" style="display:flex;gap:8px">
        <input id="q" placeholder="如：行测时间怎么分配？" style="flex:1">
        <button class="btn" id="go">发送</button>
      </div>`
    $('#go').onclick = () => run(async () => {
      const a = await req('POST', '/ai/ask', { question: $('#q').value })
      return `<div class="card">${a.notice ? `<div class="muted">${esc(a.notice)}</div>` : ''}<div style="white-space:pre-wrap">${esc(a.answer)}</div></div>`
    })
  }
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