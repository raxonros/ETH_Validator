package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi"
	"go.uber.org/zap"

	"eth_validator_api/internal/domain"
	apierr "eth_validator_api/internal/errors"
	"eth_validator_api/internal/handler"
	"eth_validator_api/internal/usecase"
	stdErr "errors"
)


type mockBR struct{}

func (m *mockBR) GetBlockReward(ctx context.Context, slot uint64) (domain.BlockReward, error) {
	return domain.BlockReward{Status: "vanilla", Reward: 1.0}, nil
}
func (m *mockBR) GetSyncDuties(ctx context.Context, slot uint64) (domain.SyncDuties, error) {
	return domain.SyncDuties{}, nil
}

type mockSD struct{}

func (m *mockSD) GetSyncDuties(ctx context.Context, slot uint64) (domain.SyncDuties, error) {
	return domain.SyncDuties{Validators: []string{"A", "B"}}, nil
}
func (m *mockSD) GetBlockReward(ctx context.Context, slot uint64) (domain.BlockReward, error) {
	return domain.BlockReward{}, nil
}

type dummyCache struct{}

func (c *dummyCache) Get(slot uint64) (domain.SyncDuties, bool) { return domain.SyncDuties{}, false }
func (c *dummyCache) Add(slot uint64, d domain.SyncDuties)      {}

func TestHTTPHandler(t *testing.T) {
	zap.ReplaceGlobals(zap.NewNop())

	brUC := usecase.NewBlockRewardUseCase(&mockBR{})
	sdUC := usecase.NewSyncDutiesUseCase(&mockSD{}, &dummyCache{})
	h := handler.NewHandler(brUC, sdUC)

	r := chi.NewRouter()
	h.Register(r)

	req1 := httptest.NewRequest("GET", "/blockreward/1", nil)
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("esperaba 200, got %d", rec1.Code)
	}
	var br domain.BlockReward
	if err := json.NewDecoder(rec1.Body).Decode(&br); err != nil {
		t.Fatalf("decoding err: %v", err)
	}
	if br.Status != "vanilla" {
		t.Errorf("status inesperado: %s", br.Status)
	}

	req2 := httptest.NewRequest("GET", "/syncduties/1", nil)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("esperaba 200, got %d", rec2.Code)
	}
	var sd domain.SyncDuties
	if err := json.NewDecoder(rec2.Body).Decode(&sd); err != nil {
		t.Fatalf("decoding err: %v", err)
	}
	if len(sd.Validators) != 2 {
		t.Errorf("validators inesperados: %+v", sd.Validators)
	}
}


type errorMockClient struct{}

func (m *errorMockClient) GetBlockReward(ctx context.Context, slot uint64) (domain.BlockReward, error) {
	if slot == 12345 {
		return domain.BlockReward{}, apierr.ErrSlotNotFound
	}
	return domain.BlockReward{}, apierr.ErrSlotInFuture
}
func (m *errorMockClient) GetSyncDuties(ctx context.Context, slot uint64) (domain.SyncDuties, error) {
	return domain.SyncDuties{}, nil
}

func TestGetBlockReward_Errors(t *testing.T) {
	zap.ReplaceGlobals(zap.NewNop())

	brUC := usecase.NewBlockRewardUseCase(&errorMockClient{})
	sdUC := usecase.NewSyncDutiesUseCase(&mockSD{}, &dummyCache{}) 
	h := handler.NewHandler(brUC, sdUC)

	r := chi.NewRouter()
	h.Register(r)

	cases := []struct {
		url        string
		wantStatus int
		wantErr    string
	}{
		{"/blockreward/abc", http.StatusBadRequest, "invalid slot"},
		{"/blockreward/999999999999", http.StatusBadRequest, "slot in future"},
		{"/blockreward/12345", http.StatusNotFound, "slot not found"},
	}

	for _, c := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", c.url, nil)
		r.ServeHTTP(rec, req)

		if rec.Code != c.wantStatus {
			t.Errorf("%s: esperado %d, obtuvo %d", c.url, c.wantStatus, rec.Code)
		}
		var body map[string]string
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("decoding err: %v", err)
		}
		if body["error"] != c.wantErr {
			t.Errorf("%s: mensaje esperado %q, obtuvo %q", c.url, c.wantErr, body["error"])
		}
	}
}


type errorSDClient struct{}

func (m *errorSDClient) GetSyncDuties(ctx context.Context, slot uint64) (domain.SyncDuties, error) {
	switch slot {
	case 10:
		return domain.SyncDuties{}, apierr.ErrSlotTooFarInFuture
	case 20:
		return domain.SyncDuties{}, apierr.ErrSlotNotFound
	}
	return domain.SyncDuties{}, stdErr.New("boom interno")
}
func (m *errorSDClient) GetBlockReward(ctx context.Context, slot uint64) (domain.BlockReward, error) {
	return domain.BlockReward{}, nil
}

func TestGetSyncDuties_Errors(t *testing.T) {
	zap.ReplaceGlobals(zap.NewNop())

	brUC := usecase.NewBlockRewardUseCase(&mockBR{}) 
	sdUC := usecase.NewSyncDutiesUseCase(&errorSDClient{}, &dummyCache{})
	h := handler.NewHandler(brUC, sdUC)

	r := chi.NewRouter()
	h.Register(r)

	cases := []struct {
		url        string
		wantStatus int
		wantErr    string
	}{
		{"/syncduties/abc", http.StatusBadRequest, "invalid slot"},
		{"/syncduties/10", http.StatusBadRequest, "slot too far in future"},
		{"/syncduties/20", http.StatusNotFound, "slot not found"},
		{"/syncduties/999", http.StatusInternalServerError, "internal error"},
	}

	for _, c := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", c.url, nil)
		r.ServeHTTP(rec, req)

		if rec.Code != c.wantStatus {
			t.Errorf("%s: esperado %d, obtuvo %d", c.url, c.wantStatus, rec.Code)
		}
		var body map[string]string
		json.NewDecoder(rec.Body).Decode(&body)
		if body["error"] != c.wantErr {
			t.Errorf("%s: mensaje esperado %q, obtuvo %q", c.url, c.wantErr, body["error"])
		}
	}
}

