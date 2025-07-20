package handler

import (
    "encoding/json"
    "net/http"
    "strconv"

    "github.com/go-chi/chi"
    "go.uber.org/zap"

    "eth_validator_api/internal/errors"
    "eth_validator_api/internal/usecase"
)

type Handler struct {
    brUseCase *usecase.BlockRewardUseCase
    sdUseCase *usecase.SyncDutiesUseCase
}

func NewHandler(br *usecase.BlockRewardUseCase, sd *usecase.SyncDutiesUseCase) *Handler {
    return &Handler{brUseCase: br, sdUseCase: sd}
}

func (h *Handler) Register(r chi.Router) {
    r.Get("/blockreward/{slot}", h.getBlockReward)
    r.Get("/syncduties/{slot}", h.getSyncDuties)
}

func (h *Handler) getBlockReward(w http.ResponseWriter, r *http.Request) {
    slotStr := chi.URLParam(r, "slot")
    slot, err := strconv.ParseUint(slotStr, 10, 64)
    if err != nil {
        zap.L().Error("invalid slot param", zap.Error(err))
        writeErrorJSON(w, http.StatusBadRequest, "invalid slot")
        return
    }
    result, err := h.brUseCase.Execute(r.Context(), slot)
    if err != nil {
        if he, ok := err.(errors.HTTPError); ok {
            writeErrorJSON(w, he.StatusCode(), he.Error())
            return
        }
        zap.L().Error("unexpected block reward error", zap.Error(err))
        writeErrorJSON(w, http.StatusInternalServerError, "internal error")
        return
    }
    writeJSON(w, result)
	
}

func (h *Handler) getSyncDuties(w http.ResponseWriter, r *http.Request) {
    slotStr := chi.URLParam(r, "slot")
    slot, err := strconv.ParseUint(slotStr, 10, 64)
    if err != nil {
        zap.L().Error("invalid slot param", zap.Error(err))
        writeErrorJSON(w, http.StatusBadRequest, "invalid slot")
        return
    }
    result, err := h.sdUseCase.Execute(r.Context(), slot)
    if err != nil {
        if he, ok := err.(errors.HTTPError); ok {
            writeErrorJSON(w, he.StatusCode(), he.Error())
            return
        }
        zap.L().Error("unexpected sync duties error", zap.Error(err))
        writeErrorJSON(w, http.StatusInternalServerError, "internal error")
        return
    }
    writeJSON(w, result)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
    w.Header().Set("Content-Type", "application/json")
    if err := json.NewEncoder(w).Encode(v) ; err != nil {
        zap.L().Error("failed to write JSON response", zap.Error(err))     
    }
}

func writeErrorJSON(w http.ResponseWriter, status int, msg string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    if err := json.NewEncoder(w).Encode(struct {
        Error string `json:"error"`
    }{Error: msg}); err != nil {
        zap.L().Error("failed to write JSON error response", zap.Error(err))
    }
}
