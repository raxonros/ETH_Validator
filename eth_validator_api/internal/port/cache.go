package port

import "eth_validator_api/internal/domain"


type SyncDutiesCache interface {
    Add(slot uint64, duties domain.SyncDuties)
    Get(slot uint64) (domain.SyncDuties, bool)
}