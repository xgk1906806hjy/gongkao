const api = require('../../services/api')
const auth = require('../../services/auth')

const EDUCATIONS = ['未填写', '大专', '本科', '硕士', '博士']
const POLITICALS = ['未填写', '中共党员', '中共预备党员', '共青团员', '群众']
const TYPES = [
  { label: '不限', value: '' },
  { label: '公务员', value: 'civil' },
  { label: '事业单位', value: 'institution' }
]
const HOURS = ['1', '2', '3', '4', '5', '6', '7', '8', '10', '12']
const KIND_TAB = { resume: 'resume', plan: 'plan', ask: 'ask' }

function fmtDate(t) {
  return t ? String(t).slice(0, 10) : ''
}

function pct(v) {
  return Math.round((v || 0) * 100) + '%'
}

function decorateResume(r) {
  if (!r) return null
  return {
    ...r,
    recommendations: (r.recommendations || []).map((it) => ({
      ...it,
      type_label: it.type === 'institution' ? '事业单位' : '公务员',
      register_end_text: fmtDate(it.register_end),
      exam_at_text: fmtDate(it.exam_at)
    }))
  }
}

function decoratePlan(p) {
  if (!p) return null
  return {
    ...p,
    diagnosis: (p.diagnosis || []).map((d) => ({ ...d, accuracy_text: d.answered ? pct(d.accuracy) : '-' }))
  }
}

Page({
  data: {
    tab: 'resume',
    status: null,
    educations: EDUCATIONS,
    politicals: POLITICALS,
    types: TYPES,
    hours: HOURS,
    profile: {},
    eduIndex: 0,
    polIndex: 0,
    typeIndex: 0,
    resumeText: '',
    showForm: false,
    resume: null,
    examDate: '',
    today: '',
    hourIndex: 2,
    focus: '',
    plan: null,
    question: '',
    chats: [],
    reports: [],
    loading: false
  },

  onLoad(q) {
    const now = new Date()
    const pad = (n) => (n < 10 ? '0' + n : '' + n)
    this.setData({
      tab: KIND_TAB[q.tab] || 'resume',
      today: now.getFullYear() + '-' + pad(now.getMonth() + 1) + '-' + pad(now.getDate())
    })
    auth.ensureLogin().then(() => {
      this.loadStatus()
      this.loadProfile()
    }).catch(() => wx.showToast({ title: '请先登录', icon: 'none' }))
  },

  loadStatus() {
    api.get('/ai/status').then((status) => this.setData({ status })).catch(() => null)
  },

  loadProfile() {
    api.get('/me/ai/profile').then((p) => {
      const hourIdx = HOURS.indexOf(String(p.daily_hours || 3))
      this.setData({
        profile: p,
        resumeText: p.resume_text || '',
        eduIndex: Math.max(0, EDUCATIONS.indexOf(p.education)),
        polIndex: Math.max(0, POLITICALS.indexOf(p.political_status)),
        typeIndex: Math.max(0, TYPES.findIndex((t) => t.value === (p.target_types || ''))),
        examDate: fmtDate(p.exam_date),
        hourIndex: hourIdx >= 0 ? hourIdx : 2
      })
    }).catch(() => null)
  },

  switchTab(e) {
    const tab = e.currentTarget.dataset.tab
    this.setData({ tab })
    if (tab === 'history') this.loadReports()
  },

  // ---------- resume ----------
  onResumeInput(e) { this.setData({ resumeText: e.detail.value }) },
  toggleForm() { this.setData({ showForm: !this.data.showForm }) },
  onEdu(e) { this.setData({ eduIndex: Number(e.detail.value) }) },
  onPol(e) { this.setData({ polIndex: Number(e.detail.value) }) },
  onType(e) { this.setData({ typeIndex: Number(e.detail.value) }) },
  onField(e) {
    const key = e.currentTarget.dataset.key
    this.setData({ ['profile.' + key]: e.detail.value })
  },
  onFresh(e) { this.setData({ 'profile.fresh_graduate': e.detail.value }) },

  profilePayload() {
    const p = this.data.profile
    const edu = EDUCATIONS[this.data.eduIndex]
    const pol = POLITICALS[this.data.polIndex]
    return {
      education: this.data.eduIndex ? edu : '',
      political_status: this.data.polIndex ? pol : '',
      major: p.major || '',
      target_regions: p.target_regions || '',
      target_types: TYPES[this.data.typeIndex].value,
      fresh_graduate: !!p.fresh_graduate,
      work_years: Number(p.work_years || 0),
      certificates: p.certificates || ''
    }
  },

  analyze() {
    if (this.data.loading) return
    const text = (this.data.resumeText || '').trim()
    const payload = this.profilePayload()
    if (!text && !payload.major && !payload.education) {
      wx.showToast({ title: '请粘贴简历或填写基本信息', icon: 'none' })
      return
    }
    this.setData({ loading: true })
    wx.showLoading({ title: 'AI 分析中...' })
    api.post('/ai/resume/analyze', { resume_text: text, profile: payload }).then((res) => {
      this.setData({ resume: decorateResume(res) })
      this.loadProfile()
      this.loadStatus()
    }).catch((err) => {
      wx.showToast({ title: err.message || '分析失败', icon: 'none' })
    }).finally(() => {
      wx.hideLoading()
      this.setData({ loading: false })
    })
  },

  openAnnouncement(e) {
    wx.navigateTo({ url: '/pages/announcement/detail?id=' + e.currentTarget.dataset.id })
  },

  // ---------- plan ----------
  onExamDate(e) { this.setData({ examDate: e.detail.value }) },
  onHours(e) { this.setData({ hourIndex: Number(e.detail.value) }) },
  onFocus(e) { this.setData({ focus: e.detail.value }) },

  makePlan() {
    if (this.data.loading) return
    this.setData({ loading: true })
    wx.showLoading({ title: '生成计划中...' })
    api.post('/ai/study-plan', {
      exam_date: this.data.examDate,
      daily_hours: Number(HOURS[this.data.hourIndex]),
      focus: this.data.focus
    }).then((res) => {
      this.setData({ plan: decoratePlan(res), examDate: res.exam_date })
      this.loadStatus()
    }).catch((err) => {
      wx.showToast({ title: err.message || '生成失败', icon: 'none' })
    }).finally(() => {
      wx.hideLoading()
      this.setData({ loading: false })
    })
  },

  goPractice() {
    wx.switchTab({ url: '/pages/practice/practice' })
  },

  // ---------- ask ----------
  onQuestion(e) { this.setData({ question: e.detail.value }) },

  ask() {
    const q = (this.data.question || '').trim()
    if (!q || this.data.loading) return
    const chats = this.data.chats.concat([{ role: 'user', text: q }])
    this.setData({ chats, question: '', loading: true })
    api.post('/ai/ask', { question: q }).then((res) => {
      this.setData({ chats: this.data.chats.concat([{ role: 'ai', text: res.answer, notice: res.notice || '' }]) })
      this.loadStatus()
    }).catch((err) => {
      this.setData({ chats: this.data.chats.concat([{ role: 'ai', text: err.message || '请求失败' }]) })
    }).finally(() => this.setData({ loading: false }))
  },

  // ---------- history ----------
  loadReports() {
    api.get('/me/ai/reports?page_size=30').then((d) => {
      const reports = (d.list || []).map((r) => ({
        ...r,
        kind_label: { resume: '简历分析', plan: '复习计划', ask: '问答' }[r.kind] || r.kind,
        created_text: String(r.created_at || '').replace('T', ' ').slice(0, 16)
      }))
      this.setData({ reports })
    }).catch(() => null)
  },

  openReport(e) {
    const id = e.currentTarget.dataset.id
    api.get('/me/ai/reports/' + id).then((d) => {
      const r = d.result || {}
      if (d.kind === 'resume') this.setData({ tab: 'resume', resume: decorateResume(r) })
      else if (d.kind === 'plan') this.setData({ tab: 'plan', plan: decoratePlan(r) })
      else this.setData({ tab: 'ask', chats: [{ role: 'user', text: d.title }, { role: 'ai', text: r.answer || '' }] })
    }).catch((err) => wx.showToast({ title: err.message || '加载失败', icon: 'none' }))
  },

  clearData() {
    wx.showModal({
      title: '清除 AI 数据',
      content: '将删除已保存的简历、资料和全部分析记录，确定吗？',
      success: (res) => {
        if (!res.confirm) return
        api.del('/me/ai/profile').then(() => {
          this.setData({ resume: null, plan: null, chats: [], reports: [], resumeText: '' })
          this.loadProfile()
          wx.showToast({ title: '已清除' })
        })
      }
    })
  }
})
