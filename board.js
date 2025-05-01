class SatCounter {
  constructor(renderer, countUp = true) {
    this.total = 0
    this.drawTotal = 0
    this.increment = 0
    this.interval = null
    this.renderer = renderer
    this.countUp = countUp
  }

  randomInt(min, max) {
    return Math.floor((Math.random() * (max - min)) + min)
  }

  add(payment) {
    this.total += payment.sats
    this.increment = Math.floor(this.total / 240)
    this.shouldRender()
  }

  shouldRender() {
    if (this.interval) return

    this.interval = setInterval(() => {
      if (this.drawTotal >= this.total) {
        clearInterval(this.interval)
        this.interval = null
        return
      }
      this.render()
    }, 10)
  }

  render() {
    const diff = this.total - this.drawTotal

    if (!this.countUp) {
      this.drawTotal = this.total
    }
    else if (diff > 0) {
      this.drawTotal += Math.min(diff, this.randomInt(50, this.increment))
    }

    this.renderer(this.drawTotal.toLocaleString())
  }
}

class LatestPayment {
  constructor(renderer) {
    this.current = null
    this.renderer = renderer
  }

  add(boost) {
    if (this.current && this.current.creation_date > boost.creation_date) return
    if (boost.action !== 'boost' && boost.action !== 'zap') return

    this.current = boost
    this.render()
  }

  render() {
    const { sender_name, sats, type, app_name, podcast, message } = this.current

    const isBoost = type === 'boost'
    const boostzap = isBoost ? 'BOOSTED' : 'ZAPPED'
    const app = isBoost ? `FROM ${app_name}` : ''

    let band = ''
    if (app_name === "The Split Kit" && !podcast.includes('Satellite Skirmish')) {
      band = `FOR ${podcast}` // split kit passes the band in the podcast name
    }

    const messageArray = [
      `${sender_name} ${boostzap} ${sats.toLocaleString()} SATS ${band} ${app}`.toUpperCase()
    ]

    if (message) {
      messageArray.push(message)
    }

    this.renderer(messageArray)
  }
}

class TopScore {
  constructor(position, name = null, sats = null) {
    this.position = position
    this.name = name
    this.sats = sats
    this.updated = 0
  }

  update(name, sats) {
    if (name !== this.name || sats !== this.sats) {
      this.updated = 90
    }

    this.name = name
    this.sats = sats
  }

  isUpdated() {
    return this.updated > 0 && this.updated--
  }
}

class Scores {
  constructor(metric, num, renderer) {
    this.scores = {}
    this.topScores = []
    this.metric = metric
    this.renderer = renderer

    for (let idx = 0; idx < num; idx++) {
      this.topScores[idx] = new TopScore(idx + 1)
    }
  }

  add(payment, old) {
    const name = payment[this.metric].toUpperCase()

    if (!this.scores[name]) {
      this.scores[name] = { name, sats: 0 }
    }

    this.scores[name].sats += payment.sats
    this.updateTopScores()
    this.render(old)
  }

  updateTopScores() {
    const newTopScores = Object.values(this.scores)
      .sort((a, b) => b.sats - a.sats)
      .slice(0, this.topScores.length)

    newTopScores.forEach((score, idx) => {
      this.topScores[idx].update(score.name, score.sats)
    })
  }

  getTopScores() {
    return this.topScores.filter(score => score.name)
  }

  escapeHtml(text) {
    const div = document.createElement("div")
    div.appendChild(document.createTextNode(text))
    return div.innerHTML
  }

  render(old) {
    this.topScores.forEach(score => this.renderer(score, old))
  }
}