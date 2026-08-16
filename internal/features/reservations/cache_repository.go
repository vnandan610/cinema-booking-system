package reservations

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/vnandan610/cinema-booking-system/internal/platform/cache"
)

type CacheClaimBook struct {
	cache   cache.Store
	holdTTL time.Duration
}

type claimRecord struct {
	Token      string     `json:"token"`
	FilmID     string     `json:"film_id"`
	SeatCode   string     `json:"seat_code"`
	CustomerID string     `json:"customer_id"`
	State      ClaimState `json:"state"`
	ExpiresAt  time.Time  `json:"expires_at,omitempty"`
}

func NewCacheClaimBook(cacheStore cache.Store, holdTTL time.Duration) *CacheClaimBook {
	return &CacheClaimBook{cache: cacheStore, holdTTL: holdTTL}
}

func (book *CacheClaimBook) Reserve(ctx context.Context, claim SeatClaim) error {
	claimed, err := book.cache.SetJSONPairIfAbsent(ctx, seatKey(claim.FilmID, claim.SeatCode), toRecord(claim), claimKey(claim.Token), seatKey(claim.FilmID, claim.SeatCode), book.holdTTL)
	if err != nil {
		return err
	}
	if !claimed {
		return ErrSeatTaken
	}
	return nil
}

func (book *CacheClaimBook) ListByFilm(ctx context.Context, filmID string) ([]SeatClaim, error) {
	keys, err := book.cache.Keys(ctx, fmt.Sprintf("cinema:v1:seat:%s:*", filmID))
	if err != nil {
		return nil, err
	}

	claims := make([]SeatClaim, 0, len(keys))
	for _, key := range keys {
		var record claimRecord
		if err := book.cache.GetJSON(ctx, key, &record); err != nil {
			if errors.Is(err, cache.ErrMiss) {
				continue
			}
			return nil, err
		}
		claims = append(claims, record.toClaim())
	}
	return claims, nil
}

func (book *CacheClaimBook) Find(ctx context.Context, token string) (SeatClaim, error) {
	key, err := book.cache.GetString(ctx, claimKey(token))
	if errors.Is(err, cache.ErrMiss) {
		return SeatClaim{}, ErrClaimNotFound
	}
	if err != nil {
		return SeatClaim{}, err
	}

	var record claimRecord
	if err := book.cache.GetJSON(ctx, key, &record); err != nil {
		if errors.Is(err, cache.ErrMiss) {
			return SeatClaim{}, ErrClaimNotFound
		}
		return SeatClaim{}, err
	}
	return record.toClaim(), nil
}

func (book *CacheClaimBook) Confirm(ctx context.Context, claim SeatClaim) error {
	if err := book.cache.SetJSON(ctx, seatKey(claim.FilmID, claim.SeatCode), toRecord(claim), 0); err != nil {
		return err
	}
	return book.cache.SetString(ctx, claimKey(claim.Token), seatKey(claim.FilmID, claim.SeatCode), 0)
}

func (book *CacheClaimBook) Delete(ctx context.Context, claim SeatClaim) error {
	return book.cache.Delete(ctx, seatKey(claim.FilmID, claim.SeatCode), claimKey(claim.Token))
}

func toRecord(claim SeatClaim) claimRecord {
	return claimRecord{
		Token:      claim.Token,
		FilmID:     claim.FilmID,
		SeatCode:   claim.SeatCode,
		CustomerID: claim.CustomerID,
		State:      claim.State,
		ExpiresAt:  claim.ExpiresAt,
	}
}

func (record claimRecord) toClaim() SeatClaim {
	return SeatClaim{
		Token:      record.Token,
		FilmID:     record.FilmID,
		SeatCode:   record.SeatCode,
		CustomerID: record.CustomerID,
		State:      record.State,
		ExpiresAt:  record.ExpiresAt,
	}
}

func seatKey(filmID string, seatCode string) string {
	return fmt.Sprintf("cinema:v1:seat:%s:%s", filmID, seatCode)
}

func claimKey(token string) string {
	return fmt.Sprintf("cinema:v1:claim:%s", token)
}
