package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
        body, _ := io.ReadAll(r.Body)
        r.Body = io.NopCloser(bytes.NewReader(body))

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "data": map[string][]string{"validators": {"AAA", "BBB"}},
        })
    })

	mux.HandleFunc("/eth/v1/beacon/states/100/validators", func(w http.ResponseWriter, r *http.Request) {
		ids := r.URL.Query().Get("id")
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
        raw, _ := io.ReadAll(r.Body)
        r.Body = io.NopCloser(bytes.NewReader(raw))

        var req struct {
            Method string        `json:"method"`
            ID     int           `json:"id"`
            Params []interface{} `json:"params"`
        }
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            w.WriteHeader(http.StatusBadRequest)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        switch req.Method {
        case "eth_blockNumber":
            json.NewEncoder(w).Encode(map[string]interface{}{
                "jsonrpc": "2.0", "id": req.ID, "result": "0x64",
            })
        case "eth_getBalance":
            json.NewEncoder(w).Encode(map[string]interface{}{
                "jsonrpc": "2.0", "id": req.ID, "result": "0xde0b6b3a7640000",
            })
		case "eth_getBlockByNumber", "eth_getHeaderByNumber":
			blockNum := req.Params[0].(string)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]interface{}{
					"difficulty":       "0x42be722b6",
					"extraData":        "0x476574682f4c5649562f76312e302e302f6c696e75782f676f312e342e32",
					"gasLimit":         "0x1388",
					"gasUsed":          "0x0",
					"hash":             "0xdfe2e70d6c116a541101cecbb256d7402d62125f6ddc9b607d49edc989825c64",
					"logsBloom":        "0x" + strings.Repeat("0", 512),
					"miner":            "0xbb7b8287f3f0a933474a79eae42cbca977791171",
					"mixHash":          "0x5bb43c0772e58084b221c8e0c859a45950c103c712c5b8f11d9566ee078a4501",
					"nonce":            "0x37129c7f29a9364b",
					"number":           blockNum,
					"parentHash":       "0xdb10afd3efa45327eb284c83cc925bd9bd7966aea53067c1eebe0724d124ec1e",
					"receiptsRoot":     "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
					"sha3Uncles":       "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
					"size":             "0x21e",
					"stateRoot":        "0x90c25f6d7fddeb31a6cc5668a6bba77adbadec705eb7aa5a51265c2d1e3bb7ac",
					"timestamp":        "0x55ba43eb",
					"transactions":     []interface{}{},
					"transactionsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
					"uncles":           []interface{}{},
					"baseFeePerGas":    "0x0",
				},
			})
	
		
        default:
            json.NewEncoder(w).Encode(map[string]interface{}{
                "jsonrpc": "2.0", "id": req.ID, "result": nil,
            })
        }
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
            consClient, _ := consensus.NewConsensusClient(server.URL, server.URL, 1, 10*time.Millisecond, 1*time.Second)
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
