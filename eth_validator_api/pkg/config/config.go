package config

import (
    "github.com/spf13/viper"
    "time"
    "fmt"
)

type Config struct {
    Server struct {
        Address string
    }
    Ethereum struct {
        RPCHTTP   string
        RPCWS     string
        MevRelays []string `mapstructure:"MEV_RELAYS"`
    }
    Cache struct {
        SyncDuties struct {
            MaxEntries int           `mapstructure:"CACHE_SYNC_MAX_ENTRIES"`
            TTL        time.Duration `mapstructure:"CACHE_SYNC_TTL"`
        }
        BlockReward struct {
            MaxEntries int           `mapstructure:"CACHE_BLOCK_REWARD_MAX_ENTRIES"`
            TTL        time.Duration `mapstructure:"CACHE_BLOCK_REWARD_TTL"`
        }
    }
	Retry struct {
        BlockReward struct {
            Timeout   time.Duration `mapstructure:"BR_TIMEOUT"`
            MaxRetries int          `mapstructure:"BR_MAX_RETRIES"`
            Backoff    time.Duration `mapstructure:"BR_BACKOFF"`
        }
        SyncDuties struct {
            Timeout   time.Duration `mapstructure:"SD_TIMEOUT"`
            MaxRetries int          `mapstructure:"SD_MAX_RETRIES"`
            Backoff    time.Duration `mapstructure:"SD_BACKOFF"`
        }
    }
}

func Load() (*Config, error) {
    v := viper.New()
    v.SetConfigFile("config.json")
    v.AutomaticEnv()

    v.SetDefault("SERVER_ADDRESS", ":8080")
    v.SetDefault("ETH_RPC_HTTP", "default_value")
    v.SetDefault("ETH_RPC_WS", "default_value")
    v.SetDefault("MEV_RELAYS", []string{})
    v.SetDefault("CACHE_SYNC_MAX_ENTRIES", 1024)
    v.SetDefault("CACHE_SYNC_TTL",  "60m")
    v.SetDefault("CACHE_BLOCK_REWARD_MAX_ENTRIES", 1024)
    v.SetDefault("CACHE_BLOCK_REWARD_TTL",  "60m")
	v.SetDefault("BR_TIMEOUT",   "5s")
	v.SetDefault("BR_MAX_RETRIES", 3)
	v.SetDefault("BR_BACKOFF",    "100ms")
	v.SetDefault("SD_TIMEOUT",   "10s")
	v.SetDefault("SD_MAX_RETRIES", 3)
	v.SetDefault("SD_BACKOFF",    "100ms")

    if err := v.ReadInConfig(); err != nil {
        if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
            return nil, err
        }
    }

    cfg := &Config{}
    cfg.Server.Address = v.GetString("SERVER_ADDRESS")
    cfg.Ethereum.RPCHTTP = v.GetString("ETH_RPC_HTTP")
    cfg.Ethereum.RPCWS = v.GetString("ETH_RPC_WS")
    cfg.Ethereum.MevRelays = v.GetStringSlice("MEV_RELAYS")

    cfg.Cache.SyncDuties.MaxEntries = v.GetInt("CACHE_SYNC_MAX_ENTRIES")
    cfg.Cache.SyncDuties.TTL = v.GetDuration("CACHE_SYNC_TTL")
    
    cfg.Cache.BlockReward.MaxEntries = v.GetInt("CACHE_BLOCK_REWARD_MAX_ENTRIES")
    cfg.Cache.BlockReward.TTL = v.GetDuration("CACHE_BLOCK_REWARD_TTL")

    cfg.Retry.BlockReward.Timeout    = v.GetDuration("BR_TIMEOUT")
    cfg.Retry.BlockReward.MaxRetries = v.GetInt("BR_MAX_RETRIES")
    cfg.Retry.BlockReward.Backoff    = v.GetDuration("BR_BACKOFF")

    cfg.Retry.SyncDuties.Timeout    = v.GetDuration("SD_TIMEOUT")
    cfg.Retry.SyncDuties.MaxRetries = v.GetInt("SD_MAX_RETRIES")
    cfg.Retry.SyncDuties.Backoff    = v.GetDuration("SD_BACKOFF")

    if cfg.Server.Address == "" {
        return nil, fmt.Errorf("SERVER_ADDRESS must not be empty")
    }
    if cfg.Ethereum.RPCHTTP == "" {
        return nil, fmt.Errorf("ETH_RPC_HTTP must not be empty")
    }
    if cfg.Ethereum.RPCWS == "" {
        return nil, fmt.Errorf("ETH_RPC_WS must not be empty")
    }
    if cfg.Retry.BlockReward.MaxRetries < 1 {
        return nil, fmt.Errorf("BR_MAX_RETRIES must be ≥ 1")
    }
    if cfg.Retry.SyncDuties.MaxRetries < 1 {
        return nil, fmt.Errorf("SD_MAX_RETRIES must be ≥ 1")
    }

    return cfg, nil
}
