package usecase

import (
    "context"
    "eth_validator_api/internal/port"
    "eth_validator_api/internal/domain"
)

type BlockRewardUseCase struct {
    client port.BlockRewardClient
    cache  port.BlockRewardCache
}

func NewBlockRewardUseCase(
    client port.BlockRewardClient,
    cache port.BlockRewardCache,
) *BlockRewardUseCase {
    return &BlockRewardUseCase{client: client, cache: cache}
}

func (uc *BlockRewardUseCase) Execute(
    ctx context.Context,
    slot uint64,
) (domain.BlockReward, error) {
    
    if v, ok := uc.cache.Get(slot); ok {
        return v, nil
    }
   
    reward, err := uc.client.GetBlockReward(ctx, slot)
    if err != nil {
        return domain.BlockReward{}, err
    }

    uc.cache.Add(slot, reward)
    return reward, nil
}
