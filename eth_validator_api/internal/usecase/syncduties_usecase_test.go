package usecase_test

import (
    "context"
    "errors"
    "testing"

    apierr "eth_validator_api/internal/errors"
    "eth_validator_api/internal/domain"
    "eth_validator_api/internal/usecase"
)

type dummyClient struct {
    duties domain.SyncDuties
    err    error
}

func (m *dummyClient) GetSyncDuties(ctx context.Context, slot uint64) (domain.SyncDuties, error) {
    return m.duties, m.err
}

type dummyCache struct {
    store map[uint64]domain.SyncDuties
}

func newDummyCache() *dummyCache {
    return &dummyCache{store: make(map[uint64]domain.SyncDuties)}
}

func (c *dummyCache) Get(slot uint64) (domain.SyncDuties, bool) {
    d, ok := c.store[slot]
    return d, ok
}

func (c *dummyCache) Add(slot uint64, duties domain.SyncDuties) {
    c.store[slot] = duties
}

func TestSyncDutiesUseCase_CacheMiss(t *testing.T) {
    want := domain.SyncDuties{Validators: []string{"A"}}
    client := &dummyClient{duties: want, err: nil}
    cache := newDummyCache()
    uc := usecase.NewSyncDutiesUseCase(client, cache)

    got, err := uc.Execute(context.Background(), 42)
    if err != nil {
        t.Fatalf("esperaba sin error, got %v", err)
    }
    if len(got.Validators) != 1 || got.Validators[0] != "A" {
        t.Errorf("unexpected result: %+v", got.Validators)
    }
    if _, found := cache.Get(42); !found {
        t.Error("esperaba que la respuesta se guardase en caché")
    }
}

func TestSyncDutiesUseCase_CacheHit(t *testing.T) {
    want := domain.SyncDuties{Validators: []string{"B"}}
    client := &dummyClient{duties: domain.SyncDuties{}, err: errors.New("no debe llamarse")}
    cache := newDummyCache()
    cache.Add(99, want)
    uc := usecase.NewSyncDutiesUseCase(client, cache)

    got, err := uc.Execute(context.Background(), 99)
    if err != nil {
        t.Fatalf("esperaba sin error, got %v", err)
    }
    if len(got.Validators) != 1 || got.Validators[0] != "B" {
        t.Errorf("unexpected result: %+v", got.Validators)
    }
}

func TestSyncDutiesUseCase_ClientError(t *testing.T) {
    client := &dummyClient{duties: domain.SyncDuties{}, err: errors.New("RPC falló")}
    cache := newDummyCache()
    uc := usecase.NewSyncDutiesUseCase(client, cache)

    _, err := uc.Execute(context.Background(), 7)
    if err == nil {
        t.Fatal("esperaba error del cliente")
    }
    if _, found := cache.Get(7); found {
        t.Error("no esperaba que se cachease tras un error")
    }
}

func TestSyncDutiesUseCase_SlotTooFarInFuture(t *testing.T) {
    client := &dummyClient{duties: domain.SyncDuties{}, err: apierr.ErrSlotTooFarInFuture}
    cache := newDummyCache()
    uc := usecase.NewSyncDutiesUseCase(client, cache)

    _, err := uc.Execute(context.Background(), 123)
    if err != apierr.ErrSlotTooFarInFuture {
        t.Fatalf("esperaba ErrSlotTooFarInFuture, got %v", err)
    }
    if _, found := cache.Get(123); found {
        t.Error("no esperaba que se cachease un slot futuro")
    }
}

func TestSyncDutiesUseCase_SlotNotFound(t *testing.T) {
    client := &dummyClient{duties: domain.SyncDuties{}, err: apierr.ErrSlotNotFound}
    cache := newDummyCache()
    uc := usecase.NewSyncDutiesUseCase(client, cache)

    _, err := uc.Execute(context.Background(), 8)
    if err != apierr.ErrSlotNotFound {
        t.Fatalf("esperaba ErrSlotNotFound, got %v", err)
    }
    if _, found := cache.Get(8); found {
        t.Error("no esperaba que se cachease un slot no encontrado")
    }
}
