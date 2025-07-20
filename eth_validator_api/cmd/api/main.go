package main

import (
    stdhttp "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"
    "context"
    "fmt"

    "go.uber.org/zap"
    "github.com/ethereum/go-ethereum/ethclient"
    "github.com/ethereum/go-ethereum/rpc"

    "eth_validator_api/internal/adapter/consensus"
    "eth_validator_api/internal/adapter/execution"
    "eth_validator_api/internal/usecase"
    httpPkg "eth_validator_api/pkg/http"
    "eth_validator_api/pkg/config"
    "eth_validator_api/pkg/logger"
)

func main() {

    log, err := logger.Init()
    if err != nil {
        panic(err)
    }
    defer func() {
        if err := log.Sync(); err != nil {
            fmt.Fprintf(os.Stderr, "error syncing logger: %v\n", err)
        }
    }()
    zap.ReplaceGlobals(log)

    cfg, err := config.Load()
    if err != nil {
        zap.L().Fatal("failed to load config", zap.Error(err))
    }

    consClient, err := consensus.NewConsensusClient(
        cfg.Ethereum.RPCHTTP,
        cfg.Retry.SyncDuties.MaxRetries,
        cfg.Retry.SyncDuties.Backoff,
        cfg.Retry.SyncDuties.Timeout,
    )
    if err != nil {
        zap.L().Fatal("init consensus client", zap.Error(err))
    }

    cache, err := consensus.NewSyncDutiesCache(
        cfg.Cache.SyncDuties.MaxEntries,
        cfg.Cache.SyncDuties.TTL,
    )
    if err != nil {
        zap.L().Fatal("init sync duties cache", zap.Error(err))
    }

    sdUC := usecase.NewSyncDutiesUseCase(consClient, cache)

    ethHTTP, err := ethclient.Dial(cfg.Ethereum.RPCHTTP)
    if err != nil {
        zap.L().Fatal("dial ethclient", zap.Error(err))
    }
    rpcHTTP, err := rpc.DialHTTP(cfg.Ethereum.RPCHTTP)
    if err != nil {
        zap.L().Fatal("dial rpc", zap.Error(err))
    }
    execClient, err := execution.NewExecutionClient(
        rpcHTTP,
        ethHTTP,
        cfg.Ethereum.MevRelays,
        cfg.Retry.BlockReward.MaxRetries,
        cfg.Retry.BlockReward.Backoff,
    )
    if err != nil {
        zap.L().Fatal("init execution client", zap.Error(err))
    }
    brUC := usecase.NewBlockRewardUseCase(execClient)

    r := httpPkg.NewRouter(cfg, brUC, sdUC)

    srv := &stdhttp.Server{
        Addr:    cfg.Server.Address,
        Handler: r,
    }
    go func() {
        zap.L().Info("starting server", zap.String("address", cfg.Server.Address))
        if err := srv.ListenAndServe(); err != nil && err != stdhttp.ErrServerClosed {
            zap.L().Fatal("listen error", zap.Error(err))
        }
    }()

    stop := make(chan os.Signal, 1)
    signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
    <-stop

    zap.L().Info("shutting down…")
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    if err := srv.Shutdown(ctx); err != nil {
        zap.L().Error("shutdown error", zap.Error(err))
    }
    zap.L().Info("server stopped")
}
