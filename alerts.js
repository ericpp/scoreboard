class AlertImage {
  playing = false
  duration = 10

  constructor(image) {
    this.image = image
  }

  play() {
    this.playing = true

    setTimeout(() => {
      this.playing = false
    }, this.duration * 1000)
  }
}

class AlertVideo extends AlertImage {
  constructor(image) {
    super(image)
    this.image.addEventListener("play", () => this.playing = true)
    this.image.addEventListener("ended", () => this.playing = false)
  }

  play() {
    this.image.play()
  }
}

function getAlert(image) {
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

  const app = new PaymentTracker(config)

  const alert = document.getElementById("alert")
  const background = getAlert(document.getElementById("background"))

  const alertQueue = []

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

    if (typeof url.showMessages !== undefined) {
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

    app.setListener((payment, old) => {
      if (payment.action !== 'boost' && payment.type !== 'zap') {
        return // filter out streams, basically
      }

      if (config.priority !== undefined && config.priority !== (payment.creation_date % config.numalerts)) {
        return // show a porition of the payments based on priority and numalerts
      }

      alertQueue.push(payment)
    })

    setInterval(() => {
      if (alertQueue.length === 0 || background.playing) {
        return
      }

      const payment = alertQueue.shift()
      const sats = payment.sats.toLocaleString()
      const message = config.showMessages ? payment.message : getRemoteInfo(payment)

      showMessage(`${sats} sat ${payment.type} from ${payment.sender_name}`, message, payment.picture)
    }, 1000)

    app.start()
  }

  const getRemoteInfo = (payment) => {
      if (!payment.remote_feed) {
        return ""
      }

      return `${payment.remote_feed} - ${payment.remote_item}`
  }

  const loadImage = (src) => {
    return new Promise((resolve, reject) => {
      let img = new Image()
      img.onload = () => resolve(img)
      img.onerror = reject
      img.src = src
      img.style.height = "127px"
      img.style.width = "auto"
    })
  }

  const showMessage = async (message, line2, picture) => {
    background.play()

    const msg = alert.querySelector("#messageText")
    msg.textContent = message

    if (line2) {
      const span = document.createElement("span")
      span.textContent = line2
      msg.innerHTML += '<br>' + span.innerHTML
    }

    if (picture) {
      const box = alert.querySelector("#picture")
      box.innerHTML = ""
      const img = await loadImage(picture)
      box.appendChild(img)
    }

    alert.classList.add("show")

    setTimeout(() => {
      alert.classList.remove("show")
    }, (background.duration - 2) * 1000)
  }

  init()
}
