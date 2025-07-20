package usecase_test

import (
    "context"
    "errors"
    "testing"

    "eth_validator_api/internal/domain"
    "eth_validator_api/internal/usecase"
)

type mockBRClient struct {
    result domain.BlockReward
    err    error
}

func (m *mockBRClient) GetBlockReward(ctx context.Context, slot uint64) (domain.BlockReward, error) {
    return m.result, m.err
}

func TestBlockRewardUseCase_GenesisSlot(t *testing.T) {
    uc := usecase.NewBlockRewardUseCase(&mockBRClient{
        result: domain.BlockReward{Status: "vanilla", Reward: 0},
        err:    nil,
    })
    res, err := uc.Execute(context.Background(), 0)
    if err != nil {
        t.Fatalf("esperaba sin error para slot génesis, got %v", err)
    }
    br, ok := res.(domain.BlockReward)
    if !ok {
        t.Fatalf("esperaba domain.BlockReward, got %T", res)
    }
    if br.Reward != 0 || br.Status != "vanilla" {
        t.Errorf("resultado inesperado: %+v", br)
    }
}

func TestBlockRewardUseCase_SlotNotFound(t *testing.T) {
    uc := usecase.NewBlockRewardUseCase(&mockBRClient{
        result: domain.BlockReward{},
        err:    errors.New("slot not found"),
    })
    _, err := uc.Execute(context.Background(), 123)
    if err == nil {
        t.Fatal("esperaba error para slot inexistente")
    }
}

func TestBlockRewardUseCase_SlotInFuture(t *testing.T) {
    uc := usecase.NewBlockRewardUseCase(&mockBRClient{
        result: domain.BlockReward{},
        err:    errors.New("slot in future"),
    })
    _, err := uc.Execute(context.Background(), 999999)
    if err == nil {
        t.Fatal("esperaba error para slot futuro")
    }
}


