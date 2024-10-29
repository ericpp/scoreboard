let remoteInfo = null;

async function addRemoteInfo(boost) {
    if (!boost.remote_feed_guid) {
        return {}
    }

    if (!remoteInfo) {
        remoteInfo = new RemoteItemInfo()
    }

    return await remoteInfo.resolve(boost.remote_feed_guid, boost.remote_item_guid)
}

function RemoteItemInfo() {
    this.resolved = {}

    this.fetch = async (podcastguid, episodeguid) => {
        const result = await fetch(`https://api.podcastindex.org/api/1.0/value/byepisodeguid?podcastguid=${podcastguid}&episodeguid=${episodeguid}`)
        const json = await result.json()

        return {
            "remote_feed": json.value.feedTitle,
            "remote_item": json.value.title,
        }
    }

    this.resolve = async (podcastguid, episodeguid) => {
        const key = podcastguid + "|" + episodeguid

        if (!this.resolved[key]) {
            this.resolved[key] = await this.fetch(podcastguid, episodeguid)
        }

        return this.resolved[key]
    }
}

function PaymentTracker() {
    const nostrRelays = ["wss://relay.damus.io", "wss://nos.lol", "wss://relay.nostr.band"]

    this.filters = {}
    this.identifiers = []
    this.listener = null
    this.lastBoostAt = null

    this.loadBoosts = true
    this.nostrBoostPkey = null
    this.nostrZapEvent = null

    this.storedBoosts = new StoredBoosts(this.filters)
    this.nostrWatcher = new NostrWatcher(nostrRelays)

    this.setFilter = (name, value) => {
        if ((name == 'before' || name == 'after') && typeof(value) != "number") {
            value = Math.floor(new Date(value) / 1000)
        }

        if (name == 'after') {
            this.lastBoostAt = value
        }

        this.filters[name] = value
    }

    this.loadBoosts = (shouldLoad) => {
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

    this.start = async () => {
        if (this.loadBoosts) {
            await this.storedBoosts.load((item, old) => {
                this.add(item, old)
            })
        }

        if (this.nostrBoostPkey) {
            this.nostrWatcher.subscribeBoosts(this.nostrBoostPkey, (item, old) => {
                if (this.lastBoostAt > item.creation_date) {
                    return
                }

                this.add(item, old)
            })
        }

        if (this.nostrZapEvent) {
            this.nostrWatcher.subscribeZaps(this.nostrZapEvent, (item, old) => {
                this.add(item, old)
            })
        }
    }

    this.add = (payment, old) => {
        if (old && !this.loadBoosts) {
            return // skip olds if loadBoosts = false
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
}

function NostrWatcher(relays) {
    this.nostrPool = new NostrTools.SimplePool()

    this.nostrNameQueue = {}
    this.nostrNames = {}

    this.nostrRelays = relays

    this.subscribeBoosts = (nostrPubkey, callback) => {
        let isOld = true
        let isOldTimeout = null
        let self = this

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

    this.subscribeZaps = (nostrEvent, callback) => {
        let isOld = true
        let isOldTimeout = null
        let self = this

        this.nostrPool.subscribeMany(this.nostrRelays, [{'#a': [nostrEvent], 'kinds': [9735]}], {
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
                const sender_name = await self.getNostrName(zaprequest.pubkey)

                // send back to subscriber
                callback({
                    type: 'zap',
                    action: 'zap',
                    identifier: event.id,
                    creation_date: event.created_at,
                    sender_name: sender_name || 'Anonymous',
                    app_name: 'Nostr',
                    podcast: 'Nostr',
                    sats: Math.floor(value_msat_total / 1000),
                    message: event.content,
                    ...await addRemoteInfo(boost),
                }, isOld)
            },
            oneose() {
                // h.close()
            }
        })
    }

    this.getNostrName = (pubkey) => {
        return new Promise((resolve, reject) => {
            if (this.nostrNames[pubkey]) {
                resolve(this.nostrNames[pubkey])
            }
            else {
                if (!this.nostrNameQueue[pubkey]) {
                    this.nostrNameQueue[pubkey] = []
                }

                this.nostrNameQueue[pubkey].push(resolve)
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
        if (!this.nostrNameQueue) return

        let pubkeys = Object.keys(this.nostrNameQueue)
        if (pubkeys.length === 0) return

        let profiles = await this.nostrPool.querySync(this.nostrRelays, {authors: pubkeys, kinds: [0]})

        profiles.forEach(event => {
            const profile = JSON.parse(event.content)
            this.nostrNames[event.pubkey] = profile.display_name || profile.name
        })

        for (let pubkey of pubkeys) {
            let resolvers = this.nostrNameQueue[pubkey]

            if (resolvers) {
                delete this.nostrNameQueue[pubkey]

                resolvers.forEach(resolve => {
                    resolve(this.nostrNames[pubkey] || null)
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