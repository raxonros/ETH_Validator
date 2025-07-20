package execution

import (
    "context"
    "math/big"
    "time"

    "go.uber.org/zap"
    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/core/types"

    "eth_validator_api/internal/errors"
    "eth_validator_api/internal/domain"
    "eth_validator_api/internal/port"
	"eth_validator_api/internal/retry"
    
)

type ExecutionClient struct {
	rpcClient *rpc.Client
    ethClient *ethclient.Client
    mevRelays map[common.Address]struct{}
    maxRetries int
    backoff    time.Duration
}



func NewExecutionClient(
	rpcHTTP *rpc.Client,
    ethHTTP *ethclient.Client,
    mevAddrs []string,
    retryMaxRetries int,
    retryBackoff time.Duration,
) (port.BlockRewardClient, error) {
    relayMap := make(map[common.Address]struct{}, len(mevAddrs))
    for _, hex := range mevAddrs {
        relayMap[common.HexToAddress(hex)] = struct{}{}
    }
    return &ExecutionClient{
		rpcClient: rpcHTTP,
        ethClient: ethHTTP,
        mevRelays: relayMap,
        maxRetries: retryMaxRetries,
        backoff:    retryBackoff,
    }, nil
}



func (ec *ExecutionClient) GetBlockReward(ctx context.Context, slot uint64) (domain.BlockReward, error) {
    if slot == 0 {
        return domain.BlockReward{Status: "vanilla", Reward: 0}, nil
    }

 
    var head uint64
    if err := retry.Do(ctx, ec.maxRetries, ec.backoff, func() error {
        var err error
        head, err = ec.ethClient.BlockNumber(ctx)
        return err
    }); err != nil {
        zap.L().Error("failed to fetch head slot", zap.Error(err))
        return domain.BlockReward{}, err
    }
    if slot > head {
        return domain.BlockReward{}, errors.ErrSlotInFuture
    }

    var header *types.Header
    if err := retry.Do(ctx, ec.maxRetries, ec.backoff, func() error {
        var err error
        header, err = ec.ethClient.HeaderByNumber(ctx, big.NewInt(int64(slot)))
        return err
    }); err != nil {
        zap.L().Error("header not found", zap.Uint64("slot", slot), zap.Error(err))
        return domain.BlockReward{}, errors.ErrSlotNotFound
    }

    status := "vanilla"
    if _, ok := ec.mevRelays[header.Coinbase]; ok {
        status = "mev"
    }

    addr := header.Coinbase

    var before *big.Int
    if err := retry.Do(ctx, ec.maxRetries, ec.backoff, func() error {
        var err error
        before, err = ec.ethClient.BalanceAt(ctx, addr, big.NewInt(int64(slot-1)))
        return err
    }); err != nil {
        zap.L().Error("failed to get balance before block", zap.Error(err))
        return domain.BlockReward{}, err
    }

    var after *big.Int
    if err := retry.Do(ctx, ec.maxRetries, ec.backoff, func() error {
        var err error
        after, err = ec.ethClient.BalanceAt(ctx, addr, big.NewInt(int64(slot)))
        return err
    }); err != nil {
        zap.L().Error("failed to get balance after block", zap.Error(err))
        return domain.BlockReward{}, err
    }

    rewardWei := new(big.Int).Sub(after, before)
    rewardGwei := new(big.Int).Div(rewardWei, big.NewInt(1e9)).Uint64()

    return domain.BlockReward{
        Status: status,
        Reward: float64(rewardGwei),
    }, nil
}
