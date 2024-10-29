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

  const app = new PaymentTracker()

  app.setNostrBoostPkey("804eeaaf5afc67cae9aa50a6ae03571ae693fcb277bd40d64b966b12dcba25ce")
  app.setFilter('excludePodcasts', ["Podcasting 2.0"])

  app.loadBoosts = false

  const boostQueue = []

  const params = (new URL(document.location)).searchParams
  const pos = params.get("pos") || "tm"
  const test = params.get("test") || ""

  app.setFilter('after', params.get("after") || "2024-10-28 00:00:00 -0500")
  app.setFilter('before', params.get("before"))

  const alert = document.getElementById("alert")
  const background = getAlert(document.getElementById("background"))

  let playing = false

  const init = () => {
    setInterval(() => {
      if (boostQueue.length === 0 || playing) {
        return
      }

      const boost = boostQueue.shift()
      const sats = boost.sats.toLocaleString()

      showMessage(`${sats} from ${boost.sender_name}`)
    }, 1000)
  }

  const showMessage = async (message) => {
    background.play()
    alert.classList.add("show")

    alert.querySelector("#message").textContent = message

    setTimeout(() => {
      alert.classList.remove("show")
    }, (background.duration - 2) * 1000)
  }

  app.setListener((boost, old) => {
    if (boost.action !== 'boost') {
      return;
    }

    if (config.priority !== undefined && config.priority !== (boost.creation_date % config.numalerts)) {
      return; // show a porition of the boosts based on priority and numalerts
    }

    boostQueue.push(boost);
  });

  if (test != "") {
    boostQueue.push({
      sender_name: "Anonymous PodcastGuru User",
      sats: 3333,
      app_name: "Test App",
    })

    app.loadBoosts = true
  }

  init()
  app.start()
}
