let font

let heightToFont = 28
let fontSize = 60

let boxWidth = 600
let boxHeight = 600

let boxOffsetWidth
let boxOffsetHeight

let stars
let numstars = 50

let scoreTracker
let paymentTracker
let newPayments
let producers
let producerScores

let apps
let appScores

const messageTime = 8

let pollInterval = 10000

const nostrBoostPkey = "804eeaaf5afc67cae9aa50a6ae03571ae693fcb277bd40d64b966b12dcba25ce"
const nostrZapEvent = "30311:b9d02cb8fddeb191701ec0648e37ed1f6afba263e0060fc06099a62851d25e04:1712441602"
const excludePodcasts = ["Podcasting 2.0", "Bands at Bitcoin", "Pew Pew", "Day 3", "Day 2", "Day 1", "Homegrown"]

const backgroundColor = [0, 0, 64]

const headerColor = [255, 0, 255]
const headerShadow = [60, 0, 60]

const textColor = [0, 255, 255]
const textShadow = [0, 60, 60]

const counterColor = [255, 255, 0]
const counterShadow = [60, 60, 0]

function setup(){
    stars = new Stars(numstars)
    // producerScores = new Scores(5)
    // appScores = new Scores(5)
    newPayments = new NewPayments('.new-boost')
    lastPayment = new LastPayment('.last-boost')
    topCounters = new TopCounters()
    boostsList = new BoostsList()

    producers = new Scoreboard('- TOP PRODUCERS -', '.top-producers')
    apps = new Scoreboard('- TOP APPS -', '.top-apps')

    let params = (new URL(document.location)).searchParams

    paymentTracker = new PaymentTracker()

    paymentTracker.setNostrBoostPkey(nostrBoostPkey)
    paymentTracker.setNostrZapEvent(params.get("nostrEvent") || nostrZapEvent)

    // apply filters specified in page url
    // paymentTracker.setFilter("podcast", params.get("podcast"))
    paymentTracker.setFilter("after", params.get("after") || "2024-08-28 00:00:00 -0500")
    paymentTracker.setFilter("before", params.get("before") || "2024-09-02 00:00:00 -0500")

    paymentTracker.setFilter("podcasts", ["NFT MSP | CULTURE CONVERGENCE 2024"])
    // paymentTracker.setFilter("excludePodcasts", excludePodcasts)

    scoreTracker = new ScoreTracker(producers, apps, topCounters, lastPayment, newPayments)

    paymentTracker.setListener((payment, old) => {
        scoreTracker.add(payment, old)
        boostsList.add(payment)
    })

    paymentTracker.start()


    fontSize = windowHeight / heightToFont
    boxWidth = windowWidth / 1.5

    boxOffsetWidth = Math.floor((windowWidth - boxWidth) / 2)
    boxOffsetHeight = 0 // Math.floor((windowHeight - boxHeight) / 2)

    createCanvas(windowWidth, windowHeight)
}

function windowResized() {
    fontSize = windowHeight / heightToFont
    boxWidth = windowWidth / 1.5

    boxOffsetWidth = Math.floor((windowWidth - boxWidth) / 2)
    boxOffsetHeight = 0// Math.floor((windowHeight - boxHeight) / 2)

    resizeCanvas(windowWidth, windowHeight)

    stars.reset()
}

function draw(){
    background(...backgroundColor)
    stars.draw()
}

function boxedText(str, x, y, width, height) {
    // calculate perfect font size to fit text based on available 2d space (width * height)
    const fontSize = Math.min(Math.sqrt((width * height) / (1.5 * str.length)), height)

    push()
    textSize(fontSize)
    text(str, x, y, width)
    pop()
}

function ScoreTracker(producers, apps, topCounters, lastPayment, newPayments) {
    this.producers = producers
    this.apps = apps
    this.topCounters = topCounters
    this.lastPayment = lastPayment
    this.newPayments = newPayments
    this.identifiers = []

    this.add = (payment, old) => {
        if (!payment.sats || isNaN(payment.sats)) {
            return;
        }

        if (this.identifiers.indexOf(payment.identifier) !== -1) {
            return
        }

        this.producers.add(payment.sender_name, payment.sats)
        // this.apps.add(payment.app_name || "Unknown", payment.sats)
        // this.apps.add(payment.podcast.substr(0, 20), payment.sats)
        this.apps.add(payment.app_name || "Unknown", payment.sats)
        this.lastPayment.add(payment)
        this.topCounters.add(payment)

        if (!old) {
            this.newPayments.add(payment)
        }

        this.identifiers.push(payment.identifier)
    }
}

function SatCounter(title, el) {
    this.title = title
    this.total = 0
    this.drawTotal = 0
    this.increment = 0

    this.el = document.querySelector(el)
    this.el.querySelector('.figure').innerText = '0'

    this.countTimer = setInterval(() => {
        this.render()
    }, 10)

    this.add = (sats) => {
        this.total += sats
        this.increment = Math.floor(this.total / 240)
    }

    this.render = (container) => {
        const diff = this.total - this.drawTotal

        if (diff == 0) {
            return;
        }

        if (diff > 0) {
            this.drawTotal += Math.min(diff, Math.floor(random(50, this.increment)))
        }

        this.el.querySelector('.figure').innerText = this.drawTotal.toLocaleString()
    }
}

function TopCounters() {
    this.el = document.querySelector('.top-scores')

    this.boostCounter = new SatCounter('BOOSTS', '.top-boosts')
    this.boostCounter.render(this.el)

    this.totalCounter = new SatCounter('TOTAL SATS', '.total-sats')
    this.totalCounter.render(this.el)

    this.zapCounter = new SatCounter('ZAPS', '.top-zaps')
    this.zapCounter.render(this.el)

    this.add = (payment) => {
        if (payment.type == 'boost') {
            this.boostCounter.add(payment.sats)
        }
        else if (payment.type == 'zap') {
            this.zapCounter.add(payment.sats)
        }

        this.totalCounter.add(payment.sats)
    }
}

function NewPayments(el) {
    this.pending = []
    this.current = null
    this.updated = 0
    this.el = document.querySelector(el)

    this.timer = setInterval(() => {
        if (this.pending.length == 0) {
            document.querySelector('.board').style.display = ''
            document.querySelector('.new-boost').style.display = 'none'
            return
        }

        document.querySelector('.board').style.display = 'none'
        document.querySelector('.new-boost').style.display = ''

        this.current = this.pending.shift()
        this.render()

    }, 5000)

    this.add = (boost) => {
        if (boost.action != 'boost' && boost.action != 'zap') {
            return // show only boosts and zaps for now
        }

        this.pending.push(boost)
    }

    this.render = () => {
        const sender = this.current.sender_name
        const sats = this.current.sats.toLocaleString()
        const boostzap = (this.current.type == 'boost' ? 'BOOSTED' : 'ZAPPED')
        const app = (this.current.type == 'boost' ? `FROM ${this.current.app_name}` : '')

        const info = `${sender} ${boostzap} ${sats} SATS ${app}`.toUpperCase()

        if (this.current.message) {
            this.el.querySelector('.title').style.display = ''
            this.el.querySelector('.title').innerText = info
            this.el.querySelector('.message').innerText = this.current.message
        }
        else {
            this.el.querySelector('.title').style.display = 'none'
            this.el.querySelector('.message').innerText = info
        }
    }
}

function LastPayment(el) {
    this.current = null
    this.el = document.querySelector(el)

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

        const info = `${sender} ${boostzap} ${sats} SATS ${app}`.toUpperCase()

        if (this.current.message) {
            this.el.querySelector('.booster').innerText = info
            this.el.querySelector('.message').innerText = this.current.message
            this.el.querySelector('.message').style.display = ''
        }
        else {
            this.el.querySelector('.booster').innerText = info
            this.el.querySelector('.message').innerText = ''
            this.el.querySelector('.message').style.display = 'none'
        }
    }
}

function Scores(num) {
    this.scores = {}
    this.topScores = []

    for (let idx = 0; idx < num; idx++) {
        this.topScores[idx] = new TopScore(idx + 1, null, null)
    }

    this.add = (name, sats) => {
        name = name.toUpperCase()

        if (!this.scores[name]) {
            this.scores[name] = {'name': name, 'sats': 0}
        }

        this.scores[name].sats += sats
        this.updateTopScores()
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
}

function Scoreboard(title, el) {
    this.title = title
    this.updated = {}
    this.lastTopScores = []
    this.scores = new Scores(5)

    this.el = document.querySelector(el)
    this.scoreRows = {}

    this.add = (name, sats) => {
        this.scores.add(name, sats)
        this.render()
    }

    this.render = () => {
        const topScores = this.scores.getTopScores()

        for (let idx = 0; idx < topScores.length; idx++) {
            if (!this.scoreRows[idx]) {
                const el = document.createElement('tr')
                el.innerHTML = `
                <td class="place visible-sm"></td>
                <td class="name truncate"></td>
                <td class="score" style="text-align: right;"></td>
                `
                this.scoreRows[idx] = this.el.querySelector('.scores').appendChild(el)
            }

            y = topScores[idx].render(this.scoreRows[idx])
        }
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

    this.render = (el) => {
        let place = ''
        if (this.position == 1) place = '1ST'
        else if (this.position == 2) place = '2ND'
        else if (this.position == 3) place = '3RD'
        else place = (this.position) + 'TH'

        el.querySelector('.place').innerText = place
        el.querySelector('.score').innerText = this.sats.toLocaleString()
        el.querySelector('.name').innerText = this.name.toUpperCase()
    }
}

function Stars(numstars) {
    this.stars = []
    this.numstars = numstars

    this.reset = () => {
        this.stars = []
    }

    this.draw = () => {
        for (let idx = 0; this.stars.length < this.numstars; idx++) {
            this.stars.push(new Star())
        }

        for (let idx = 0; idx < this.stars.length; idx++){
            this.stars[idx].draw()

            if (this.stars[idx].isOffScreen()){
                this.stars[idx] = new Star(0)
            }
        }
    }
}

function Star(top){
    // this.pos = createVector(random(width), top === undefined ? random(height) : top)
    this.pos = createVector(random(width), top === 0 ? 0 : random(height))
    this.vel = createVector(0, 2)
    this.pick = random(360)
    this.alpha = Math.round(random(4)) / 4
    this.size = 3

    this.draw = () => {
        this.pos.add(this.vel)

        push()
        strokeWeight(0)
        colorMode(HSB)

        this.pick = Math.floor((this.pick + 1) % 360)

        fill(this.pick, 100, 100, this.alpha)
        rect(this.pos.x, this.pos.y, this.size, this.size)

        colorMode(RGB)
        pop()
    }

    this.isOffScreen = function(){
        return (this.pos.y >= height)
    }
}

function BoostsList() {
    this.el = document.getElementById('list')

    document.querySelectorAll('.toggle-list').forEach(toggle => {
        toggle.addEventListener('click', () => {
            const div = document.querySelector('.boost-list')
            div.style.display = (div.style.display == 'none' ? '' : 'none')
        })
    })

    this.add = (boost) => {
        let beforeItem = null

        for (let item of this.el.children) {
            if (boost.creation_date > item.getAttribute('data-timestamp')) {
              beforeItem = item
              break
            }
        }

        const boostzap = (boost.type == 'boost' ? 'boosted' : 'zapped')

        let message = `${boost.sender_name} ${boostzap} ${boost.sats} sats from ${boost.app_name}`

        if (boost.message) {
          message += ` saying "${boost.message}"`
        }

        let date = new Date(boost.creation_date * 1000)

        description = `${date.toLocaleString()} - ${boost.podcast}`

        const li = document.createElement("li")
        li.classList.add("added")
        li.setAttribute('data-timestamp', boost.creation_date)
        li.appendChild(document.createTextNode(message))
        li.appendChild(document.createElement('br'))

        const small = document.createElement("small")
        small.appendChild(document.createTextNode(description))
        li.appendChild(small)

        if (beforeItem) {
            list.insertBefore(li, beforeItem)
        }
        else {
            list.appendChild(li)
        }

        li.addEventListener("click", () => {
            [...list.children].forEach((item) => {
                item.classList.remove("selected")
            })

            li.classList.add("selected")
        })

        setTimeout(() => li.classList.remove("added"), 1)
    }

}