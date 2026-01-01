class PaymentTracker {
  constructor(config = {}) {
    this.relays = config.relays || ["wss://relay.damus.io", "wss://nos.lol", "wss://relay.primal.net", "wss://relay.nos.social"]
    this.nostrBoostPkey = config.nostrBoostPkey || null
    this.nostrZapEvent = config.nostrZapEvent || null
    this.loadBoosts = config.loadBoosts ?? true
    this.loadZaps = config.loadZaps ?? true
    this.maxIdentifiers = config.maxIdentifiers || 10000 // Configurable memory limit
    this.filters = {}
    this.identifiers = []
    this.listener = null
    this.storedBoosts = null
    this.nostrWatcher = null
    this.lastBoostAt = null
    this.destroyed = false

    // Initialize filters from config
    Object.entries(config).forEach(([name, value]) => {
      this.setFilter(name, value)
    })

    this.nostrWatcher = new NostrWatcher(this.relays)
  }

  setFilter(name, value) {
    value = this.parseFilterValue(name, value)

    if (name === 'after') {
      this.lastBoostAt = value
    }

    if (name === 'podcast') {
      name = 'podcasts'
      value = [value]
    }

    this.filters[name] = value
  }

  parseFilterValue(name, value) {
    if ((name === 'before' || name === 'after') && typeof value !== "number") {
      if (typeof value === "string" && /^\d+$/.test(value)) {
        return parseInt(value)
      }

      return Math.floor(new Date(value) / 1000)
    }

    return value
  }

  setLoadBoosts(shouldLoad) {
    this.loadBoosts = shouldLoad
  }

  setNostrBoostPkey(pkey) {
    this.nostrBoostPkey = pkey
  }

  setNostrZapEvent(event) {
    this.nostrZapEvent = event
  }

  setListener(listener) {
    if (typeof listener !== 'function') {
      throw new Error('Listener must be a function')
    }
    this.listener = listener
  }

  async loadStoredBoosts() {
    this.storedBoosts = new StoredBoosts(this.filters)
    try {
      await this.storedBoosts.load((item) => this.add(item))
    } catch (error) {
      console.error('Error loading stored boosts:', error)
      throw error
    }
  }

  subscribeBoosts() {
    this.nostrWatcher.subscribeBoosts(this.nostrBoostPkey, (item) => {
      if (this.destroyed) return
      if (this.lastBoostAt && this.lastBoostAt > item.creation_date) return
      this.add(item)
    })
  }

  subscribeZaps() {
    this.nostrWatcher.subscribeZaps(this.nostrZapEvent, (item) => {
      if (this.destroyed) return
      this.add(item)
    })
  }

  async start() {
    if (!this.listener) {
      throw new Error('Listener must be set before calling start()')
    }

    if (this.destroyed) {
      throw new Error('Cannot start a destroyed PaymentTracker')
    }

    try {
      if (this.loadBoosts) {
        await this.loadStoredBoosts()
      }

      if (this.nostrBoostPkey) {
        this.subscribeBoosts()
      }

      if (this.nostrZapEvent) {
        this.subscribeZaps()
      }
    } catch (error) {
      console.error('Error starting PaymentTracker:', error)
      throw error
    }
  }

  testBoost(name, sats) {
    const testPayment = {
      type: 'boost',
      identifier: String(Math.floor(Math.random() * 100000000)),
      creation_date: Math.floor(Date.now() / 1000),
      sats: sats,
      sender_name: this.sanitize(name),
      app_name: 'Test',
      podcast: 'Test',
      event_guid: 'Test',
      episode_guid: 'Test',
      episode: 'Test',
      isOld: false,
      isTest: true,
    }
    this.add(testPayment)
  }

  sanitize(text) {
    if (typeof text !== 'string') return text
    // Basic XSS protection - escape HTML entities
    return text
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#x27;')
  }

  add(payment) {
    if (this.destroyed || !this.listener) return

    // Sanitize user-provided text fields
    if (payment.sender_name) payment.sender_name = this.sanitize(payment.sender_name)
    if (payment.message) payment.message = this.sanitize(payment.message)

    if (payment.isTest) {
      this.listener(payment, payment.isOld)
      this.addIdentifier(payment.identifier)
      return
    }

    // Skip old items if loading is disabled for that type
    if (payment.isOld && payment.type === 'boost' && !this.loadBoosts) return
    if (payment.isOld && payment.type === 'zap' && !this.loadZaps) return

    // Skip invalid or duplicate payments
    if (!payment.sats || isNaN(payment.sats)) return
    if (this.identifiers.includes(payment.identifier)) return

    // Apply filters
    if (this.filters.excludePodcasts &&
        this.filters.excludePodcasts.some(filter =>
          payment.podcast?.toLowerCase().includes(filter.toLowerCase()))) {
      return
    }

    if (this.filters.before && this.filters.before < payment.creation_date) return
    if (this.filters.after && this.filters.after > payment.creation_date) return

    // Check podcast, event, or episode match if filters are set
    const podcastMatch = this.filters.podcasts?.some(p => payment.podcast?.toLowerCase().includes(p?.toLowerCase()))
    const eventGuidMatch = this.filters.eventGuids?.some(e => payment.event_guid === e)
    const episodeGuidMatch = this.filters.episodeGuids?.some(e => payment.episode_guid === e)

    if (
      payment.type === 'boost' &&
      (this.filters.podcasts || this.filters.eventGuids || this.filters.episodeGuids) &&
      (!podcastMatch && !eventGuidMatch && !episodeGuidMatch)
    ) {
      return
    }

    // Process valid payment
    this.listener(payment, payment.isOld)
    this.addIdentifier(payment.identifier)
  }

  addIdentifier(identifier) {
    // Implement circular buffer to prevent memory leak
    this.identifiers.push(identifier)
    if (this.identifiers.length > this.maxIdentifiers) {
      this.identifiers.shift()
    }
  }

  destroy() {
    this.destroyed = true
    if (this.nostrWatcher) {
      this.nostrWatcher.destroy()
    }
    this.identifiers = []
    this.listener = null
  }
}

class NostrWatcher {
  constructor(relays) {
    this.nostrPool = new NostrTools.SimplePool()
    this.nostrProfileQueue = {}
    this.nostrProfiles = {}
    this.nostrRelays = relays
    this.profileInterval = null
    this.subscriptions = []
    this.destroyed = false
    this.reconnectAttempts = new Map() // Track reconnection attempts
    this.maxReconnectAttempts = 5
    this.reconnectDelay = 1000

    // Set up profile resolution interval
    this.setupProfileResolution()
  }

  parsePubkey(pubkey) {
    if (!pubkey || !pubkey.includes('npub')) return pubkey
    try {
      const { data } = NostrTools.nip19.decode(pubkey)
      return data
    } catch (error) {
      console.error('Error parsing pubkey:', error)
      return pubkey
    }
  }

  parseActivity(addr) {
    if (!addr || !addr.includes('naddr')) return addr
    try {
      const { data } = NostrTools.nip19.decode(addr)
      return [data.kind, data.pubkey, data.identifier].join(':')
    } catch (error) {
      console.error('Error parsing activity:', error)
      return addr
    }
  }

  handleOldState() {
    // Reusable function for isOld state management
    let isOld = true
    let isOldTimeout = null

    const updateOldState = () => {
      if (isOldTimeout) clearTimeout(isOldTimeout)
      if (isOld) {
        isOldTimeout = setTimeout(() => { isOld = false }, 5000)
      }
    }

    const cleanup = () => {
      if (isOldTimeout) clearTimeout(isOldTimeout)
    }

    const getIsOld = () => isOld

    return { updateOldState, cleanup, getIsOld }
  }

  subscribeBoosts(nostrPubkey, callback) {
    if (this.destroyed) return

    const subscriptionId = `boosts-${nostrPubkey}`
    const parsedPubkey = this.parsePubkey(nostrPubkey)
    const { updateOldState, cleanup, getIsOld } = this.handleOldState()
    const self = this

    const filters = {
      'authors': [parsedPubkey],
      'kinds': [30078],
    }

    const sub = this.nostrPool.subscribeMany(this.nostrRelays, filters, {
      async onevent(event) {
        if (self.destroyed) return

        try {
          const invoice = JSON.parse(event.content)
          updateOldState()

          if (!invoice.boostagram) return

          const boost = invoice.boostagram
          const remoteInfo = await addRemoteInfo(boost)

          callback({
            type: 'boost',
            action: boost.action || 'unknown',
            identifier: invoice.identifier,
            creation_date: invoice.creation_date,
            sender_name: boost.sender_name || 'Anonymous',
            picture: null,
            app_name: boost.app_name || 'Unknown',
            podcast: boost.podcast || 'Unknown',
            event_guid: boost.eventGuid || null,
            episode_guid: boost.episode_guid || null,
            episode: boost.episode || null,
            sats: Math.floor(boost.value_msat_total / 1000),
            message: boost.message,
            isOld: getIsOld(),
            ...remoteInfo,
          })
        } catch (error) {
          console.error('Error processing boost event:', error)
        }
      },
      oneose() {
        // Subscription completed
      },
      onclose() {
        if (self.destroyed) {
          cleanup()
          return
        }

        const attempts = self.reconnectAttempts.get(subscriptionId) || 0

        if (attempts < self.maxReconnectAttempts) {
          console.log(`Socket closed, reconnecting to boosts (attempt ${attempts + 1}/${self.maxReconnectAttempts})...`)
          self.reconnectAttempts.set(subscriptionId, attempts + 1)

          setTimeout(() => {
            if (!self.destroyed) {
              self.subscribeBoosts(nostrPubkey, callback)
            }
          }, self.reconnectDelay * Math.pow(2, attempts)) // Exponential backoff
        } else {
          console.error('Max reconnection attempts reached for boosts subscription')
          cleanup()
        }
      }
    })

    this.subscriptions.push({ id: subscriptionId, sub, cleanup })
  }

  subscribeZaps(nostrActivity, callback) {
    if (this.destroyed) return

    const subscriptionId = `zaps-${nostrActivity}`
    const parsedActivity = this.parseActivity(nostrActivity)
    const { updateOldState, cleanup, getIsOld } = this.handleOldState()
    const self = this

    const filters = {
      '#a': [parsedActivity],
      'kinds': [9735],
    }

    const sub = this.nostrPool.subscribeMany(this.nostrRelays, filters, {
      async onevent(event) {
        if (self.destroyed) return

        try {
          updateOldState()

          // Convert tags array into an object
          const tags = event.tags.reduce((result, tag) => {
            const [name, value] = tag
            if (!result[name]) result[name] = []
            result[name].push(value)
            return result
          }, {})

          if (!tags.description || !tags.bolt11) {
            console.warn('Missing required tags in zap event')
            return
          }

          // Process zap data
          const zapRequest = JSON.parse(tags.description[0])
          const value_msat_total = self.getMsatsFromBolt11(tags.bolt11[0])

          if (!value_msat_total) {
            console.warn('Could not parse sats from bolt11')
            return
          }

          const profile = await self.getNostrProfile(zapRequest.pubkey)

          callback({
            type: 'zap',
            action: 'zap',
            identifier: event.id,
            creation_date: event.created_at,
            sender_name: profile.display_name || profile.name || 'Anonymous',
            picture: profile.picture || null,
            app_name: 'Nostr',
            podcast: 'Nostr',
            event_guid: null,
            episode_guid: null,
            episode: null,
            sats: Math.floor(value_msat_total / 1000),
            message: event.content,
            isOld: getIsOld(),
          })
        } catch (error) {
          console.error('Error processing zap event:', error)
        }
      },
      oneose() {
        // Subscription completed
      },
      onclose() {
        if (self.destroyed) {
          cleanup()
          return
        }

        const attempts = self.reconnectAttempts.get(subscriptionId) || 0

        if (attempts < self.maxReconnectAttempts) {
          console.log(`Socket closed, reconnecting to zaps (attempt ${attempts + 1}/${self.maxReconnectAttempts})...`)
          self.reconnectAttempts.set(subscriptionId, attempts + 1)

          setTimeout(() => {
            if (!self.destroyed) {
              self.subscribeZaps(nostrActivity, callback)
            }
          }, self.reconnectDelay * Math.pow(2, attempts)) // Exponential backoff
        } else {
          console.error('Max reconnection attempts reached for zaps subscription')
          cleanup()
        }
      }
    })

    this.subscriptions.push({ id: subscriptionId, sub, cleanup })
  }

  getNostrProfile(pubkey) {
    return new Promise(resolve => {
      if (this.nostrProfiles[pubkey]) {
        resolve(this.nostrProfiles[pubkey])
      } else {
        if (!this.nostrProfileQueue[pubkey]) {
          this.nostrProfileQueue[pubkey] = []
        }
        this.nostrProfileQueue[pubkey].push(resolve)
      }
    })
  }

  getMsatsFromBolt11(bolt11) {
    const multipliers = {
      m: 100000000,
      u: 100000,
      n: 100,
      p: 0.1,
    }

    try {
      // Parse amount from bolt11 string (e.g. lnbc100n -> 100n -> 100 * 100 = 10,000)
      const matches = bolt11.match(/^ln\w+?(\d+)([a-zA-Z]?)/)

      if (!matches) return null

      return parseInt(matches[1]) * (multipliers[matches[2]] || 1)
    } catch (error) {
      console.error('Error parsing bolt11:', error)
      return null
    }
  }

  setupProfileResolution() {
    this.profileInterval = setInterval(async () => {
      if (this.destroyed || !this.nostrProfileQueue) return

      const pubkeys = Object.keys(this.nostrProfileQueue)
      if (pubkeys.length === 0) return

      try {
        const profiles = await this.nostrPool.querySync(
          this.nostrRelays,
          {authors: pubkeys, kinds: [0]}
        )

        // Update profiles cache
        profiles.forEach(event => {
          try {
            this.nostrProfiles[event.pubkey] = JSON.parse(event.content)
          } catch (error) {
            console.error('Error parsing profile content:', error)
            this.nostrProfiles[event.pubkey] = {}
          }
        })

        // Resolve pending promises (snapshot to avoid race conditions)
        const queueSnapshot = {...this.nostrProfileQueue}

        for (const pubkey of Object.keys(queueSnapshot)) {
          const resolvers = queueSnapshot[pubkey]
          if (resolvers && this.nostrProfileQueue[pubkey]) {
            delete this.nostrProfileQueue[pubkey]
            resolvers.forEach(resolve => {
              resolve(this.nostrProfiles[pubkey] || {})
            })
          }
        }
      } catch (error) {
        console.error('Error resolving profiles:', error)
      }
    }, 1000)
  }

  destroy() {
    this.destroyed = true

    // Clear profile resolution interval
    if (this.profileInterval) {
      clearInterval(this.profileInterval)
      this.profileInterval = null
    }

    // Clean up all subscriptions
    this.subscriptions.forEach(({ cleanup }) => {
      if (cleanup) cleanup()
    })
    this.subscriptions = []

    // Clear data structures
    this.nostrProfileQueue = {}
    this.nostrProfiles = {}
    this.reconnectAttempts.clear()
  }
}

class StoredBoosts {
  constructor(filters = {}) {
    this.filters = filters
    this.maxPages = filters.maxPages || 100 // Configurable max pages
  }

  async load(callback) {
    let page = 1
    const items = 1000
    let lastBoostAt = this.filters.after || null

    try {
      while (page <= this.maxPages) {
        const query = new URLSearchParams()
        query.set("page", page)
        query.set("items", items)

        // Apply filters to query
        if (this.filters.podcasts) {
          query.set("podcast", this.filters.podcasts.join(","))
        }

        if (this.filters.eventGuids) {
          query.set("eventGuid", this.filters.eventGuids.join(","))
        }

        if (this.filters.episodeGuids) {
          query.set("episodeGuid", this.filters.episodeGuids.join(","))
        }

        if (this.filters.before) {
          query.set("created_at_lt", this.filters.before)
        }

        if (this.filters.after) {
          query.set("created_at_gt", this.filters.after)
        }

        // Fetch boosts with timeout
        const controller = new AbortController()
        const timeoutId = setTimeout(() => controller.abort(), 30000) // 30 second timeout

        let result
        try {
          result = await fetch(`https://boostboard.vercel.app/api/boosts?${query}`, {
            signal: controller.signal
          })
          clearTimeout(timeoutId)
        } catch (error) {
          clearTimeout(timeoutId)
          if (error.name === 'AbortError') {
            console.error('Boost fetch timeout')
            break
          }
          throw error
        }

        if (!result.ok) {
          console.error(`HTTP error fetching boosts: ${result.status}`)
          break
        }

        const boosts = await result.json()

        if (!boosts || boosts.length === 0) break

        // Update last boost time
        lastBoostAt = Math.max(
          lastBoostAt || 0,
          Math.max(...boosts.map(x => x.creation_date))
        )

        // Sort boosts by creation date for replaying
        boosts.sort((a, b) => a.creation_date - b.creation_date)

        // Process boosts
        for (const invoice of boosts) {
          if (!invoice.boostagram) continue

          const boost = invoice.boostagram

          try {
            const remoteInfo = await addRemoteInfo(boost)

            callback({
              type: 'boost',
              action: boost.action || 'unknown',
              identifier: invoice.identifier,
              creation_date: invoice.creation_date,
              sender_name: boost.sender_name || 'Anonymous',
              app_name: boost.app_name || 'Unknown',
              podcast: boost.podcast || 'Unknown',
              event_guid: boost.eventGuid || null,
              episode_guid: boost.episode_guid || null,
              episode: boost.episode || null,
              sats: Math.floor(boost.value_msat_total / 1000),
              message: boost.message,
              isOld: true,
              ...remoteInfo,
            })
          } catch (error) {
            console.error('Error processing boost:', error)
          }
        }

        page++
      }

      if (page > this.maxPages) {
        console.warn(`Reached max pages limit (${this.maxPages})`)
      }
    } catch (error) {
      console.error('Error loading stored boosts:', error)
      throw error
    }

    return lastBoostAt
  }
}

class RemoteItemInfo {
  constructor() {
    this.resolved = {}
    this.queue = {}
    this.inProgress = {}
    this.interval = null
    this.destroyed = false

    this.setupItemResolution()
  }

  async fetch(podcastguid, episodeguid) {
    try {
      const controller = new AbortController()
      const timeoutId = setTimeout(() => controller.abort(), 10000) // 10 second timeout

      const url = `https://api.podcastindex.org/api/1.0/value/byepisodeguid?podcastguid=${podcastguid}&episodeguid=${episodeguid}`

      let result
      try {
        result = await fetch(url, { signal: controller.signal })
        clearTimeout(timeoutId)
      } catch (error) {
        clearTimeout(timeoutId)
        if (error.name === 'AbortError') {
          console.error('Remote item fetch timeout')
          return {}
        }
        throw error
      }

      if (!result.ok) {
        console.error(`HTTP error fetching remote item: ${result.status}`)
        return {}
      }

      const json = await result.json()

      if (json.status === 'false') return {}

      return {
        remote_feed: json.value?.feedTitle,
        remote_item: json.value?.title,
      }
    } catch (error) {
      console.error('Error fetching remote item info:', error)
      return {}
    }
  }

  resolve(podcastguid, episodeguid) {
    return new Promise(resolve => {
      const key = `${podcastguid}|${episodeguid}`

      if (!this.queue[key]) {
        this.queue[key] = {
          podcastguid,
          episodeguid,
          resolvers: [],
        }
      }

      this.queue[key].resolvers.push(resolve)
    })
  }

  setupItemResolution() {
    this.interval = setInterval(async () => {
      if (this.destroyed) return

      // Create snapshot to avoid race conditions
      const queueSnapshot = Object.entries(this.queue).map(([key, item]) => ({
        key,
        ...item,
        resolvers: [...item.resolvers]
      }))

      for (const item of queueSnapshot) {
        if (this.destroyed) break

        const key = item.key

        // Only fetch if not already resolved and not currently in progress
        if (this.resolved[key] === undefined && !this.inProgress[key]) {
          this.inProgress[key] = true
          try {
            this.resolved[key] = await this.fetch(item.podcastguid, item.episodeguid)
          } catch (error) {
            console.error(`Failed to fetch remote info for ${key}:`, error)
            this.resolved[key] = {} // Set empty object on error to prevent retries
          } finally {
            delete this.inProgress[key]
          }
        }

        // Only resolve if we have a result and there are pending resolvers
        if (this.resolved[key] !== undefined && this.queue[key]?.resolvers.length > 0) {
          const resolvers = [...this.queue[key].resolvers]
          this.queue[key].resolvers = []

          for (const resolver of resolvers) {
            resolver(this.resolved[key])
          }

          // Clean up queue entry if no more resolvers
          if (this.queue[key].resolvers.length === 0) {
            delete this.queue[key]
          }
        }
      }
    }, 100)
  }

  destroy() {
    this.destroyed = true
    if (this.interval) {
      clearInterval(this.interval)
      this.interval = null
    }
    this.queue = {}
    this.inProgress = {}
  }
}

// Singleton for remote info
let remoteInfo = null

async function addRemoteInfo(boost) {
  if (!boost || !boost.remote_feed_guid || !boost.remote_item_guid) {
    return {}
  }

  if (!remoteInfo) {
    remoteInfo = new RemoteItemInfo()
  }

  try {
    return await remoteInfo.resolve(boost.remote_feed_guid, boost.remote_item_guid)
  } catch (error) {
    console.error('Error adding remote info:', error)
    return {}
  }
}

function getUrlConfig(url) {
  try {
    const params = new URL(url).searchParams.entries()
    return [...params].reduce((result, [key, val]) => {
      result[key] = val
      return result
    }, {})
  } catch (error) {
    console.error('Error parsing URL config:', error)
    return {}
  }
}

// Cleanup function for global singleton
function cleanupRemoteInfo() {
  if (remoteInfo) {
    remoteInfo.destroy()
    remoteInfo = null
  }
}