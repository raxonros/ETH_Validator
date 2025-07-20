package http

import (
    "encoding/json"
    "net/http"

    "github.com/go-chi/chi"
    "github.com/go-chi/chi/middleware"
	"go.uber.org/zap"

    "eth_validator_api/internal/handler"
    "eth_validator_api/internal/usecase"
    "eth_validator_api/pkg/config"
)

func NewRouter(
    cfg *config.Config,
    brUC *usecase.BlockRewardUseCase,
    sdUC *usecase.SyncDutiesUseCase,
) *chi.Mux {
    r := chi.NewRouter()

    r.Use(middleware.RequestID)
    r.Use(middleware.RealIP)
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)

    r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			zap.L().Error("failed to encode health check response", zap.Error(err))
		}
    })

    h := handler.NewHandler(brUC, sdUC)
    h.Register(r)

    return r
}
