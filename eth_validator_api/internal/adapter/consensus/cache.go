package consensus

import (
    "time"

    lru "github.com/hashicorp/golang-lru"
    "eth_validator_api/internal/domain"
)

type SyncDutiesCache struct {
    lruCache *lru.Cache
    ttl      time.Duration
}

type cacheEntry struct {
    duties domain.SyncDuties
    ts     time.Time
}

func NewSyncDutiesCache(maxEntries int, ttl time.Duration) (*SyncDutiesCache, error) {
    c, err := lru.New(maxEntries)
    if err != nil {
        return nil, err
    }
    return &SyncDutiesCache{
        lruCache: c,
        ttl:      ttl,
    }, nil
}

func (c *SyncDutiesCache) Get(slot uint64) (domain.SyncDuties, bool) {
    raw, ok := c.lruCache.Get(slot)
    if !ok {
        return domain.SyncDuties{}, false
    }
    e := raw.(cacheEntry)
    if time.Since(e.ts) > c.ttl {
        c.lruCache.Remove(slot)
        return domain.SyncDuties{}, false
    }
    return e.duties, true
}

func (c *SyncDutiesCache) Add(slot uint64, duties domain.SyncDuties) {
    c.lruCache.Add(slot, cacheEntry{
        duties: duties,
        ts:     time.Now(),
    })
}
