package execution

import (
    "time"

    lru "github.com/hashicorp/golang-lru"
    "eth_validator_api/internal/domain"
)

type BlockRewardCache struct {
    lruCache *lru.Cache
    ttl      time.Duration
}

type cacheEntry struct {
    reward domain.BlockReward
    ts     time.Time
}

func NewBlockRewardCache(maxEntries int, ttl time.Duration) (*BlockRewardCache, error) {
    c, err := lru.New(maxEntries)
    if err != nil {
        return nil, err
    }
    return &BlockRewardCache{
        lruCache: c,
        ttl:      ttl,
    }, nil
}

func (c *BlockRewardCache) Get(slot uint64) (domain.BlockReward, bool) {
    raw, ok := c.lruCache.Get(slot)
    if !ok {
        return domain.BlockReward{}, false
    }
    e := raw.(cacheEntry)
    if time.Since(e.ts) > c.ttl {
        c.lruCache.Remove(slot)
        return domain.BlockReward{}, false
    }
    return e.reward, true
}

func (c *BlockRewardCache) Add(slot uint64, reward domain.BlockReward) {
    c.lruCache.Add(slot, cacheEntry{
        reward: reward,
        ts:     time.Now(),
    })
}
