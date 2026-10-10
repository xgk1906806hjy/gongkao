const api = require('../../../services/api')

Page({
  data: {
    sessionId: 0,
    questions: [],
    index: 0,
    total: 0,
    q: null,
    answers: {},
    reveals: {},
    showAnalysis: true,
    locked: false
  },
  onLoad() {
    const sess = wx.getStorageSync('practice_session') || {}
    const questions = sess.questions || []
    this.setData({
      sessionId: sess.session_id,
      questions,
      total: questions.length,
      q: questions[0] || null,
      index: 0,
      answers: {},
      reveals: {},
      showAnalysis: sess.show_analysis !== false,
      locked: false
    })
  },
  choose(e) {
    if (this.data.locked) return
    const key = e.currentTarget.dataset.key
    const qid = this.data.q.id
    const answers = { ...this.data.answers, [qid]: key }
    this.setData({ answers })
    if (!this.data.showAnalysis || this.data.reveals[qid]) return
    this.setData({ locked: true })
    api.post('/practice/check', {
      session_id: this.data.sessionId,
      question_id: qid,
      user_key: key
    }).then((res) => {
      const reveals = {
        ...this.data.reveals,
        [qid]: {
          is_correct: res.is_correct,
          answer_key: res.answer_key,
          analysis: res.analysis || ''
        }
      }
      this.setData({ reveals, locked: false })
    }).catch(() => {
      this.setData({ locked: false })
    })
  },
  prev() {
    const index = this.data.index - 1
    this.setData({ index, q: this.data.questions[index] })
  },
  next() {
    const index = this.data.index + 1
    this.setData({ index, q: this.data.questions[index] })
  },
  submit() {
    const answers = Object.keys(this.data.answers).map((qid) => ({
      question_id: Number(qid),
      user_key: this.data.answers[qid]
    }))
    api.post('/practice/submit', {
      session_id: this.data.sessionId,
      answers
    }).then((res) => {
      wx.setStorageSync('practice_result', res)
      wx.redirectTo({ url: '/pages/practice/result/result' })
    }).catch((e) => {
      wx.showToast({ title: e.message || '交卷失败', icon: 'none' })
    })
  }
})
