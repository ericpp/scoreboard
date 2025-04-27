function escapeHtml(text) {
  const span = document.createElement("span")
  span.textContent = text
  return span.innerHTML
}

class AlertMedia {
  playing = false
  duration = 10
  currentTime = 0
  timeHandlers = []

  constructor(element) {
    this.element = element
  }

  setUrl(url) {
    this.element.src = url
  }

  onTimeUpdate(handler) {
    this.timeHandlers.push(handler)
  }
}

class AlertImage extends AlertMedia {
  constructor(element) {
    super(element)
    this.setTimeInterval()
  }

  setTimeInterval() {
    const tickRate = 0.1

    return setInterval(() => {
      if (!this.playing) return

      this.currentTime = this.currentTime + tickRate
      this.timeHandlers.forEach(handler => handler(this.currentTime))
    }, tickRate * 1000)
  }

  play() {
    this.playing = true
    this.currentTime = 0.0

    setTimeout(() => {
      this.playing = false
    }, this.duration * 1000)
  }
}

class AlertVideo extends AlertMedia {
  constructor(element) {
    super(element)
    this.element.addEventListener("play", () => this.playing = true)
    this.element.addEventListener("ended", () => this.playing = false)
  }

  onTimeUpdate(handler) {
    this.element.addEventListener("timeupdate", () => handler(this.element.currentTime))
  }

  play() {
    this.element.play()
  }
}

class AlertSlot {
  constructor(rootId, options = {}) {
    this.root = document.querySelector(rootId)
    this.messageRoot = this.root.querySelector(".message")
    this.rootShown = false
    this.messageShown = false

    const mediaElement = this.root.querySelector(".background")
    this.alert = mediaElement.tagName === "IMG" ? new AlertImage(mediaElement) : new AlertVideo(mediaElement)

    this.images = options.images || []
    this.triggers = options.triggers || []
    this.events = options.events || {}
    this.message = options.message || {}
    this.showMessages = options.showMessages || false
    this.payment = null
    this.selectedImage = null
    this.lastSatTotal = 0

    const timeShow = parseFloat(this.messageRoot.getAttribute("data-timeshow"))
    const timeHide = parseFloat(this.messageRoot.getAttribute("data-timehide"))

    this.message.timeShow = isNaN(timeShow) ? 0.0 : timeShow
    this.message.timeHide = isNaN(timeHide) ? null : timeHide

    this.alert.onTimeUpdate(this.handleTimeUpdate.bind(this))
  }

  pickImage(payment) {
    if (this.triggers.length) {
      const strSats = String(payment.sats)

      const img = this.triggers.find(trig =>
        (trig.threshold && payment.satTotal >= trig.threshold && payment.lastSatTotal < trig.threshold) ||
        (trig.endsWith && strSats.endsWith(trig.endsWith)) ||
        (trig.contains && strSats.includes(trig.contains))
      )

      this.alert.setUrl(img?.src)
    }
    else if (this.images.length) {
      this.selectedImage = Math.floor(Math.random() * this.images.length)
      this.alert.setUrl(this.images[this.selectedImage])
    }
  }

  loadPicture(src) {
    return new Promise((resolve, reject) => {
      const img = new Image()
      img.onload = () => resolve(img)
      img.onerror = reject
      img.src = src
      img.style.height = "127px"
      img.style.width = "auto"
    })
  }

  async setProfilePicture(src) {
    const box = this.messageRoot.querySelector(".picture")
    if (!box) return

    box.innerHTML = ""
    if (src) {
      box.appendChild(await this.loadPicture(src))
    }
  }

  async setMessage(lines, picture) {
    const text = this.messageRoot.querySelector(".messageText")
    text.innerHTML = lines.join("<br>")

    await this.setProfilePicture(picture)

    this.messageRoot.style.paddingTop = 0
    this.messageRoot.style.paddingBottom = 0

    if (this.selectedImage !== null && this.message.offsetY?.[this.selectedImage]) {
      const offset = this.message.offsetY[this.selectedImage]
      if (offset > 0) {
        this.messageRoot.style.paddingTop = offset
      } else if (offset < 0) {
        this.messageRoot.style.paddingBottom = -offset
      }
    }
  }

  handleTimeUpdate(time) {
    const messageHide = this.message.timeHide || this.alert.duration - 2

    if (!this.messageShown && time >= this.message.timeShow && time < messageHide) {
      this.messageRoot.classList.add("show")
      this.messageShown = true
      this.events.messageShow?.(this.payment)
    }

    if (this.messageShown && time >= messageHide) {
      this.messageRoot.classList.remove("show")
      this.messageShown = false
      this.events.messageHide?.(this.payment)
    }

    if (this.rootShown && time >= this.alert.duration) {
      this.root.classList.remove("show")
      this.rootShown = false
      this.events.hide?.(this.payment)
    }
  }

  playing() {
    return this.alert.playing
  }

  renderMessage(payment, renderer) {
    if (renderer) {
      return renderer(payment)
    }

    const sats = payment.sats.toLocaleString()
    const userMessage = this.showMessages
      ? payment.message
      : payment.remote_feed
        ? `${payment.remote_feed} - ${payment.remote_item}`
        : ""

    return [
      `${escapeHtml(sats)} sat ${escapeHtml(payment.type)} from ${escapeHtml(payment.sender_name)}`,
      escapeHtml(userMessage),
    ]
  }

  play() {
    this.root.classList.add("show")
    this.rootShown = true
    this.events.show?.(this.payment)
    this.alert.play()
  }

  show(payment) {
    this.payment = payment
    this.pickImage(payment)
    this.setMessage(
      this.renderMessage(this.payment, this.events.messageRender),
      payment.picture
    )
    this.handleTimeUpdate(0)
    this.play()

    console.log(
      payment.sats,
      payment.satTotal,
      payment.lastSatTotal,
      payment.sender_name,
    )
  }
}

async function startAlerts(config = {}) {
  // Set defaults
  config = {
    loadBoosts: false,
    loadZaps: false,
    showMessages: false,
    nostrBoostPkey: "npub1sp8w4t66l3nu46d22zn2uq6hrtnf8l9jw775p4jtje439h96yh8qzmpw6q",
    excludePodcasts: ["Podcasting 2.0"],
    slots: [{"id": "#alert"}],
    ...config
  }

  const app = new PaymentTracker(config)
  const url = getUrlConfig(document.location)

  const slots = config.slots.map(slot => new AlertSlot(slot.id, slot))
  let curSlot = 0

  const alertQueue = []

  let satTotal = 0
  let lastSatTotal = 0
  let paymentCounter = 0

  function paymentReceived(payment) {
    satTotal = satTotal + payment.sats

    if (payment.action !== 'boost' && payment.type !== 'zap') {
      return // filter out streams
    }

    paymentCounter++

    if (config.priority !== undefined && config.priority !== (paymentCounter % config.numalerts)) {
      return // show only a portion of payments based on priority/numalerts
    }

    payment.satTotal = satTotal
    payment.lastSatTotal = lastSatTotal

    lastSatTotal = payment.satTotal

    if (payment.isOld && !url.test) {
      return
    }

    alertQueue.push(payment)
  }

  function getAvailableSlots() {
    const availableSlots = slots.filter(x => !x.playing())

    if (availableSlots.length === 0) {
      return [] // none available
    }

    if (config.activeSlots !== undefined && config.activeSlots <= slots.length - availableSlots.length) {
      return [] // too many slots taken
    }

    return availableSlots
  }

  function selectSlot(availableSlots) {
    if (config.randomizeSlots !== false) {
      return {
        slot: availableSlots[Math.floor(Math.random() * availableSlots.length)],
        newSlot: curSlot
      }
    }

    const slot = availableSlots[curSlot]
    const newSlot = (curSlot + 1) % slots.length

    return { slot, newSlot }
  }

  function handleAlertQueue() {
    if (alertQueue.length === 0) return

    const availableSlots = getAvailableSlots()
    if (availableSlots.length === 0) return

    const { slot, newSlot } = selectSlot(availableSlots)
    curSlot = newSlot
    slot.show(alertQueue.shift())
  }

  function init() {
    if (url.before) app.setFilter("before", url.before)
    if (url.after) app.setFilter("after", url.after)
    if (url.naddr) app.setNostrZapEvent(url.naddr)

    if (url.loadBoosts) app.loadBoosts = true
    if (url.loadZaps) app.loadZaps = true

    if (url.test) {
      paymentReceived({
        "action": "boost",
        "app_name": "CurioCaster",
        "creation_date": 1745014628,
        "episode": "Trailer",
        "episode_guid": "35a27eb9-9342-4387-8c3b-3b19f73418b3",
        "event_guid": null,
        "identifier": "WYDoAEsvQwgi4AEwKgKKjbGh",
        "isOld": true,
        "lastSatTotal": 0,
        "message": "test toast?",
        "podcast": "The Satellite Spotlight",
        "satTotal": parseInt(url.test),
        "sats": parseInt(url.test),
        "sender_name": "Anonymous PodcastGuru User",
        "type": "boost"
      })

      app.loadBoosts = true
      app.loadZaps = true
    }

    setInterval(handleAlertQueue, 1000)

    app.setListener(paymentReceived)
    app.start()
  }

  init()
}
