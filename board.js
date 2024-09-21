function SatCounter(renderer) {
    this.total = 0
    this.drawTotal = 0
    this.increment = 0
    this.interval = null
    this.renderer = renderer

    const randomInt = (min, max) => {
        return Math.floor((Math.random() * (max - min)) + min)
    }

    this.add = (payment, old) => {
        this.total += payment.sats
        this.increment = Math.floor(this.total / 240)
        this.shouldRender()
    }

    this.shouldRender = () => {
        if (this.interval) {
            return
        }

        this.interval = setInterval(() => {
            if (this.drawTotal >= this.total) {
                clearInterval(this.interval)
                this.interval = null
            }

            this.render()
        }, 10)
    }

    this.render = () => {
        const diff = this.total - this.drawTotal

        if (diff > 0) {
            this.drawTotal += Math.min(diff, randomInt(50, this.increment))
        }

        this.renderer(this.drawTotal.toLocaleString())
    }
}

function LatestPayment(renderer) {
    this.current = null
    this.renderer = renderer

    this.add = (boost) => {
        if (this.current && this.current.creation_date > boost.creation_date) {
            return
        }

        if (boost.action != 'boost' && boost.action != 'zap') {
            return // show only boosts and zaps for now
        }

        this.current = boost
        this.render()
    }

    this.render = () => {
        const sender = this.current.sender_name
        const sats = this.current.sats.toLocaleString()
        const boostzap = (this.current.type == 'boost' ? 'BOOSTED' : 'ZAPPED')
        const app = (this.current.type == 'boost' ? `FROM ${this.current.app_name}` : '')
        let band = '';

        if (this.current.app_name == "The Split Kit" && this.current.podcast.indexOf('Satellite Skirmish') === -1) {
            band = `FOR ${this.current.podcast}` // split kit passes the band in the podcast name
        }

        let message = [`${sender} ${boostzap} ${sats} SATS ${band} ${app}`.toUpperCase()]

        if (this.current.message) {
            message.push(this.current.message)
        }

        this.renderer(message)
    }
}

function TopScore(position, name, sats) {
    this.position = position
    this.name = name
    this.sats = sats
    this.updated = 0

    this.update = (name, sats) => {
        if (name != this.name || sats != this.sats) {
            this.updated = 90
        }

        this.name = name
        this.sats = sats
    }

    this.isUpdated = () => {
        return this.updated > 0 && this.updated--
    }
}

function Scores(metric, num, renderer) {
    this.scores = {}
    this.topScores = []
    this.metric = metric
    this.renderer = renderer

    for (let idx = 0; idx < num; idx++) {
        this.topScores[idx] = new TopScore(idx + 1, null, null)
    }

    this.add = (payment, old) => {
        name = payment[metric].toUpperCase()

        if (!this.scores[name]) {
            this.scores[name] = {'name': name, 'sats': 0}
        }

        this.scores[name].sats += payment.sats
        this.updateTopScores()
        this.render()
    }

    this.updateTopScores = () => {
        const newTopScores = Object.values(this.scores).sort((a, b) => {
            return b.sats - a.sats
        }).slice(0, num)

        newTopScores.forEach((score, idx) => {
            this.topScores[idx].update(score.name, score.sats)
        })
    }

    this.getTopScores = () => {
        return this.topScores.filter(x => x.name)
    }

    this.escapeHtml = (text) => {
        const div = document.createElement("div")
        div.appendChild(document.createTextNode(text))
        return div.innerHTML
    }

    this.render = () => {
        this.topScores.forEach(score => this.renderer(score))
    }
}