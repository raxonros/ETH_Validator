package port

import "eth_validator_api/internal/domain"


type SyncDutiesCache interface {
    Add(slot uint64, duties domain.SyncDuties)
    Get(slot uint64) (domain.SyncDuties, bool)
}

type BlockRewardCache interface {
    Add(slot uint64, reward domain.BlockReward)
    Get(slot uint64) (domain.BlockReward, bool)
}