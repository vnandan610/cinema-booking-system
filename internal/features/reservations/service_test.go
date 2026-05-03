package reservations

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDeskAllowsOnlyOneHoldForASeat(t *testing.T) {
	repo := newMemoryClaimBook()
	desk := NewDesk(repo, time.Minute)

	const attempts = 1000
	var successes atomic.Int64
	var wg sync.WaitGroup
	wg.Add(attempts)

	for i := 0; i < attempts; i++ {
		go func() {
			defer wg.Done()
			_, err := desk.Hold(context.Background(), HoldCommand{
				FilmID:     "inception",
				SeatCode:   "A1",
				CustomerID: "customer",
			})
			if err == nil {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()

	if got := successes.Load(); got != 1 {
		t.Fatalf("expected exactly one successful hold, got %d", got)
	}
}

func TestDeskRejectsReleaseByDifferentCustomer(t *testing.T) {
	repo := newMemoryClaimBook()
	desk := NewDesk(repo, time.Minute)

	claim, err := desk.Hold(context.Background(), HoldCommand{
		FilmID:     "dune",
		SeatCode:   "B3",
		CustomerID: "owner",
	})
	if err != nil {
		t.Fatalf("hold: %v", err)
	}

	err = desk.Release(context.Background(), ReleaseCommand{
		Token:      claim.Token,
		CustomerID: "stranger",
	})
	if err != ErrWrongCustomer {
		t.Fatalf("expected ErrWrongCustomer, got %v", err)
	}
}

type memoryClaimBook struct {
	mu       sync.Mutex
	byToken  map[string]SeatClaim
	bySeatID map[string]string
}

func newMemoryClaimBook() *memoryClaimBook {
	return &memoryClaimBook{
		byToken:  map[string]SeatClaim{},
		bySeatID: map[string]string{},
	}
}

func (book *memoryClaimBook) Reserve(_ context.Context, claim SeatClaim) error {
	book.mu.Lock()
	defer book.mu.Unlock()

	seat := claim.FilmID + ":" + claim.SeatCode
	if _, exists := book.bySeatID[seat]; exists {
		return ErrSeatTaken
	}
	book.bySeatID[seat] = claim.Token
	book.byToken[claim.Token] = claim
	return nil
}

func (book *memoryClaimBook) ListByFilm(_ context.Context, filmID string) ([]SeatClaim, error) {
	book.mu.Lock()
	defer book.mu.Unlock()

	var claims []SeatClaim
	for _, claim := range book.byToken {
		if claim.FilmID == filmID {
			claims = append(claims, claim)
		}
	}
	return claims, nil
}

func (book *memoryClaimBook) Find(_ context.Context, token string) (SeatClaim, error) {
	book.mu.Lock()
	defer book.mu.Unlock()

	claim, ok := book.byToken[token]
	if !ok {
		return SeatClaim{}, ErrClaimNotFound
	}
	return claim, nil
}

func (book *memoryClaimBook) Confirm(_ context.Context, claim SeatClaim) error {
	book.mu.Lock()
	defer book.mu.Unlock()

	book.byToken[claim.Token] = claim
	return nil
}

func (book *memoryClaimBook) Delete(_ context.Context, claim SeatClaim) error {
	book.mu.Lock()
	defer book.mu.Unlock()

	delete(book.byToken, claim.Token)
	delete(book.bySeatID, claim.FilmID+":"+claim.SeatCode)
	return nil
}
