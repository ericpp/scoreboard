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
let newPayments
let producers
let producerScores

let apps
let appScores

const messageTime = 10

const nostrRelays = ["wss://relay.damus.io", "wss://nos.lol", "wss://relay.nostr.band"]
const nostrBoostPkey = "804eeaaf5afc67cae9aa50a6ae03571ae693fcb277bd40d64b966b12dcba25ce"

let nostrPool

const nostrNames = {};
const nostrNameQueue = {};

let pollInterval = 10000

let pew = new Audio('pew.mp3')

// apply podcast filter if specified in page url
let params = (new URL(document.location)).searchParams
let podcast = params.get("podcast")

let nostrZappedEvent = params.get("nostrEvent") || "30311:b9d02cb8fddeb191701ec0648e37ed1f6afba263e0060fc06099a62851d25e04:1712441602"

let after = params.get("after") || "2024-04-06 15:00:00 -0500"
after = new Date(after)
after = Math.floor(after / 1000)

let lastBoostAt = after

function setup(){
    stars = new Stars(numstars)
    producerScores = new Scores(5)
    appScores = new Scores(5)
    newPayments = new NewPayments()
    lastPayment = new LastPayment()
    topCounters = new TopCounters()

    tracker = new Tracker(producerScores, appScores, topCounters, lastPayment, newPayments)
    producers = new Scoreboard("- TOP PRODUCERS -", producerScores)
    apps = new Scoreboard("- TOP APPS -", appScores)

    getBoosts(true).then(() => {
        initNostr(true)
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
    posY = topCounters.draw(boxOffsetWidth, posY, boxWidth)

    if (newPayments.isDrawable()) {
        newPayments.draw(0, posY, windowWidth)
    }
    else {
        posY = lastPayment.draw(0, posY, windowWidth)
        posY = producers.draw(boxOffsetWidth, posY, boxWidth)
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

async function getBoosts(old) {
    let page = 1
    let items = 1000
    let boosts = []

    while (true) {
        const query = new URLSearchParams()
        query.set("page", page)
        query.set("items", items)
        query.set("created_at_gt", lastBoostAt)

        const result = await fetch(`/api/boosts?${query}`)
        const boosts = await result.json()

        if (!boosts || boosts.length === 0) {
            break
        }

        lastBoostAt = Math.max(lastBoostAt, Math.max(...boosts.map(x => x.creation_date)))

        if (podcast) {
            boosts = boosts.filter(x => x.boostagram.podcast == podcast)
        }

        boosts.forEach(invoice => {
            tracker.addBoost(invoice, true)
        })

        page++
    }
}

async function initNostr(old) {
    nostrPool = new NostrTools.SimplePool()

    let isOld = old

    nostrPool.subscribeMany(nostrRelays, [{authors: [nostrBoostPkey]}], {
        onevent(event) {
            invoice = JSON.parse(event.content)

            if (podcast && podcast != invoice.podcast) {
                return
            }

            if (lastBoostAt > invoice.creation_date) {
                return;
            }

            tracker.addBoost(invoice, isOld)
        },
        oneose() {
            // h.close()
        }
    })

    nostrPool.subscribeMany(nostrRelays, [{'#a': [nostrZappedEvent], 'kinds': [9735]}], {
        async onevent(event) {
            if (after > event.created_at) {
                return;
            }

            await tracker.addZap(event, isOld)
        },
        oneose() {
            // h.close()
        }
    })

    setInterval(async () => {
        if (!nostrNameQueue) return

        let pubkeys = Object.keys(nostrNameQueue)
        if (pubkeys.length === 0) return

        let profiles = await nostrPool.querySync(nostrRelays, {authors: pubkeys, kinds: [0]})

        profiles.forEach(event => {
            const profile = JSON.parse(event.content)
            nostrNames[event.pubkey] = profile.display_name || profile.name
        })

        for (let pubkey of pubkeys) {
            let resolvers = nostrNameQueue[pubkey]

            if (resolvers) {
                delete nostrNameQueue[pubkey]

                resolvers.forEach(resolve => {
                    resolve(nostrNames[pubkey] || null)
                })
            }
        }

    }, 1000)

    setTimeout(() => { isOld = false }, 5000)
}

function getNostrName(pubkey) {
    return new Promise((resolve, reject) => {
        if (nostrNames[pubkey]) {
            resolve(nostrNames[pubkey])
        }
        else {
            if (!nostrNameQueue[pubkey]) {
                nostrNameQueue[pubkey] = []
            }

            nostrNameQueue[pubkey].push(resolve)
        }
    })
}

function getMsatsFromBolt11(bolt11) {
    const multipliers = {
        m: 100000000,
        u: 100000,
        n: 100,
        p: 0.1,
    }

    // msat amount encoded in the first part (e.g. lnbc100n -> 100n -> 100 * 100 = 100,000)
    let matches = bolt11.match(/^ln\w+?(\d+)([a-zA-Z]?)/)

    if (!matches) {
        return null // no match
    }

    // calculate the msats from the number and multiplier
    return parseInt(matches[1]) * multipliers[matches[2]]
}


function Tracker(producers, apps, topCounters, lastPayment, newPayments) {
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

        this.producers.add(payment.sender_name || "Unknown", payment.sats)
        this.apps.add(payment.app_name || "Unknown", payment.sats)
        this.lastPayment.add(payment)
        this.topCounters.add(payment)

        if (!old) {
            this.newPayments.add(payment)
        }

        this.identifiers.push(payment.identifier)
    }

    this.addBoost = async (invoice, old) => {
        const boost = invoice.boostagram

        this.add({
            type: 'boost',
            action: boost.action || 'unknown',
            identifier: invoice.identifier,
            creation_date: invoice.creation_date,
            sender_name: boost.sender_name || 'Anonymous',
            app_name: boost.app_name || 'Unknown',
            sats: Math.floor(boost.value_msat_total / 1000),
            message: boost.message,
        }, old)
    }

    this.addZap = async (zap, old) => {
        const tags = zap.tags.reduce((result, tag) => {
            const [name, value] = tag
            if (!result[name]) result[name] = []
            result[name].push(value)
            return result
        }, {})

        // original zap request is encoded as a tag in the zap receipt
        const zaprequest = JSON.parse(tags['description'][0])

        // msats can be calculated from bolt11 request
        const value_msat_total = getMsatsFromBolt11(tags['bolt11'][0])

        // batch look up names based on original zap request pubkey
        const sender_name = await getNostrName(zaprequest.pubkey)

        // push
        this.add({
            type: 'zap',
            action: 'zap',
            identifier: zap.id,
            creation_date: zap.created_at,
            sender_name: sender_name || 'Anonymous',
            app_name: 'Tunestr',
            sats: Math.floor(value_msat_total / 1000),
            message: zap.content,
        }, old)
    }

}

function SatCounter(title) {
    this.title = title
    this.total = 0
    this.drawTotal = 0
    this.increment = 0

    this.add = (sats) => {
        this.total += sats
        this.increment = Math.floor(this.total / 240)
    }

    this.draw = (x, y, width) => {
        const diff = this.total - this.drawTotal

        if (diff > 0) {
            this.drawTotal += Math.min(diff, Math.floor(random(50, this.increment)))
        }

        push()

        fill(255, 0, 0)
        stroke(60, 0, 0)
        textAlign(CENTER)

        y += textSize()
        text(this.title, x, y, width)
        y += textSize()

        fill(255, 255, 255)
        stroke(60, 60, 60)
        text(this.drawTotal.toLocaleString(), x, y, width)
        y += 2 * textSize()

        pop()

        return y
    }
}

function TopCounters() {
    this.boostCounter = new SatCounter('BOOSTS')
    this.zapCounter = new SatCounter('ZAPS')
    this.totalCounter = new SatCounter('TOTAL SATS')

    this.add = (payment) => {
        if (payment.type == 'boost') {
            this.boostCounter.add(payment.sats)
        }
        else if (payment.type == 'zap') {
            this.zapCounter.add(payment.sats)
        }

        this.totalCounter.add(payment.sats)
    }

    this.draw = (x, y, width) => {
        const indent = (width / 3) + 100

        this.boostCounter.draw(x - indent, y, width)
        this.zapCounter.draw(x + indent, y, width)

        return this.totalCounter.draw(x, y, width)
    }
}

function NewPayments() {
    this.pending = []
    this.current = null
    this.updated = 0

    this.add = (boost) => {
        if (boost.action != 'boost' && boost.action != 'zap') {
            return // show only boosts and zaps for now
        }

        pew.play()

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

        const sender = this.current.sender_name
        const sats = this.current.sats.toLocaleString()
        const boostzap = (this.current.type == 'boost' ? 'BOOSTED' : 'ZAPPED')
        const app = (this.current.type == 'boost' ? `FROM ${this.current.app_name}` : '')

        const info = `${sender} ${boostzap} ${sats} SATS ${app}`.toUpperCase()

        if (this.current.message) {
            fill(0, 255, 255)
            stroke(0, 60, 60)
            boxedText(info, x, y, width, 2 * textSize())
            y += 3 * textSize()

            fill(255, 255, 255)
            stroke(60, 60, 60)

            boxedText(this.current.message, x, y, width, windowHeight - y - 200)
        }
        else {
            fill(255, 255, 255)
            stroke(60, 60, 60)
            boxedText(info, x, y, width, windowHeight - y - 200)
        }

        pop()

    }
}

function LastPayment() {
    this.current = null

    this.add = (boost) => {
        if (this.current && this.current.creation_date > boost.creation_date) {
            return
        }

        if (boost.action != 'boost' && boost.action != 'zap') {
            return // show only boosts and zaps for now
        }

        this.current = boost
    }

    this.draw = (x, y, width) => {
        if (!this.current) {
            return y
        }

        push()

        fill(0, 255, 255)
        stroke(0, 60, 60)
        textAlign(CENTER)

        const sender = this.current.sender_name
        const sats = this.current.sats.toLocaleString()
        const boostzap = (this.current.type == 'boost' ? 'BOOSTED' : 'ZAPPED')
        const app = (this.current.type == 'boost' ? `FROM ${this.current.app_name}` : '')

        const info = `${sender} ${boostzap} ${sats} SATS ${app}`.toUpperCase()

        if (this.current.message) {
            boxedText(info, x, y, width, textSize())
            y += 1.25 * textSize()

            boxedText(this.current.message.toUpperCase(), x, y, width, 2 * textSize())
            y += 3 * textSize()
        }
        else {
            boxedText(info, x, y, width, 2 * textSize())
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

        textAlign(RIGHT)
        fill(0, 255, 255)
        stroke(0, 60, 60)
        text("SCORE", x + (width/2.5), y, 0)
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

        textAlign(RIGHT)
        text(this.sats.toLocaleString(), x + (width/2.5), y, 0)
        text(this.name.toUpperCase(), x, y, width)

        y += 1.25 * textSize()

        pop()

        return y
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

function bech32_to_hex(str) {
    let grammar = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"
    let idx = 0;
    let hex = "";

    // remove prefix
    str = str.replace(/^\w+1(.+)$/, '$1')

    // remove checksum (to-do later)
    str = str.replace(/^(.+?).{6}$/, '$1')

    // convert to hex
    while (idx < str.length) {
        let word = 0;
console.log(idx, str[idx], str[idx+1], str[idx+2], str[idx+3])
        word += grammar.indexOf(str[idx++]) << 15
        word += grammar.indexOf(str[idx++]) << 10
        word += grammar.indexOf(str[idx++]) << 5
        word += grammar.indexOf(str[idx++])
        hex += word.toString(16);
console.log(word, word.toString(16), hex)
    }

    return hex;
}
