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
    execClient *ethclient.Client
    httpClient *http.Client
    endpoint   string
    maxRetries int
    backoff    time.Duration
}


func NewConsensusClient(httpEndpoint string, syncDutiesMaxRetries int, syncDutiesBackoff time.Duration, syncDutiesRequestTimeout time.Duration) (port.SyncDutiesClient, error) {
   
    execCli, err := ethclient.Dial(httpEndpoint)
    if err != nil {
        return nil, err
    }

	httpCli := &http.Client{Timeout: syncDutiesRequestTimeout}
	
    return &ConsensusClient{
        execClient: execCli,
        httpClient: httpCli,
        endpoint:   httpEndpoint,
        maxRetries: syncDutiesMaxRetries,
        backoff:    syncDutiesBackoff,
    }, nil
}

func (cc *ConsensusClient) GetSyncDuties(ctx context.Context, slot uint64) (domain.SyncDuties, error) {
    head, err := cc.execClient.BlockNumber(ctx)
    if err != nil {
        zap.L().Error("failed to fetch head slot", zap.Error(err))
        return domain.SyncDuties{}, err
    }
    if slot > head {
        return domain.SyncDuties{}, apierr.ErrSlotTooFarInFuture
    }

    indices, err := cc.fetchSyncCommittees(ctx, slot)
    if err != nil {
        return domain.SyncDuties{}, err
    }
    if len(indices) == 0 {
        return domain.SyncDuties{Validators: []string{}}, nil
    }

    pubkeys, err := cc.fetchValidatorPubkeys(ctx, slot, indices)
    if err != nil {
        return domain.SyncDuties{}, err
    }

    return domain.SyncDuties{Validators: pubkeys}, nil
}

func (cc *ConsensusClient) fetchSyncCommittees(ctx context.Context, slot uint64) ([]string, error) {
    url := fmt.Sprintf(cc.endpoint+syncCommitteesPath, slot)
    body, status, err := cc.doGet(ctx, url)
    if err != nil {
        if stderrors.Is(err, context.DeadlineExceeded) {
            zap.L().Warn("sync_committees request timed out", zap.Uint64("slot", slot))
            return nil, apierr.ErrRequestTimeout
        }
        return nil, err
    }

    switch status {
    case http.StatusOK:
        var out struct{ Data struct{ Validators []string }} 
        if err := json.Unmarshal(body, &out); err != nil {
            zap.L().Error("decoding sync_committees failed", zap.Error(err))
            return nil, err
        }
        return out.Data.Validators, nil

    case http.StatusBadRequest:
        if strings.Contains(string(body), "not activated for Altair") {
            return nil, nil
        }
        return nil, apierr.ErrSlotTooFarInFuture

    case http.StatusNotFound:
        return nil, apierr.ErrSlotNotFound

    default:
        zap.L().Error("unexpected status sync_committees", zap.Int("code", status))
        return nil, fmt.Errorf("unexpected status %d", status)
    }
}

func (cc *ConsensusClient) fetchValidatorPubkeys(ctx context.Context, slot uint64, indices []string) ([]string, error) {
    idxParam := strings.Join(indices, ",")
    url := fmt.Sprintf(cc.endpoint+validatorsPath, slot, idxParam)
    body, status, err := cc.doGet(ctx, url)
    if err != nil {
        if stderrors.Is(err, context.DeadlineExceeded) {
            zap.L().Warn("validators request timed out", zap.Uint64("slot", slot))
            return nil, apierr.ErrRequestTimeout
        }
        return nil, err
    }
    if status != http.StatusOK {
        zap.L().Error("validators error", zap.Int("code", status))
        return nil, fmt.Errorf("validators returned %d", status)
    }

    var vr struct{ Data []struct {
        Index     string `json:"index"`
        Validator struct{ Pubkey string `json:"pubkey"` } `json:"validator"`
    } }
    if err := json.Unmarshal(body, &vr); err != nil {
        zap.L().Error("decoding validators failed", zap.Error(err))
        return nil, err
    }

    pubkeys := make([]string, len(indices))
    for i, idx := range indices {
        for _, e := range vr.Data {
            if e.Index == idx {
                pubkeys[i] = e.Validator.Pubkey
                break
            }
        }
    }
    return pubkeys, nil
}

func (cc *ConsensusClient) doGet(ctx context.Context, url string) ([]byte, int, error) {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil {
        return nil, 0, err
    }

    var body []byte
    var status int
    err = retry.Do(ctx, cc.maxRetries, cc.backoff, func() error {
        resp, err := cc.httpClient.Do(req)
        if err != nil {
            return err
        }
        defer resp.Body.Close()

        body, _ = io.ReadAll(resp.Body)
        status = resp.StatusCode
        if status >= 500 || status == 429 {
            return fmt.Errorf("transient status %d", status)
        }
        return nil
    })
    if err != nil {
        return nil, 0, err
    }
    return body, status, nil
}
