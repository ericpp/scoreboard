
function PaymentTracker(config) {
  config = config || {}

  this.relays = config.relays || ["wss://relay.damus.io", "wss://nos.lol"]

  this.nostrBoostPkey = config.nostrBoostPkey || null
  this.nostrZapEvent = config.nostrZapEvent || null

  this.loadBoosts = true

  if (config.loadBoosts !== undefined) {
    this.loadBoosts = config.loadBoosts
  }

  this.loadZaps = true

  if (config.loadZaps !== undefined) {
    this.loadZaps = config.loadZaps
  }

  this.filters = {}
  this.identifiers = []
  this.listener = null

  this.storedBoosts = null
  this.nostrWatcher = null
  this.lastBoostAt = null

  this.init = () => {
    Object.entries(config || {}).forEach(([name, value]) => {
      this.setFilter(name, value)
    })

    this.nostrWatcher = new NostrWatcher(this.relays)
  }

  this.setFilter = (name, value) => {
    value = this.parseFilterValue(name, value)

    if (name == 'after') {
      this.lastBoostAt = value
    }

    this.filters[name] = value
  }

  this.parseFilterValue = (name, value) => {
    if ((name == 'before' || name == 'after') && typeof(value) != "number") {
      return Math.floor(new Date(value) / 1000)
    }

    return value
  }

  this.setLoadBoosts = (shouldLoad) => {
    this.loadBoosts = shouldLoad
  }

  this.setNostrBoostPkey = (pkey) => {
    this.nostrBoostPkey = pkey
  }

  this.setNostrZapEvent = (event) => {
    this.nostrZapEvent = event
  }

  this.setListener = (listener) => {
    this.listener = listener
  }

  this.loadStoredBoosts = async () => {
    this.storedBoosts = new StoredBoosts(this.filters)

    await this.storedBoosts.load((item, old) => {
      this.add(item, old)
    })
  }

  this.subscribeBoosts = () => {
    this.nostrWatcher.subscribeBoosts(this.nostrBoostPkey, (item, old) => {
      if (this.lastBoostAt > item.creation_date) {
        return
      }

      this.add(item, old)
    })
  }

  this.subscribeZaps = () => {
    this.nostrWatcher.subscribeZaps(this.nostrZapEvent, (item, old) => {
      this.add(item, old)
    })
  }

  this.start = async () => {
    if (this.loadBoosts) {
      this.loadStoredBoosts()
    }

    if (this.nostrBoostPkey) {
      this.subscribeBoosts()
    }

    if (this.nostrZapEvent) {
      this.subscribeZaps()
    }
  }

  this.add = (payment, old) => {
    if (old && payment.type == 'boost' && !this.loadBoosts) {
      return // skip olds if loadBoosts = false
    }

    if (old && payment.type == 'zap' && !this.loadZaps) {
      return // skip olds if loadZaps = false
    }

    if (!payment.sats || isNaN(payment.sats)) {
      return // missing sat amount
    }

    if (this.identifiers.indexOf(payment.identifier) !== -1) {
      return // already seen
    }

    if (this.filters.excludePodcasts) {
      const exclude = this.filters.excludePodcasts.filter(
        filter => payment.podcast.indexOf(filter) !== -1
      ).length

      if (exclude) {
        return
      }
    }

    if (this.filters.podcast && this.filters.podcast != payment.podcast) {
      return
    }

    if (this.filters.before && this.filters.before < payment.creation_date) {
      return
    }

    if (this.filters.after && this.filters.after > payment.creation_date) {
      return
    }

    this.listener(payment, old)

    this.identifiers.push(payment.identifier)
  }

  this.init()
}

function NostrWatcher(relays) {
  this.nostrPool = new NostrTools.SimplePool()

  this.nostrProfileQueue = {}
  this.nostrProfiles = {}

  this.nostrRelays = relays

  this.parsePubkey = (pubkey) => {
    if (pubkey.indexOf('npub') === -1) {
      return pubkey
    }

    const parse = NostrTools.nip19.decode(pubkey)
    return parse.data
  }

  this.parseActivity = (addr) => {
    if (addr.indexOf('naddr') === -1) {
      return addr
    }

    const parse = NostrTools.nip19.decode(addr)
    return [parse.data.kind, parse.data.pubkey, parse.data.identifier].join(':')
  }

  this.subscribeBoosts = (nostrPubkey, callback) => {
    let isOld = true
    let isOldTimeout = null
    let self = this

    nostrPubkey = this.parsePubkey(nostrPubkey)

    this.nostrPool.subscribeMany(this.nostrRelays, [{authors: [nostrPubkey]}], {
      async onevent(event) {
        invoice = JSON.parse(event.content)

        if (isOldTimeout) {
          clearTimeout(isOldTimeout)
        }

        if (isOld) {
          // turn off isOld after 5 seconds of inactivity
          isOldTimeout = setTimeout(() => {
            isOld = false
          }, 5000)
        }

        if (!invoice.boostagram) {
          return
        }

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
          sats: Math.floor(boost.value_msat_total / 1000),
          message: boost.message,
          ...await addRemoteInfo(boost),
        }, isOld)
      },
      oneose() {
        // h.close()
      }
    })
  }

  this.subscribeZaps = (nostrActivity, callback) => {
    let isOld = true
    let isOldTimeout = null
    let self = this

    nostrActivity = this.parseActivity(nostrActivity)

    this.nostrPool.subscribeMany(this.nostrRelays, [{'#a': [nostrActivity], 'kinds': [9735]}], {
      async onevent(event) {
        if (isOldTimeout) {
          clearTimeout(isOldTimeout)
        }

        if (isOld) {
          // turn off isOld after 5 seconds of inactivity
          isOldTimeout = setTimeout(() => {
            isOld = false
          }, 5000)
        }

        // convert tags array into an object
        const tags = event.tags.reduce((result, tag) => {
          const [name, value] = tag
          if (!result[name]) result[name] = []
          result[name].push(value)
          return result
        }, {})

        // original zap request is encoded as a tag in the zap receipt
        const zaprequest = JSON.parse(tags.description[0])

        // msats can be calculated from bolt11 request
        const value_msat_total = self.getMsatsFromBolt11(tags.bolt11[0])

        // batch look up names based on original zap request pubkey
        const profile = await self.getNostrProfile(zaprequest.pubkey)

        // send back to subscriber
        callback({
          type: 'zap',
          action: 'zap',
          identifier: event.id,
          creation_date: event.created_at,
          sender_name: profile.display_name || profile.name || 'Anonymous',
          picture: profile.picture || null,
          app_name: 'Nostr',
          podcast: 'Nostr',
          sats: Math.floor(value_msat_total / 1000),
          message: event.content,
        }, isOld)
      },
      oneose() {
        // h.close()
      }
    })
  }

  this.getNostrProfile = (pubkey) => {
    return new Promise((resolve, reject) => {
      if (this.nostrProfiles[pubkey]) {
        resolve(this.nostrProfiles[pubkey])
      }
      else {
        if (!this.nostrProfileQueue[pubkey]) {
          this.nostrProfileQueue[pubkey] = []
        }

        this.nostrProfileQueue[pubkey].push(resolve)
      }
    })
  }

  this.getMsatsFromBolt11 = (bolt11) => {
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

  // resolve queued nostr pubkeys to names
  setInterval(async () => {
    if (!this.nostrProfileQueue) return

    let pubkeys = Object.keys(this.nostrProfileQueue)
    if (pubkeys.length === 0) return

    let profiles = await this.nostrPool.querySync(this.nostrRelays, {authors: pubkeys, kinds: [0]})

    profiles.forEach(event => {
      const profile = JSON.parse(event.content)
      this.nostrProfiles[event.pubkey] = profile
    })

    for (let pubkey of pubkeys) {
      let resolvers = this.nostrProfileQueue[pubkey]

      if (resolvers) {
        delete this.nostrProfileQueue[pubkey]

        resolvers.forEach(resolve => {
          resolve(this.nostrProfiles[pubkey] || {})
        })
      }
    }
  }, 1000)
}

function StoredBoosts(filters) {

  this.filters = filters || {}

  this.load = async (callback) => {
    let page = 1
    let items = 1000
    let lastBoostAt = null

    if (filters.after) {
      lastBoostAt = filters.after
    }

    while (true) {
      const query = new URLSearchParams()
      query.set("page", page)
      query.set("items", items)

      if (filters.before) {
        query.set("created_at_lt", filters.before)
      }

      if (filters.after) {
        query.set("created_at_gt", filters.after)
      }

      const result = await fetch(`https://boostboard.vercel.app/api/boosts?${query}`)
      const boosts = await result.json()

      if (!boosts || boosts.length === 0) {
        break
      }

      lastBoostAt = Math.max(lastBoostAt, Math.max(...boosts.map(x => x.creation_date)))

      boosts.forEach(async invoice => {
        if (!invoice.boostagram) {
          return
        }

        const boost = invoice.boostagram

        callback({
          type: 'boost',
          action: boost.action || 'unknown',
          identifier: invoice.identifier,
          creation_date: invoice.creation_date,
          sender_name: boost.sender_name || 'Anonymous',
          app_name: boost.app_name || 'Unknown',
          podcast: boost.podcast || 'Unknown',
          sats: Math.floor(boost.value_msat_total / 1000),
          message: boost.message,
          ...await addRemoteInfo(boost),
        }, true)
      })

      page++
    }

    return lastBoostAt
  }
}

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

function RemoteItemInfo() {
  this.resolved = {}
  this.queue = {}

  this.fetch = async (podcastguid, episodeguid) => {
    const result = await fetch(`https://api.podcastindex.org/api/1.0/value/byepisodeguid?podcastguid=${podcastguid}&episodeguid=${episodeguid}`)
    const json = await result.json()

    if (json.status == 'false') {
      return {}
    }

    return {
      "remote_feed": json.value.feedTitle,
      "remote_item": json.value.title,
    }
  }

  this.resolve = (podcastguid, episodeguid) => {
    return new Promise(resolve => {
      const key = podcastguid + "|" + episodeguid

      if (!this.queue[key]) {
        this.queue[key] = {
          "podcastguid": podcastguid,
          "episodeguid": episodeguid,
          "resolvers": [],
        }
      }

      this.queue[key].resolvers.push(resolve)
    })
  }

  setInterval(async () => {
    Object.values(this.queue).forEach(async item => {
      const key = item.podcastguid + "|" + item.episodeguid

      if (this.resolved[key] === undefined) {
        this.resolved[key] = await this.fetch(item.podcastguid, item.episodeguid)
      }

      item.resolvers.forEach((resolver, index) => {
        delete item.resolvers[index]
        resolver(this.resolved[key])
      })
    })
  }, 100)
}


const getUrlConfig = (url) => {
  const params = (new URL(url)).searchParams

  return params.entries().reduce((result, [key, val]) => {
    result[key] = val
    return result
  }, {})
}