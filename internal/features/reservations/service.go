package reservations

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ClaimBook interface {
	Confirm(ctx context.Context, claim SeatClaim) error
	Delete(ctx context.Context, claim SeatClaim) error
	Find(ctx context.Context, token string) (SeatClaim, error)
	ListByFilm(ctx context.Context, filmID string) ([]SeatClaim, error)
	Reserve(ctx context.Context, claim SeatClaim) error
}

type Desk struct {
	book    ClaimBook
	holdTTL time.Duration
	now     func() time.Time
}

func NewDesk(book ClaimBook, holdTTL time.Duration) *Desk {
	return &Desk{
		book:    book,
		holdTTL: holdTTL,
		now:     time.Now,
	}
}

func (desk *Desk) Hold(ctx context.Context, cmd HoldCommand) (SeatClaim, error) {
	cmd = cmd.normalize()
	if cmd.FilmID == "" || cmd.SeatCode == "" || cmd.CustomerID == "" {
		return SeatClaim{}, ErrInvalidInput
	}

	claim := SeatClaim{
		Token:      uuid.NewString(),
		FilmID:     cmd.FilmID,
		SeatCode:   cmd.SeatCode,
		CustomerID: cmd.CustomerID,
		State:      ClaimHeld,
		ExpiresAt:  desk.now().Add(desk.holdTTL),
	}
	if err := desk.book.Reserve(ctx, claim); err != nil {
		return SeatClaim{}, err
	}
	return claim, nil
}

func (desk *Desk) Seats(ctx context.Context, filmID string) ([]SeatClaim, error) {
	if filmID == "" {
		return nil, ErrInvalidInput
	}
	return desk.book.ListByFilm(ctx, filmID)
}

func (desk *Desk) Confirm(ctx context.Context, cmd ConfirmCommand) (SeatClaim, error) {
	cmd = cmd.normalize()
	if cmd.Token == "" || cmd.CustomerID == "" {
		return SeatClaim{}, ErrInvalidInput
	}

	claim, err := desk.book.Find(ctx, cmd.Token)
	if err != nil {
		return SeatClaim{}, err
	}
	if err := ensureSameCustomer(claim, cmd.CustomerID); err != nil {
		return SeatClaim{}, err
	}

	claim.State = ClaimConfirmed
	claim.ExpiresAt = time.Time{}
	if err := desk.book.Confirm(ctx, claim); err != nil {
		return SeatClaim{}, err
	}
	return claim, nil
}

func (desk *Desk) Release(ctx context.Context, cmd ReleaseCommand) error {
	cmd = cmd.normalize()
	if cmd.Token == "" || cmd.CustomerID == "" {
		return ErrInvalidInput
	}

	claim, err := desk.book.Find(ctx, cmd.Token)
	if err != nil {
		return err
	}
	if err := ensureSameCustomer(claim, cmd.CustomerID); err != nil {
		return err
	}
	if claim.State == ClaimConfirmed {
		return ErrClaimFinalized
	}
	return desk.book.Delete(ctx, claim)
}
