package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"royalescout/internal/domain/player"
)

func TestReplaceSnapshotPublishesAtomicallyAndRemovesFormerPlayers(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "royale-scout.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	firstSnapshot := []player.Player{
		{Tag: "#ONE", NicknameOriginal: "One", NicknameNormalized: "one", RankedRating: 2900, CountryID: 1, CountryCode: "AA"},
		{Tag: "#TWO", NicknameOriginal: "Two", NicknameNormalized: "two", RankedRating: 2910, CountryID: 2, CountryCode: "BB"},
	}
	if err := store.ReplaceSnapshot(ctx, firstSnapshot, time.Unix(100, 0)); err != nil {
		t.Fatalf("ReplaceSnapshot() error = %v", err)
	}

	secondSnapshot := []player.Player{
		{Tag: "#TWO", NicknameOriginal: "Two updated", NicknameNormalized: "twoupdated", RankedRating: 2920, CountryID: 2, CountryCode: "BB"},
	}
	if err := store.ReplaceSnapshot(ctx, secondSnapshot, time.Unix(200, 0)); err != nil {
		t.Fatalf("ReplaceSnapshot() error = %v", err)
	}

	count, err := store.Count(ctx)
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("Count() = %d, want 1", count)
	}
	players, err := store.SearchByRating(ctx, 2900, 2950)
	if err != nil {
		t.Fatalf("SearchByRating() error = %v", err)
	}
	if len(players) != 1 || players[0].Tag != "#TWO" || players[0].NicknameOriginal != "Two updated" {
		t.Fatalf("players = %#v", players)
	}
}

func TestMetadataRoundTrip(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "royale-scout.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if _, found, err := store.Get(ctx, "season"); err != nil || found {
		t.Fatalf("Get() before set = found %t, error %v", found, err)
	}
	if err := store.Set(ctx, "season", "2026-07"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	value, found, err := store.Get(ctx, "season")
	if err != nil || !found || value != "2026-07" {
		t.Fatalf("Get() = %q, found %t, error %v", value, found, err)
	}
}
