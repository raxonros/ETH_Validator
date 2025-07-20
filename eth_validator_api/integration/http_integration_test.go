package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
    "log"
	"time"
	"io"
	"bytes"
	"strings"

	"github.com/go-chi/chi"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	

	"eth_validator_api/internal/adapter/consensus"
	"eth_validator_api/internal/adapter/execution"
	"eth_validator_api/internal/handler"
	"eth_validator_api/internal/usecase"
	"eth_validator_api/internal/domain"
)

func mockQuickNode() *httptest.Server {
    mux := http.NewServeMux()

    mux.HandleFunc("/eth/v1/beacon/states/100/sync_committees", func(w http.ResponseWriter, r *http.Request) {
        log.Printf("[MOCK] REST GET %s", r.URL.Path)
        body, _ := io.ReadAll(r.Body)
        log.Printf("[MOCK]   Body: %s", string(body))
        r.Body = io.NopCloser(bytes.NewReader(body))

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "data": map[string][]string{"validators": {"AAA", "BBB"}},
        })
    })

    mux.HandleFunc("/eth/v1/beacon/states/100/validators", func(w http.ResponseWriter, r *http.Request) {
        log.Printf("[MOCK] REST GET %s?%s", r.URL.Path, r.URL.RawQuery)
        ids := r.URL.Query().Get("id")
        log.Printf("[MOCK]   ids: %s", ids)
        parts := strings.Split(ids, ",")
        type valEntry struct {
            Index     string `json:"index"`
            Validator struct {
                Pubkey string `json:"pubkey"`
            } `json:"validator"`
        }
        var data []valEntry
        for _, idx := range parts {
            e := valEntry{Index: idx}
            e.Validator.Pubkey = idx
            data = append(data, e)
        }
        resp := map[string]interface{}{"data": data}
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(resp)
    })

    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        log.Printf("[MOCK] RPC %s %s", r.Method, r.URL.Path)
        raw, _ := io.ReadAll(r.Body)
        log.Printf("[MOCK]   Body: %s", string(raw))
        r.Body = io.NopCloser(bytes.NewReader(raw))
        w.Header().Set("Content-Type", "application/json")
    
        if len(raw) > 0 && raw[0] == '[' {
            var batch []struct {
                Method string        `json:"method"`
                ID     int           `json:"id"`
                Params []interface{} `json:"params"`
            }
            if err := json.Unmarshal(raw, &batch); err != nil {
                log.Printf("[MOCK]   Batch decode error: %v", err)
                w.WriteHeader(http.StatusBadRequest)
                return
            }
            var replies []map[string]interface{}
            for _, req := range batch {
                rep := map[string]interface{}{
                    "jsonrpc": "2.0",
                    "id":      req.ID,
                }
                switch req.Method {
                case "eth_blockNumber":
                    rep["result"] = "0x64"
                case "eth_getBalance":
                    rep["result"] = "0xde0b6b3a7640000"
                case "eth_getBlockByNumber", "eth_getHeaderByNumber":
                    blk := map[string]interface{}{
                        "difficulty":   "0x480676368",
                        "extraData":    "0x476574682f76312e302e302f6c696e75782f676f312e342e32",
                        "gasLimit":     "0x1388",
                        "gasUsed":      "0x0",
                        "hash":         "0xfeebb1c60ceca18290b0f20aa581d34d293e240fcb6ccb5ee283c007dd5814e2",
                        "logsBloom":    "0x" + strings.Repeat("0", 512),
                        "miner":        "0x28921e4e2C9d84F4c0f0C0cEb991f45751a0fe93",
                        "mixHash":      "0x81434e7b287e3a3bfb45c5a62f8b84795187242d2b8b059426cc7742097f12a2",
                        "nonce":        "0x7098a77b4363303d",
                        "number":       req.Params[0].(string),
                        "parentHash":   "0xc6319dc266cc65771870a9d04800ecc7c624d481e1ff0d6368be5ec2f09b3ff9",
                        "receiptsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
                        "sha3Uncles":   "0x139492965079f29bdf1f4765d7892da50cb0cc85c8ea3641718ec2b8f60526b5",
                        "size":         "0x437",
                        "stateRoot":    "0x7a84186c8bce5654cb92a3913c88fe7d2cf4766b4dd2c1759d9f0ff620ec8d53",
                        "timestamp":    "0x55ba4520",
                        "transactions": []interface{}{"0x76512fy3gff1523r"},
                        "transactionsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
                        "uncles":           []interface{}{"0x9022b8f085f8ae4f6d808213ed42d7edddc7ca61fe6092ef7b9f3ac2b62266d2"},
                    }
                    rep["result"] = blk
                default:
                    rep["result"] = nil
                }
                replies = append(replies, rep)
            }
            json.NewEncoder(w).Encode(replies)
            return
        }
    
        var req struct {
            Method string        `json:"method"`
            ID     int           `json:"id"`
            Params []interface{} `json:"params"`
        }
        if err := json.Unmarshal(raw, &req); err != nil {
            log.Printf("[MOCK]   Decode error: %v", err)
            w.WriteHeader(http.StatusBadRequest)
            return
        }
        rep := map[string]interface{}{
            "jsonrpc": "2.0",
            "id":      req.ID,
        }
        switch req.Method {
        case "eth_blockNumber":
            rep["result"] = "0x64"
        case "eth_getBalance":
            rep["result"] = "0xde0b6b3a7640000"
        case "eth_getBlockByNumber", "eth_getHeaderByNumber":
            rep["result"] = map[string]interface{}{
                "difficulty":   "0x480676368",
                "extraData":    "0x476574682f76312e302e302f6c696e75782f676f312e342e32",
                "gasLimit":     "0x1388",
                "gasUsed":      "0x0",
                "hash":         "0xfeebb1c60ceca18290b0f20aa581d34d293e240fcb6ccb5ee283c007dd5814e2",
                "logsBloom":    "0x" + strings.Repeat("0", 512),
                "miner":        "0x28921e4e2C9d84F4c0f0C0cEb991f45751a0fe93",
                "mixHash":      "0x81434e7b287e3a3bfb45c5a62f8b84795187242d2b8b059426cc7742097f12a2",
                "nonce":        "0x7098a77b4363303d",
                "number":       req.Params[0].(string),
                "parentHash":   "0xc6319dc266cc65771870a9d04800ecc7c624d481e1ff0d6368be5ec2f09b3ff9",
                "receiptsRoot":"0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
                "sha3Uncles":"0x139492965079f29bdf1f4765d7892da50cb0cc85c8ea3641718ec2b8f60526b5",
                "size":"0x437",
                "stateRoot":"0x7a84186c8bce5654cb92a3913c88fe7d2cf4766b4dd2c1759d9f0ff620ec8d53",
                "timestamp":"0x55ba4520",
                "transactions":[]interface{}{"0x76512fy3gff1523r"},
                "transactionsRoot":"0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
                "uncles":[]interface{}{"0x9022b8f085f8ae4f6d808213ed42d7edddc7ca61fe6092ef7b9f3ac2b62266d2"},
            }
        default:
            rep["result"] = nil
        }
        json.NewEncoder(w).Encode(rep)
    })

    return httptest.NewServer(mux)
}


func TestIntegration_BlockRewardAndSyncDuties(t *testing.T) {
	mock := mockQuickNode()
	defer mock.Close()

	ethHTTP, err := ethclient.Dial(mock.URL)
	if err != nil {
		t.Fatalf("ethclient.Dial: %v", err)
	}
	rpcHTTP, err := rpc.DialHTTP(mock.URL)
	if err != nil {
		t.Fatalf("rpc.DialHTTP: %v", err)
	}

	execClient, err := execution.NewExecutionClient(
		rpcHTTP, ethHTTP,
		[]string{},       
		3,               
		100*time.Millisecond,
	)
	if err != nil {
		t.Fatalf("NewExecutionClient: %v", err)
	}
	brUC := usecase.NewBlockRewardUseCase(execClient)


	consClient, err := consensus.NewConsensusClient(    
		mock.URL,       
		3,              
		100*time.Millisecond,
		5*time.Second,  
	)
	if err != nil {
		t.Fatalf("NewConsensusClient: %v", err)
	}
	
	cache, _ := consensus.NewSyncDutiesCache(
        128,
        60*time.Second, 
    )

	sdUC := usecase.NewSyncDutiesUseCase(consClient, cache)

	r := chi.NewRouter()
	h := handler.NewHandler(brUC, sdUC)
	h.Register(r)

	
	rec1 := httptest.NewRecorder()
	req1 := httptest.NewRequest("GET", "/blockreward/100", nil)
	r.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("blockreward status = %d, want 200", rec1.Code)
	}
	var br domain.BlockReward
	if err := json.NewDecoder(rec1.Body).Decode(&br); err != nil {
		t.Fatalf("decoding blockreward: %v", err)
	}
	if br.Status == "" {
		t.Errorf("esperaba status no vacío, got '%s'", br.Status)
	}


	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/syncduties/100", nil)
	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("syncduties status = %d, want 200", rec2.Code)
	}
	var sd domain.SyncDuties
	if err := json.NewDecoder(rec2.Body).Decode(&sd); err != nil {
		t.Fatalf("decoding syncduties: %v", err)
	}
	if len(sd.Validators) != 2 || sd.Validators[0] != "AAA" {
		t.Errorf("syncduties mismatch: %+v", sd.Validators)
	}
}


func TestIntegration_SyncDuties_ErrorScenarios(t *testing.T) {
    cases := []struct {
        name           string
        status         int
        body           string
        wantStatusCode int
    }{
        {
            name:           "400 SlotTooFar",
            status:         http.StatusBadRequest,
            body:           `{"message":"slot too far in future"}`,
            wantStatusCode: http.StatusBadRequest,
        },
        {
            name:           "400 NotActivatedForAltair",
            status:         http.StatusBadRequest,
            body:           `{"message":"not activated for Altair"}`,
            wantStatusCode: http.StatusOK,
        },
        {
            name:           "404 SlotNotFound",
            status:         http.StatusNotFound,
            body:           ``,
            wantStatusCode: http.StatusNotFound,
        },
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            mux := http.NewServeMux()
            mux.HandleFunc("/eth/v1/beacon/states/100/sync_committees", func(w http.ResponseWriter, r *http.Request) {
                w.Header().Set("Content-Type", "application/json")
                w.WriteHeader(tc.status)
                if tc.body != "" {
                    io.WriteString(w, tc.body)
                }
            })
            mux.HandleFunc("/eth/v1/beacon/states/100/validators", func(w http.ResponseWriter, r *http.Request) {
                json.NewEncoder(w).Encode(map[string]interface{}{"data": []interface{}{}})
            })
            mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
                var req struct {
                    Method string `json:"method"`
                    ID     int    `json:"id"`
                }
                _ = json.NewDecoder(r.Body).Decode(&req)
                w.Header().Set("Content-Type", "application/json")
                json.NewEncoder(w).Encode(map[string]interface{}{
                    "jsonrpc": "2.0", "id": req.ID, "result": "0x64",
                })
            })

            server := httptest.NewServer(mux)
            defer server.Close()

            ethHTTP, _ := ethclient.Dial(server.URL)
            rpcHTTP, _ := rpc.DialHTTP(server.URL)
            execClient, _ := execution.NewExecutionClient(rpcHTTP, ethHTTP, []string{}, 1, 10*time.Millisecond)
            brUC := usecase.NewBlockRewardUseCase(execClient)
            consClient, _ := consensus.NewConsensusClient(server.URL, 1, 10*time.Millisecond, 1*time.Second)
            cache, _ := consensus.NewSyncDutiesCache(10, time.Minute)
            sdUC := usecase.NewSyncDutiesUseCase(consClient, cache)

            r := chi.NewRouter()
            h := handler.NewHandler(brUC, sdUC)
            h.Register(r)

            rec := httptest.NewRecorder()
            req := httptest.NewRequest("GET", "/syncduties/100", nil)
            r.ServeHTTP(rec, req)
            if rec.Code != tc.wantStatusCode {
                t.Fatalf("expected %d, got %d (body=%s)", tc.wantStatusCode, rec.Code, rec.Body.String())
            }
        })
    }
}

func TestIntegration_BlockReward_ErrorScenarios(t *testing.T) {
    cases := []struct {
        name           string
        mockBlockNum   string
        headerStatus   int
        wantStatusCode int
    }{
        {
            name:           "SlotInFuture",
            mockBlockNum:   "0x0",  
            headerStatus:   200,
            wantStatusCode: http.StatusBadRequest,
        },
        {
            name:           "HeaderNotFound",
            mockBlockNum:   "0x64", 
            headerStatus:   500,   
            wantStatusCode: http.StatusNotFound,
        },
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            mux := http.NewServeMux()
            mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
                var req struct {
                    Method string        `json:"method"`
                    ID     int           `json:"id"`
                    Params []interface{} `json:"params"`
                }
                _ = json.NewDecoder(r.Body).Decode(&req)
                w.Header().Set("Content-Type", "application/json")

                switch req.Method {
                case "eth_blockNumber":
                    json.NewEncoder(w).Encode(map[string]interface{}{
                        "jsonrpc": "2.0", "id": req.ID, "result": tc.mockBlockNum,
                    })
                case "eth_getBlockByNumber", "eth_getHeaderByNumber":
                    w.WriteHeader(tc.headerStatus)
                    if tc.headerStatus == 200 {
                        num := req.Params[0].(string)
                        json.NewEncoder(w).Encode(map[string]interface{}{
                            "jsonrpc": "2.0", "id": req.ID,
                            "result": map[string]interface{}{"number": num, "miner": "0x00"},
                        })
                    }
                default:
                    json.NewEncoder(w).Encode(map[string]interface{}{
                        "jsonrpc": "2.0", "id": req.ID, "result": "0x0",
                    })
                }
            })

            server := httptest.NewServer(mux)
            defer server.Close()

            ethHTTP, _ := ethclient.Dial(server.URL)
            rpcHTTP, _ := rpc.DialHTTP(server.URL)
            execClient, _ := execution.NewExecutionClient(rpcHTTP, ethHTTP, []string{}, 1, 10*time.Millisecond)
            brUC := usecase.NewBlockRewardUseCase(execClient)

            r := chi.NewRouter()
            h := handler.NewHandler(brUC, usecase.NewSyncDutiesUseCase(nil, nil))
            h.Register(r)

            rec := httptest.NewRecorder()
            req := httptest.NewRequest("GET", "/blockreward/100", nil)
            r.ServeHTTP(rec, req)
            if rec.Code != tc.wantStatusCode {
                t.Fatalf("expected %d, got %d", tc.wantStatusCode, rec.Code)
            }
        })
    }
}
