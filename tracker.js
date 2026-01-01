class PaymentTracker {
  constructor(config = {}) {
    this.relays = config.relays || ["wss://relay.damus.io", "wss://nos.lol"]
    this.nostrBoostPkey = config.nostrBoostPkey || null
    this.nostrZapEvent = config.nostrZapEvent || null
    this.loadBoosts = config.loadBoosts ?? true
    this.loadZaps = config.loadZaps ?? true
    this.filters = {}
    this.identifiers = []
    this.listener = null
    this.storedBoosts = null
    this.nostrWatcher = null
    this.lastBoostAt = null

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
    this.listener = listener
  }

  async loadStoredBoosts() {
    this.storedBoosts = new StoredBoosts(this.filters)
    await this.storedBoosts.load((item) => this.add(item))
  }

  subscribeBoosts() {
    this.nostrWatcher.subscribeBoosts(this.nostrBoostPkey, (item) => {
      if (this.lastBoostAt > item.creation_date) return
      this.add(item)
    })
  }

  subscribeZaps() {
    this.nostrWatcher.subscribeZaps(this.nostrZapEvent, (item) => {
      this.add(item)
    })
  }

  async start() {
    if (this.loadBoosts) {
      await this.loadStoredBoosts()
    }

    if (this.nostrBoostPkey) {
      this.subscribeBoosts()
    }

    if (this.nostrZapEvent) {
      this.subscribeZaps()
    }
  }

  testBoost(name, sats) {
    this.add({
      type: 'boost',
      identifier: String(Math.floor(Math.random()*100000000)),
      creation_date: Math.floor(Date.now() / 1000),
      sats: sats,
      sender_name: name,
      app_name: 'Test',
      podcast: 'Test',
      event_guid: 'Test',
      episode_guid: 'Test',
      episode: 'Test',
      isOld: false,
      isTest: true,
    })
  }

  add(payment) {
    if (payment.isTest) {
      this.listener(payment, payment.isOld)
      this.identifiers.push(payment.identifier)
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
    this.identifiers.push(payment.identifier)
  }
}

class NostrWatcher {
  constructor(relays) {
    this.nostrPool = new NostrTools.SimplePool()
    this.nostrProfileQueue = {}
    this.nostrProfiles = {}
    this.nostrRelays = relays

    // Set up profile resolution interval
    this.setupProfileResolution()
  }

  parsePubkey(pubkey) {
    if (!pubkey.includes('npub')) return pubkey
    const { data } = NostrTools.nip19.decode(pubkey)
    return data
  }

  parseActivity(addr) {
    if (!addr.includes('naddr')) return addr
    const { data } = NostrTools.nip19.decode(addr)
    return [data.kind, data.pubkey, data.identifier].join(':')
  }

  subscribeBoosts(nostrPubkey, callback) {
    let isOld = true
    let isOldTimeout = null
    const parsedPubkey = this.parsePubkey(nostrPubkey)
    const self = this

    const filters = {
      'authors': [parsedPubkey],
      'kinds': [30078],
    }

    this.nostrPool.subscribeMany(this.nostrRelays, filters, {
      async onevent(event) {
        const invoice = JSON.parse(event.content)

        // Manage isOld state
        if (isOldTimeout) clearTimeout(isOldTimeout)
        if (isOld) {
          isOldTimeout = setTimeout(() => { isOld = false }, 5000)
        }

        if (!invoice.boostagram) return

        const boost = invoice.boostagram
        
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
          isOld: isOld,
          ...await addRemoteInfo(boost),
        }, isOld)
      },
      oneose() {
        // Subscription completed
      },
      onclose() {
        // Subscription closed
        console.log('Socket closed, reconnecting and resubscribing to boosts...')
        self.subscribeBoosts(nostrPubkey, callback)
      }
    })
  }

  subscribeZaps(nostrActivity, callback) {
    let isOld = true
    let isOldTimeout = null
    const self = this
    const parsedActivity = this.parseActivity(nostrActivity)

    const filters = {
      '#a': [parsedActivity],
      'kinds': [9735],
    }

    this.nostrPool.subscribeMany(this.nostrRelays, filters, {
      async onevent(event) {
        // Manage isOld state
        if (isOldTimeout) clearTimeout(isOldTimeout)
        if (isOld) {
          isOldTimeout = setTimeout(() => { isOld = false }, 5000)
        }

        // Convert tags array into an object
        const tags = event.tags.reduce((result, tag) => {
          const [name, value] = tag
          if (!result[name]) result[name] = []
          result[name].push(value)
          return result
        }, {})

        // Process zap data
        const zapRequest = JSON.parse(tags.description[0])
        const value_msat_total = self.getMsatsFromBolt11(tags.bolt11[0])
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
          isOld: isOld,
        }, isOld)
      },
      oneose() {
        // Subscription completed
      },
      onclose() {
        // Subscription closed
        console.log('Socket closed, reconnecting and resubscribing to zaps...')
        self.subscribeZaps(nostrActivity, callback)
      }
    })
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

    // Parse amount from bolt11 string (e.g. lnbc100n -> 100n -> 100 * 100 = 10,000)
    const matches = bolt11.match(/^ln\w+?(\d+)([a-zA-Z]?)/)
    
    if (!matches) return null
    
    return parseInt(matches[1]) * (multipliers[matches[2]] || 1)
  }

  setupProfileResolution() {
    setInterval(async () => {
      if (!this.nostrProfileQueue) return

      const pubkeys = Object.keys(this.nostrProfileQueue)
      if (pubkeys.length === 0) return

      const profiles = await this.nostrPool.querySync(
        this.nostrRelays, 
        {authors: pubkeys, kinds: [0]}
      )

      // Update profiles cache
      profiles.forEach(event => {
        this.nostrProfiles[event.pubkey] = JSON.parse(event.content)
      })

      // Resolve pending promises
      for (const pubkey of pubkeys) {
        const resolvers = this.nostrProfileQueue[pubkey]
        if (resolvers) {
          delete this.nostrProfileQueue[pubkey]
          resolvers.forEach(resolve => {
            resolve(this.nostrProfiles[pubkey] || {})
          })
        }
      }
    }, 1000)
  }
}

class StoredBoosts {
  constructor(filters = {}) {
    this.filters = filters
  }

  async load(callback) {
    let page = 1
    const items = 1000
    let lastBoostAt = this.filters.after || null

    while (true) {
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

      // Fetch boosts
      const result = await fetch(`https://boostboard.vercel.app/api/boosts?${query}`)
      const boosts = await result.json()

      if (!boosts || boosts.length === 0) break

      // Update last boost time
      lastBoostAt = Math.max(
        lastBoostAt || 0, 
        Math.max(...boosts.map(x => x.creation_date))
      )

      // Sort boosts by creation date for replaying
      boosts.sort(
        (a, b) => a.creation_date - b.creation_date
      )

      // Process boosts
      for (const invoice of boosts) {
        if (!invoice.boostagram) continue

        const boost = invoice.boostagram
        
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
          ...await addRemoteInfo(boost),
        }, true)
      }

      page++
    }

    return lastBoostAt
  }
}

class RemoteItemInfo {
  constructor() {
    this.resolved = {}
    this.queue = {}
    this.inProgress = {} // Track requests in progress to prevent duplicates
    
    this.setupItemResolution()
  }

  async fetch(podcastguid, episodeguid) {
    const url = `https://api.podcastindex.org/api/1.0/value/byepisodeguid?podcastguid=${podcastguid}&episodeguid=${episodeguid}`
    const result = await fetch(url)
    const json = await result.json()

    if (json.status === 'false') return {}

    return {
      remote_feed: json.value.feedTitle,
      remote_item: json.value.title,
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
    setInterval(async () => {
      for (const item of Object.values(this.queue)) {
        const key = `${item.podcastguid}|${item.episodeguid}`

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

        // Only resolve if we have a result (either cached or just fetched)
        if (this.resolved[key] !== undefined && item.resolvers.length > 0) {
          const resolvers = [...item.resolvers]
          item.resolvers = []

          for (const resolver of resolvers) {
            resolver(this.resolved[key])
          }
        }
      }
    }, 100)
  }
}

// Singleton for remote info
let remoteInfo = null

async function addRemoteInfo(boost) {
  if (!boost.remote_feed_guid || !boost.remote_item_guid) {
    return {}
  }

  if (!remoteInfo) {
    remoteInfo = new RemoteItemInfo()
  }

  return await remoteInfo.resolve(boost.remote_feed_guid, boost.remote_item_guid)
}

function getUrlConfig(url) {
  const params = new URL(url).searchParams.entries()
  return [...params].reduce((result, [key, val]) => {
    result[key] = val
    return result
  }, {})
}