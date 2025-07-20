package port

import (
    "context"
    "eth_validator_api/internal/domain"
)

type BlockRewardClient interface {
    GetBlockReward(ctx context.Context, slot uint64) (domain.BlockReward, error)
}
type SyncDutiesClient interface {
    GetSyncDuties(ctx context.Context, slot uint64) (domain.SyncDuties, error)
}