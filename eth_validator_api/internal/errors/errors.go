package errors

import "net/http"

type HTTPError interface {
    error
    StatusCode() int
}

type apiError struct {
    msg  string
    code int
}

func (e *apiError) Error() string     { return e.msg }
func (e *apiError) StatusCode() int   { return e.code }

var (
    ErrSlotInFuture       = &apiError{msg: "slot in future", code: http.StatusBadRequest}
    ErrSlotNotFound       = &apiError{msg: "slot not found", code: http.StatusNotFound}
    ErrSlotTooFarInFuture = &apiError{msg: "slot too far in future", code: http.StatusBadRequest}

	ErrRequestTimeout     = &apiError{"request timed out", http.StatusGatewayTimeout}
	ErrInternal           = &apiError{msg: "internal server error", code: http.StatusInternalServerError}
)