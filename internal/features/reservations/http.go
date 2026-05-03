package reservations

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/sikozonpc/cinema/internal/platform/web"
)

type HTTPHandler struct {
	desk   *Desk
	logger *slog.Logger
}

type customerRequest struct {
	CustomerID string `json:"customer_id"`
	UserID     string `json:"user_id,omitempty"`
}

type claimResponse struct {
	ClaimID    string `json:"claim_id"`
	FilmID     string `json:"film_id"`
	SeatCode   string `json:"seat_code"`
	CustomerID string `json:"customer_id"`
	State      string `json:"state"`
	ExpiresAt  string `json:"expires_at,omitempty"`
}

type seatResponse struct {
	SeatCode   string `json:"seat_code"`
	CustomerID string `json:"customer_id"`
	Reserved   bool   `json:"reserved"`
	Confirmed  bool   `json:"confirmed"`
	State      string `json:"state"`
}

func NewHTTPHandler(desk *Desk, logger *slog.Logger) *HTTPHandler {
	return &HTTPHandler{desk: desk, logger: logger}
}

func (handler *HTTPHandler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/films/{filmID}/seats", handler.listSeats)
	mux.HandleFunc("POST /api/v1/films/{filmID}/seats/{seatCode}/claims", handler.createClaim)
	mux.HandleFunc("PUT /api/v1/claims/{claimID}/confirm", handler.confirmClaim)
	mux.HandleFunc("DELETE /api/v1/claims/{claimID}", handler.releaseClaim)
}

func (handler *HTTPHandler) listSeats(w http.ResponseWriter, r *http.Request) {
	claims, err := handler.desk.Seats(r.Context(), r.PathValue("filmID"))
	if err != nil {
		handler.writeError(w, r, err)
		return
	}

	seats := make([]seatResponse, 0, len(claims))
	for _, claim := range claims {
		seats = append(seats, seatResponse{
			SeatCode:   claim.SeatCode,
			CustomerID: claim.CustomerID,
			Reserved:   true,
			Confirmed:  claim.State == ClaimConfirmed,
			State:      string(claim.State),
		})
	}
	web.JSON(w, http.StatusOK, map[string]any{"seats": seats})
}

func (handler *HTTPHandler) createClaim(w http.ResponseWriter, r *http.Request) {
	var req customerRequest
	if err := web.DecodeJSON(r, &req); err != nil {
		web.Problem(w, r, http.StatusBadRequest, "bad_json", "Request body must be valid JSON")
		return
	}

	claim, err := handler.desk.Hold(r.Context(), HoldCommand{
		FilmID:     r.PathValue("filmID"),
		SeatCode:   r.PathValue("seatCode"),
		CustomerID: req.customerID(),
	})
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	web.JSON(w, http.StatusCreated, toClaimResponse(claim))
}

func (handler *HTTPHandler) confirmClaim(w http.ResponseWriter, r *http.Request) {
	var req customerRequest
	if err := web.DecodeJSON(r, &req); err != nil {
		web.Problem(w, r, http.StatusBadRequest, "bad_json", "Request body must be valid JSON")
		return
	}

	claim, err := handler.desk.Confirm(r.Context(), ConfirmCommand{
		Token:      r.PathValue("claimID"),
		CustomerID: req.customerID(),
	})
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	web.JSON(w, http.StatusOK, toClaimResponse(claim))
}

func (handler *HTTPHandler) releaseClaim(w http.ResponseWriter, r *http.Request) {
	var req customerRequest
	if err := web.DecodeJSON(r, &req); err != nil {
		web.Problem(w, r, http.StatusBadRequest, "bad_json", "Request body must be valid JSON")
		return
	}

	err := handler.desk.Release(r.Context(), ReleaseCommand{
		Token:      r.PathValue("claimID"),
		CustomerID: req.customerID(),
	})
	if err != nil {
		handler.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (handler *HTTPHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		web.Problem(w, r, http.StatusBadRequest, "invalid_request", "Missing or invalid reservation data")
	case errors.Is(err, ErrSeatTaken):
		web.Problem(w, r, http.StatusConflict, "seat_unavailable", "Seat is already reserved")
	case errors.Is(err, ErrClaimNotFound):
		web.Problem(w, r, http.StatusNotFound, "claim_not_found", "Reservation claim was not found or has expired")
	case errors.Is(err, ErrWrongCustomer):
		web.Problem(w, r, http.StatusForbidden, "customer_mismatch", "Reservation claim belongs to another customer")
	case errors.Is(err, ErrClaimFinalized):
		web.Problem(w, r, http.StatusConflict, "claim_finalized", "Confirmed seats cannot be released")
	default:
		handler.logger.Error("reservation request failed", "error", err, "request_id", web.RequestID(r.Context()))
		web.Problem(w, r, http.StatusInternalServerError, "reservation_error", "Reservation request failed")
	}
}

func toClaimResponse(claim SeatClaim) claimResponse {
	resp := claimResponse{
		ClaimID:    claim.Token,
		FilmID:     claim.FilmID,
		SeatCode:   claim.SeatCode,
		CustomerID: claim.CustomerID,
		State:      string(claim.State),
	}
	if !claim.ExpiresAt.IsZero() {
		resp.ExpiresAt = claim.ExpiresAt.UTC().Format(time.RFC3339)
	}
	return resp
}

func (req customerRequest) customerID() string {
	if req.CustomerID != "" {
		return req.CustomerID
	}
	return req.UserID
}
