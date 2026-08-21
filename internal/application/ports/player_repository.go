package ports

import (
	"context"
	"time"

	"royalescout/internal/domain/player"
)

type PlayerRepository interface {
	SearchByRating(ctx context.Context, minimum, maximum int) ([]player.Player, error)
	ReplaceSnapshot(ctx context.Context, players []player.Player, syncedAt time.Time) error
	ClearActive(ctx context.Context) error
	Count(ctx context.Context) (int, error)
}

type MetadataRepository interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key, value string) error
}
