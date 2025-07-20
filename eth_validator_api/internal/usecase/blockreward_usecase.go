package usecase

import (
    "context"
    "eth_validator_api/internal/port"
)

type BlockRewardUseCase struct {
    client port.BlockRewardClient
}

func NewBlockRewardUseCase(client port.BlockRewardClient) *BlockRewardUseCase {
    return &BlockRewardUseCase{client: client}
}

func (uc *BlockRewardUseCase) Execute(ctx context.Context, slot uint64) (interface{}, error) {
    return uc.client.GetBlockReward(ctx, slot)
}