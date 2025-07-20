package usecase

import (
    "context"

    "eth_validator_api/internal/domain"
    "eth_validator_api/internal/port"
)

type SyncDutiesUseCase struct {
    client port.SyncDutiesClient
    cache  port.SyncDutiesCache
}

func NewSyncDutiesUseCase(
    client port.SyncDutiesClient,
    cache port.SyncDutiesCache,
) *SyncDutiesUseCase {
    return &SyncDutiesUseCase{client: client, cache: cache}
}

func (uc *SyncDutiesUseCase) Execute(
    ctx context.Context,
    slot uint64,
) (domain.SyncDuties, error) {
    
    if v, ok := uc.cache.Get(slot); ok {
        return v, nil
    }
   
    duties, err := uc.client.GetSyncDuties(ctx, slot)
    if err != nil {
        return domain.SyncDuties{}, err
    }

    uc.cache.Add(slot, duties)
    return duties, nil
}
