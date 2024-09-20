
async function startAlerts(config) {
  const app = new PaymentTracker()

  app.setNostrBoostPkey("804eeaaf5afc67cae9aa50a6ae03571ae693fcb277bd40d64b966b12dcba25ce")
  app.setFilter('excludePodcasts', ["Podcasting 2.0", "Pew Pew", "12 Rods"])

  app.loadBoosts = false

  const boostQueue = []

  const params = (new URL(document.location)).searchParams
  const pos = params.get("pos") || "tm"
  const test = params.get("test") || ""

  const alert = document.getElementById("alert")
  const background = document.getElementById("background")

  let playing = false

  background.addEventListener("play", () => playing = true)
  background.addEventListener("ended", () => playing = false)

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
