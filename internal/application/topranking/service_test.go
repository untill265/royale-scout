package topranking

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"royalescout/internal/application/battlelog"
	"royalescout/internal/domain/topplayer"
)

func TestSnapshotAndDeckBatchUseCurrentTopAndCache(t *testing.T) {
	syncedAt := time.Unix(100, 0).UTC()
	repository := fakeRepository{players: []topplayer.Player{
		{Rank: 1, Tag: "#ONE", Name: "One", RankedRating: 3200, ClanName: "Clan", ClanBadgeID: 16000001, SyncedAt: syncedAt},
		{Rank: 2, Tag: "#TWO", Name: "Two", RankedRating: 3190, SyncedAt: syncedAt},
	}}
	provider := &fakeDeckProvider{}
	service := New(repository, provider)

	snapshot, err := service.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if len(snapshot.Players) != 2 || snapshot.Players[0].ClanBadgeID != 16000001 || snapshot.UpdatedAt != syncedAt.Format(time.RFC3339) {
		t.Fatalf("Snapshot() = %#v", snapshot)
	}

	for range 2 {
		batch, err := service.Decks(context.Background(), []string{"#ONE", "#TWO"})
		if err != nil {
			t.Fatalf("Decks() error = %v", err)
		}
		if len(batch.Players) != 2 || len(batch.Players[0].Matches) != 1 {
			t.Fatalf("Decks() = %#v", batch)
		}
	}
	if calls := provider.calls.Load(); calls != 2 {
		t.Fatalf("deck provider calls = %d, want one per player", calls)
	}
	if _, err := service.Decks(context.Background(), []string{"#OUTSIDE"}); err == nil {
		t.Fatal("Decks() accepted a player outside the top")
	}
}

type fakeRepository struct {
	players []topplayer.Player
}

func (repository fakeRepository) ReplaceTopSnapshot(context.Context, []topplayer.Player, time.Time) error {
	return nil
}

func (repository fakeRepository) ListTop(context.Context) ([]topplayer.Player, error) {
	return repository.players, nil
}

func (repository fakeRepository) ClearTop(context.Context) error {
	return nil
}

type fakeDeckProvider struct {
	calls atomic.Int32
}

func (provider *fakeDeckProvider) LatestRankedDecksLimit(_ context.Context, playerTag string, limit int) (battlelog.Result, error) {
	provider.calls.Add(1)
	return battlelog.Result{PlayerTag: playerTag, Matches: []battlelog.DeckMatch{
		{BattleTime: "2026-08-20T12:00:00Z", AverageElixir: 3.4},
		{BattleTime: "2026-08-20T11:00:00Z", AverageElixir: 3.7},
	}[:limit]}, nil
}
