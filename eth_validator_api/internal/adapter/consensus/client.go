package consensus

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    stderrors "errors"         
    "strings"
    "time"

    "go.uber.org/zap"
    "github.com/ethereum/go-ethereum/ethclient"
    "github.com/ethereum/go-ethereum/rpc"

    apierr "eth_validator_api/internal/errors"  
    "eth_validator_api/internal/domain"
    "eth_validator_api/internal/port"
	"eth_validator_api/internal/retry"
)

const (
    syncCommitteesPath = "/eth/v1/beacon/states/%d/sync_committees"
    validatorsPath     = "/eth/v1/beacon/states/%d/validators?id=%s"
)


type ConsensusClient struct {
    rpcClient  *rpc.Client
    execClient *ethclient.Client
    httpClient *http.Client
    endpoint   string
    maxRetries int
    backoff    time.Duration
}


func NewConsensusClient(wsEndpoint, httpEndpoint string, syncDutiesMaxRetries int, syncDutiesBackoff time.Duration, syncDutiesRequestTimeout time.Duration) (port.SyncDutiesClient, error) {
    rpcCli, err := rpc.Dial(wsEndpoint)
    if err != nil {
        return nil, err
    }
    execCli, err := ethclient.Dial(httpEndpoint)
    if err != nil {
        return nil, err
    }

	httpCli := &http.Client{Timeout: syncDutiesRequestTimeout}
	
    return &ConsensusClient{
        rpcClient:  rpcCli,
        execClient: execCli,
        httpClient: httpCli,
        endpoint:   httpEndpoint,
        maxRetries: syncDutiesMaxRetries,
        backoff:    syncDutiesBackoff,
    }, nil
}


func (cc *ConsensusClient) GetSyncDuties(
    ctx context.Context,
    slot uint64,
) (domain.SyncDuties, error) {
    head, err := cc.execClient.BlockNumber(ctx)
    if err != nil {
        zap.L().Error("failed to fetch head slot", zap.Error(err))
        return domain.SyncDuties{}, err
    }
    if slot > head {
        return domain.SyncDuties{}, apierr.ErrSlotTooFarInFuture
    }

    url := fmt.Sprintf("%s"+syncCommitteesPath, cc.endpoint, slot)
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil {
        zap.L().Error("building sync_committees request failed", zap.Error(err))
        return domain.SyncDuties{}, err
    }

    var respBody []byte
    var statusCode int
    if err := retry.Do(ctx, cc.maxRetries, cc.backoff, func() error {
        resp, err := cc.httpClient.Do(req)
        if err != nil {
            return err
        }
        defer resp.Body.Close()
        respBody, _ = io.ReadAll(resp.Body)
        statusCode = resp.StatusCode
        if statusCode >= 500 || statusCode == 429 {
            return fmt.Errorf("transient status %d", statusCode)
        }
        return nil
    }); err != nil {
        if stderrors.Is(err, context.DeadlineExceeded) {
            zap.L().Warn("sync_committees request timed out", zap.Uint64("slot", slot))
            return domain.SyncDuties{}, apierr.ErrRequestTimeout
        }
        return domain.SyncDuties{}, err
    }

    var indices []string
    switch statusCode {
    case http.StatusOK:
        var out struct {
            Data struct {
                Validators []string `json:"validators"`
            } `json:"data"`
        }
        if err := json.Unmarshal(respBody, &out); err != nil {
            zap.L().Error("decoding sync_committees response failed", zap.Error(err))
            return domain.SyncDuties{}, err
        }
        indices = out.Data.Validators

    case http.StatusBadRequest:
        if strings.Contains(string(respBody), "not activated for Altair") {
            return domain.SyncDuties{Validators: []string{}}, nil
        }
        return domain.SyncDuties{}, apierr.ErrSlotTooFarInFuture

    case http.StatusNotFound:
        return domain.SyncDuties{}, apierr.ErrSlotNotFound

    default:
        zap.L().Error("unexpected status from sync_committees endpoint",
            zap.Int("code", statusCode),
            zap.ByteString("body", respBody),
        )
        return domain.SyncDuties{}, fmt.Errorf("unexpected status %d", statusCode)
    }

    if len(indices) == 0 {
        return domain.SyncDuties{Validators: []string{}}, nil
    }

    idxParam := strings.Join(indices, ",")
    valURL := fmt.Sprintf("%s"+validatorsPath, cc.endpoint, slot, idxParam)
    valReq, err := http.NewRequestWithContext(ctx, http.MethodGet, valURL, nil)
    if err != nil {
        zap.L().Error("building validators request failed", zap.Error(err))
        return domain.SyncDuties{}, err
    }

    var valBody []byte
    var valStatus int
    if err := retry.Do(ctx, cc.maxRetries, cc.backoff, func() error {
        resp, err := cc.httpClient.Do(valReq)
        if err != nil {
            return err
        }
        defer resp.Body.Close()
        valBody, _ = io.ReadAll(resp.Body)
        valStatus = resp.StatusCode
        if valStatus >= 500 || valStatus == 429 {
            return fmt.Errorf("transient status %d", valStatus)
        }
        return nil
    }); err != nil {
        if stderrors.Is(err, context.DeadlineExceeded) {
            zap.L().Warn("validators request timed out", zap.Uint64("slot", slot))
            return domain.SyncDuties{}, apierr.ErrRequestTimeout
        }
        return domain.SyncDuties{}, err
    }

    if valStatus != http.StatusOK {
        zap.L().Error("validators endpoint error",
            zap.Int("code", valStatus),
            zap.ByteString("body", valBody),
        )
        return domain.SyncDuties{}, fmt.Errorf("validators endpoint returned %d", valStatus)
    }

    var vr struct {
        Data []struct {
            Index     string `json:"index"`
            Validator struct {
                Pubkey string `json:"pubkey"`
            } `json:"validator"`
        } `json:"data"`
    }
    if err := json.Unmarshal(valBody, &vr); err != nil {
        zap.L().Error("decoding validators response failed", zap.Error(err))
        return domain.SyncDuties{}, err
    }

    pubkeys := make([]string, len(indices))
    for i, idx := range indices {
        for _, entry := range vr.Data {
            if entry.Index == idx {
                pubkeys[i] = entry.Validator.Pubkey
                break
            }
        }
    }

    return domain.SyncDuties{Validators: pubkeys}, nil
}



