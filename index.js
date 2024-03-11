let font

let heightToFont = 28
let fontSize = 60

let boxWidth = 600
let boxHeight = 600

let boxOffsetWidth
let boxOffsetHeight

let stars
let numstars = 50

let tracker
let newBoosts
let boosters
let boosterScores

let apps
let appScores

const messageTime = 15

let lightning

// start at midnight of current date
let lastBoostAt = new Date('2024-03-10 00:00:00 -0500')
// lastBoostAt.setHours(0)
// lastBoostAt.setMinutes(0)
// lastBoostAt.setSeconds(0)
lastBoostAt = Math.floor(lastBoostAt / 1000)

let lastInvoiceId = null

let pollInterval = 20000

let pew = new Audio('pew.mp3')

async function getBoosts(old) {
    let page = 1
    let items = 25
    let boosts = []

    // apply podcast filter if specified in page url
    let params = (new URL(document.location)).searchParams;
    let podcast = params.get("podcast");

    for (let idx = 0; idx < 10; idx++) { // safety for now
        const query = new URLSearchParams()
        query.set("page", page)
        query.set("items", items)

        if (lastInvoiceId) {
            query.set("since", lastInvoiceId)
        }
        else {
            query.set("created_at_gt", lastBoostAt)
        }

        const result = await fetch(`/api/boosts?${query}`)
        const transactions = await result.json()

        boosts = [...boosts, ...transactions]

        if (!transactions || transactions.length === 0 || transactions.length < items) {
            break
        }

        page++
    }

    if (podcast) {
        boosts = boosts.filter(x => x.boostagram.podcast == podcast)
    }

    boosts.sort((a, b) => a.creation_date - b.creation_date)

    boosts.forEach(boost => {
        lastBoostAt = boost.creation_date
        lastInvoiceId = boost.identifier
        tracker.addBoost(boost.boostagram, old)
    })
}

function setup(){
    stars = new Stars(numstars)
    boosterScores = new Scores(5)
    appScores = new Scores(5)
    newBoosts = new NewBoosts()
    lastBoost = new LastBoost()
    totalSats = new TotalSats()

    tracker = new Tracker(boosterScores, appScores, totalSats, lastBoost, newBoosts)
    boosters = new Scoreboard("- TOP BOOSTERS -", boosterScores)
    apps = new Scoreboard("- TOP APPS -", appScores)

    getBoosts(true).then(() => {
        setInterval(() => getBoosts(), pollInterval)
    })

    fontSize = windowHeight / heightToFont
    boxWidth = windowWidth / 1.75

    boxOffsetWidth = Math.floor((windowWidth - boxWidth) / 2)
    boxOffsetHeight = 0 // Math.floor((windowHeight - boxHeight) / 2)

    createCanvas(windowWidth, windowHeight)
}

function windowResized() {
    fontSize = windowHeight / heightToFont
    boxWidth = windowWidth / 1.75

    boxOffsetWidth = Math.floor((windowWidth - boxWidth) / 2)
    boxOffsetHeight = 0// Math.floor((windowHeight - boxHeight) / 2)

    resizeCanvas(windowWidth, windowHeight)

    stars.reset()
}

function draw(){
    background(0)

    stars.draw()

    strokeWeight(8)

    textSize(fontSize)
    textFont('Joystix')
    textAlign(CENTER)

    posY = boxOffsetHeight
    posY = totalSats.draw(0, posY, windowWidth)

    if (newBoosts.isDrawable()) {
        newBoosts.draw(0, posY, windowWidth)
    }
    else {
        posY = lastBoost.draw(0, posY, windowWidth)
        posY = boosters.draw(boxOffsetWidth, posY, boxWidth)
        posY = apps.draw(boxOffsetWidth, posY, boxWidth)
    }
}

function boxedText(str, x, y, width, height) {
    // calculate perfect font size to fit text based on available 2d space (width * height)
    const fontSize = Math.min(Math.sqrt((width * height) / (1.5 * str.length)), height)

    push()
    textSize(fontSize)
    text(str, x, y, width)
    pop()
}

function Tracker(boosters, apps, totalSats, lastBoost, newBoosts) {
    this.boosters = boosters
    this.apps = apps
    this.totalSats = totalSats
    this.lastBoost = lastBoost
    this.newBoosts = newBoosts 

    this.addBoost = (boost, old) => {
        const sats = boost.value_msat_total / 1000
        this.boosters.add(boost.sender_name, sats)
        this.apps.add(boost.app_name, sats)
        this.totalSats.add(sats)
        this.lastBoost.add(boost)

        if (!old) {
            this.newBoosts.add(boost)
            pew.play()
        }
    }
}

function TotalSats() {
    this.total = 0
    this.drawTotal = 0

    this.add = (sats) => {
        this.total += sats
    }

    this.draw = (x, y, width) => {
        const diff = this.total - this.drawTotal

        if (diff > 0) {
            this.drawTotal += Math.min(diff, Math.floor(random(50, 200)))
        }

        push()

        fill(255, 0, 0)
        stroke(60, 0, 0)
        textAlign(CENTER)

        y += textSize()
        text("TOTAL SATS", x, y, width)
        y += textSize()

        fill(255, 255, 255)
        stroke(60, 60, 60)
        text(this.drawTotal.toLocaleString(), x, y, width)
        y += 2 * textSize()

        pop()

        return y
    }
}

function NewBoosts() {
    this.pending = []
    this.current = null
    this.updated = 0

    this.add = (boost) => {
        this.pending.push(boost)
    }

    this.isDrawable = () => {
        return (this.updated > 0 || this.pending.length > 0)
    }

    this.draw = (x, y, width) => {
        if (this.updated === 0 && this.pending.length === 0) {
            return
        }

        if (this.updated === 0) {
            this.current = this.pending.shift()
            this.updated = Math.floor(frameRate()) * messageTime
        }

        if(this.updated > 0) {
            this.updated--
        }

        push()
        strokeWeight(8)

        textAlign(CENTER)

        let sats = (this.current.value_msat_total / 1000).toLocaleString()
        let str = `${this.current.sender_name} BOOSTED ${sats.toLocaleString()} SATS FROM ${this.current.app_name}`.toUpperCase()

        if (this.current.message) {
            fill(0, 255, 255)
            stroke(0, 60, 60)
            boxedText(str, x, y, width, 2 * textSize())
            y += 3 * textSize()

            fill(255, 255, 255)
            stroke(60, 60, 60)

            boxedText(this.current.message, x, y, width, windowHeight - y - 200)
        }
        else {
            fill(255, 255, 255)
            stroke(60, 60, 60)
            boxedText(str, x, y, width, windowHeight - y - 200)
        }

        pop()

    }
}

function LastBoost() {
    this.boost = null

    this.add = (boost) => {
        this.boost = boost
    }

    this.draw = (x, y, width) => {
        if (!this.boost) {
            return y
        }

        push()

        fill(0, 255, 255)
        stroke(0, 60, 60)
        textAlign(CENTER)

        let sats = (this.boost.value_msat_total / 1000).toLocaleString()
        let str = `${this.boost.sender_name} BOOSTED ${sats} SATS FROM ${this.boost.app_name}`.toUpperCase()

        if (this.boost.message) {
            boxedText(str, x, y, width, textSize())
            y += 1.25 * textSize()

            boxedText(this.boost.message.toUpperCase(), x, y, width, 2 * textSize())
            y += 3 * textSize()
        }
        else {
            boxedText(str, x, y, width, 2 * textSize())
            y += 3 * textSize()
        }

        pop()

        return y
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

function Scoreboard(title, scores) {
    this.title = title
    this.scores = scores
    this.updated = {}
    this.lastTopScores = []

    this.draw = function(x, y, width) {
        push()

        textAlign(CENTER)
        fill(255, 0, 0)
        stroke(60, 0, 0)

        text(this.title, 0, y, windowWidth)
        y +=  1.5 * fontSize

        textAlign(LEFT)
        fill(0, 255, 255)
        stroke(0, 60, 60)
        text("SCORE", x + width/2 - 200, y, width)

        textAlign(RIGHT)
        text("NAME", x, y, width)
        y += 1.5 * fontSize

        const topScores = this.scores.getTopScores()

        for (let idx = 0; idx < topScores.length; idx++) {
            y = topScores[idx].draw(x, y, width)
        }

        y += fontSize

        pop()

        return y
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

    this.draw = (x, y, width) => {
        push()

        fill(0, 255, 255)
        stroke(0, 60, 60)

        if (this.updated > 0) {
            if (Math.floor(this.updated / 10) % 2 === 0) {
                fill(255, 255, 255)
                stroke(60, 60, 60)
            }

            this.updated--
        }

        let place = ""
        if (this.position == 1) place = '1ST'
        else if (this.position == 2) place = '2ND'
        else if (this.position == 3) place = '3RD'
        else place = (this.position) + 'TH'

        textAlign(LEFT)
        text(place, x, y, width)
        text(this.sats.toLocaleString(), x + width/2 - 200, y, width)

        textAlign(RIGHT)
        text(this.name.toUpperCase(), x, y, width)

        y += 1.25 * textSize()

        pop()

        return y
    }
}