package execution

import (
    "context"
    "math/big"
    "time"
    "regexp"
    

    "go.uber.org/zap"
    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/core/types"
    "github.com/ethereum/go-ethereum/common/hexutil"

    "eth_validator_api/internal/errors"
    "eth_validator_api/internal/domain"
    "eth_validator_api/internal/port"
	"eth_validator_api/internal/retry"
    
)

var mevRegex = regexp.MustCompile(`(?i)(flashbots|titanbuilder|eden|mev-boost)`)


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
    if mevRegex.Match(header.Extra) {
        status = "mev"
    }
    if _, ok := ec.mevRelays[header.Coinbase]; ok {
        status = "mev"
    }

    hexSlotPrev := hexutil.EncodeUint64(slot - 1)
    hexSlot := hexutil.EncodeUint64(slot)
    addr := header.Coinbase.Hex()

    batch := []rpc.BatchElem{
        {
            Method: "eth_getBalance",
            Args:   []interface{}{addr, hexSlotPrev},
            Result: new(string),
        },
        {
            Method: "eth_getBalance",
            Args:   []interface{}{addr, hexSlot},
            Result: new(string),
        },
    }

    if err := ec.rpcClient.BatchCallContext(ctx, batch); err != nil {
        zap.L().Error("batch balance call failed", zap.Error(err))
        return domain.BlockReward{}, err
    }

    beforeStr := *batch[0].Result.(*string)
    afterStr := *batch[1].Result.(*string)

    beforeWei, err := hexutil.DecodeBig(beforeStr)
    if err != nil {
        return domain.BlockReward{}, err
    }
    afterWei, err := hexutil.DecodeBig(afterStr)
    if err != nil {
        return domain.BlockReward{}, err
    }

    rewardWei := new(big.Int).Sub(afterWei, beforeWei)
    rewardGwei := new(big.Int).Div(rewardWei, big.NewInt(1e9)).Uint64()

    return domain.BlockReward{
        Status: status,
        Reward: float64(rewardGwei),
    }, nil
}

