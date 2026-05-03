package reservations

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrClaimFinalized = errors.New("reservation claim is already confirmed")
	ErrClaimNotFound  = errors.New("reservation claim was not found")
	ErrInvalidInput   = errors.New("reservation request is invalid")
	ErrSeatTaken      = errors.New("seat is already reserved")
	ErrWrongCustomer  = errors.New("reservation claim belongs to a different customer")
)

type ClaimState string

const (
	ClaimHeld      ClaimState = "held"
	ClaimConfirmed ClaimState = "confirmed"
)

type SeatClaim struct {
	Token      string
	FilmID     string
	SeatCode   string
	CustomerID string
	State      ClaimState
	ExpiresAt  time.Time
}

type HoldCommand struct {
	FilmID     string
	SeatCode   string
	CustomerID string
}

type ConfirmCommand struct {
	Token      string
	CustomerID string
}

type ReleaseCommand struct {
	Token      string
	CustomerID string
}

func (cmd HoldCommand) normalize() HoldCommand {
	cmd.FilmID = strings.TrimSpace(cmd.FilmID)
	cmd.SeatCode = strings.ToUpper(strings.TrimSpace(cmd.SeatCode))
	cmd.CustomerID = strings.TrimSpace(cmd.CustomerID)
	return cmd
}

func (cmd ConfirmCommand) normalize() ConfirmCommand {
	cmd.Token = strings.TrimSpace(cmd.Token)
	cmd.CustomerID = strings.TrimSpace(cmd.CustomerID)
	return cmd
}

func (cmd ReleaseCommand) normalize() ReleaseCommand {
	cmd.Token = strings.TrimSpace(cmd.Token)
	cmd.CustomerID = strings.TrimSpace(cmd.CustomerID)
	return cmd
}

func ensureSameCustomer(claim SeatClaim, customerID string) error {
	if claim.CustomerID != customerID {
		return ErrWrongCustomer
	}
	return nil
}
