function escapeHtml(text) {
  const span = document.createElement("span")
  span.textContent = text
  return span.innerHTML
}

class AlertImage {
  playing = false
  duration = 10
  currentTime = 0
  timeHandlers = []

  constructor(image) {
    this.image = image
    this.setTimeInterval()
  }

  setTimeInterval() {
    const tickRate = 0.1

    return setInterval(() => {
      if (!this.playing) return

      this.currentTime = this.currentTime + tickRate
      this.timeHandlers.forEach(handle => handle(this.currentTime))
    }, tickRate * 1000)
  }

  setUrl(url) {
    this.image.src = url
  }

  onTimeUpdate(handler) {
    this.timeHandlers.push(handler)
  }

  play() {
    this.playing = true
    this.currentTime = 0.0

    setTimeout(() => {
      this.playing = false
    }, this.duration * 1000)
  }
}

class AlertVideo extends AlertImage {
  timedEvents = []

  constructor(image) {
    super(image)

    this.image.addEventListener("play", () => this.playing = true)
    this.image.addEventListener("ended", () => this.playing = false)
  }

  handleTimeUpdate({ target }) {
    const currentTime = Math.floor(target.currentTime * 1000)
    const timedEvent = this.timedEvents[0]

    if (timedEvent && timedEvent.time <= currentTime) {
      this.timedEvents.push(this.timedEvents.shift())
      timedEvent.callback()
    }
  }

  setUrl(url) {
    this.image.src = url
  }

  onTimeUpdate(handler) {
    this.image.addEventListener("timeupdate", () => handler(this.image.currentTime))
  }

  play() {
    this.image.play()
  }
}

class AlertSlot {
  root = null
  rootShown = false

  messageRoot = null
  alert = null

  images = []
  selectedImage = null

  message = null
  messageShown = false

  constructor(rootId, options) {
    options = options || {}

    this.root = document.querySelector(rootId)
    this.messageRoot = this.root.querySelector(".message")

    this.alert = getAlertObject(this.root.querySelector(".background"))
    this.images = options.images || null

    this.message = options.message || {}

    const timeShow = parseFloat(this.messageRoot.getAttribute("data-timeshow"))
    const timeHide = parseFloat(this.messageRoot.getAttribute("data-timehide"))

    this.message.timeShow = isNaN(timeShow) ? 0.0 : timeShow;
    this.message.timeHide = isNaN(timeHide) ? null : timeHide;

    this.alert.onTimeUpdate(this.handleTimeUpdate.bind(this))
  }

  pickImage() {
    if (this.images) {
      this.selectedImage = Math.floor(Math.random() * this.images.length)
      this.alert.setUrl(this.images[this.selectedImage])
    }
  }

  async setMessage(message, line2, picture) {
    let textContent = escapeHtml(message)

    if (line2) {
      text.innerHTML += '<br>' + escapeHtml(line2)
    }

    await this.setHtmlMessage(message, picture)
  }

  async setHtmlMessage(message, picture) {
    const text = this.messageRoot.querySelector(".messageText")
    text.innerHTML = message

    const box = this.messageRoot.querySelector(".picture")
    box.innerHTML = ""

    if (picture) {
      const img = await this.loadPicture(picture)
      box.appendChild(img)
    }

    this.messageRoot.style.paddingTop = 0
    this.messageRoot.style.paddingBottom = 0

    if (this.selectedImage !== null && this.message.offsetY[this.selectedImage] > 0) {
      this.messageRoot.style.paddingTop = this.message.offsetY[this.selectedImage]
    }

    if (this.selectedImage !== null && this.message.offsetY[this.selectedImage] < 0) {
      this.messageRoot.style.paddingBottom = -this.message.offsetY[this.selectedImage]
    }
  }

  loadPicture(src) {
    return new Promise((resolve, reject) => {
      let img = new Image()
      img.onload = () => resolve(img)
      img.onerror = reject
      img.src = src
      img.style.height = "127px"
      img.style.width = "auto"
    })
  }

  handleTimeUpdate(time) {
    const messageHide = (this.message.timeHide || this.alert.duration - 2)

    if (!this.messageShown && time >= this.message.timeShow && time < messageHide) {
      this.messageRoot.classList.add("show")
      this.messageShown = true
    }

    if (this.messageShown && time >= messageHide) {
      this.messageRoot.classList.remove("show")
      this.messageShown = false
    }

    if (this.rootShown && time >= this.alert.duration - 2) {
      this.root.classList.remove("show")
      this.rootShown = false
    }
  }

  playing() {
    return this.alert.playing
  }

  play() {
    this.root.classList.add("show")
    this.rootShown = true
    this.alert.play()
  }

  showMessage(message, line2, picture) {
    this.pickImage()
    this.setMessage(message, line2, picture)
    this.handleTimeUpdate(0)
    this.play()
  }
}

function getAlertObject(image) {
  if (image.tagName == "IMG") {
    return new AlertImage(image)
  }

  return new AlertVideo(image)
}

async function startAlerts(config) {
  config = config || {}

  config.loadBoosts = config.loadBoosts || false
  config.loadZaps = config.loadZaps || false
  config.showMessages = config.showMessages || false

  config.nostrBoostPkey = config.nostrBoostPkey || "npub1sp8w4t66l3nu46d22zn2uq6hrtnf8l9jw775p4jtje439h96yh8qzmpw6q"
  config.excludePodcasts = config.excludePodcasts || ["Podcasting 2.0"]

  const app = new PaymentTracker(config)

  const slots = (config.slots || []).map(slot => new AlertSlot(slot.id, slot))
  let curSlot = 0

  if (slots.length === 0) {
    slots.push(new AlertSlot("#alert"))
  }

  const alertQueue = []
  let paymentCounter = 0;

  const init = () => {
    const url = getUrlConfig(document.location)

    if (url.before) {
      app.setFilter("before", url.before)
    }

    if (url.after) {
      app.setFilter("after", url.after)
    }

    if (url.naddr) {
      app.setNostrZapEvent(url.naddr)
    }

    if (url.loadBoosts) {
      app.loadBoosts = true
    }

    if (url.loadZaps) {
      app.loadZaps = true
    }

    if (url.showMessages !== undefined) {
      config.showMessages = url.showMessages
    }

    if (url.test) {
      alertQueue.push({
        sender_name: "Anonymous PodcastGuru User",
        sats: 3333,
        app_name: "Test App",
        type: "boost",
      })

      app.loadBoosts = true
      app.loadZaps = true
    }

    setInterval(handleAlertQueue, 1000)

    app.setListener(paymentReceived)
    app.start()
  }

  const paymentReceived = (payment, old) => {
    if (payment.action !== 'boost' && payment.type !== 'zap') {
      return // filter out streams, basically
    }

    paymentCounter++

    if (config.priority !== undefined && config.priority !== (paymentCounter % config.numalerts)) {
      return // show a porition of the payments based on priority and numalerts
    }

    alertQueue.push(payment)
  }

  const getAvailableSlots = (slots, activeSlots) => {
    const availableSlots = slots.filter(x => !x.playing())

    if (availableSlots.length === 0) {
      return [] // none available
    }

    if (activeSlots !== undefined && activeSlots <= slots.length - availableSlots.length) {
      return [] // too many slots taken
    }

    return availableSlots
  }

  const selectSlot = (slots, availableSlots, curSlot) => {
    if (config.randomizeSlots === undefined || config.randomizeSlots) {
      return { slot: availableSlots[Math.floor(Math.random() * availableSlots.length)], newSlot: curSlot }
    }

    const slot = availableSlots[curSlot]
    const newSlot = (curSlot + 1) % slots.length

    return { slot, newSlot }
  }

  const getRemoteInfo = (payment) => {
      return (payment.remote_feed) ? `${payment.remote_feed} - ${payment.remote_item}` : ""
  }

  const defaultMessageRenderer = (slot, payment) => {
    const sats = payment.sats.toLocaleString()
    const userMessage = config.showMessages ? payment.message : getRemoteInfo(payment)

    slot.showMessage(`${sats} sat ${payment.type} from ${payment.sender_name}`, userMessage, payment.picture)
  }

  const handleAlertQueue = () => {
    if (alertQueue.length === 0) {
      return
    }

    const availableSlots = getAvailableSlots(slots, config.activeSlots)

    if (availableSlots.length === 0) {
      return
    }

    const { slot, newSlot } = selectSlot(slots, availableSlots, curSlot)
    curSlot = newSlot

    const payment = alertQueue.shift()
    const renderer = config.messageRender || defaultMessageRenderer

    renderer(slot, payment)
  }


  init()
}
