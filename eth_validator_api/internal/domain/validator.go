package domain


type BlockReward struct {
    Status string  `json:"status"`
    Reward float64 `json:"reward_gwei"`
}

type SyncDuties struct {
    Validators []string `json:"validators"`
}