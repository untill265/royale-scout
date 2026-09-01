package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"royalescout/internal/application/ports"
	"royalescout/internal/domain/player"
	"royalescout/internal/domain/topplayer"
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

func TestReplaceLocationSnapshotsRefreshesOnlySuccessfulLocations(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "royale-scout.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	initial := []player.Player{
		{Tag: "#OLD-AA", NicknameOriginal: "Old AA", NicknameNormalized: "oldaa", RankedRating: 2800, CountryID: 1, CountryCode: "AA"},
		{Tag: "#OLD-BB", NicknameOriginal: "Old BB", NicknameNormalized: "oldbb", RankedRating: 2810, CountryID: 2, CountryCode: "BB"},
	}
	if err := store.ReplaceSnapshot(ctx, initial, time.Unix(100, 0)); err != nil {
		t.Fatalf("ReplaceSnapshot() error = %v", err)
	}

	refreshedAt := time.Unix(200, 0).UTC()
	err = store.ReplaceLocationSnapshots(ctx, []ports.LocationPlayerSnapshot{
		{
			LocationID: 1,
			Players: []player.Player{
				{Tag: "#FRESH-AA", NicknameOriginal: "Fresh AA", NicknameNormalized: "freshaa", RankedRating: 2900, CountryID: 1, CountryCode: "AA"},
			},
		},
	}, refreshedAt)
	if err != nil {
		t.Fatalf("ReplaceLocationSnapshots() error = %v", err)
	}

	players, err := store.SearchByRating(ctx, 0, 99999)
	if err != nil {
		t.Fatalf("SearchByRating() error = %v", err)
	}
	byTag := make(map[string]player.Player, len(players))
	for _, stored := range players {
		byTag[stored.Tag] = stored
	}
	if _, found := byTag["#OLD-AA"]; found {
		t.Fatalf("old successful-location row still exists: %#v", byTag["#OLD-AA"])
	}
	if !byTag["#FRESH-AA"].SyncedAt.Equal(refreshedAt) {
		t.Fatalf("fresh location timestamp = %s", byTag["#FRESH-AA"].SyncedAt)
	}
	if byTag["#OLD-BB"].CountryID != 2 || !byTag["#OLD-BB"].SyncedAt.Equal(time.Unix(100, 0).UTC()) {
		t.Fatalf("failed location was changed: %#v", byTag["#OLD-BB"])
	}
}

func TestTopPlayerSnapshotKeepsRankClanAndTimestamp(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "royale-scout.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	syncedAt := time.Unix(300, 0).UTC()
	players := []topplayer.Player{
		{Rank: 2, Tag: "#TWO", Name: "Two", RankedRating: 3200},
		{Rank: 1, Tag: "#ONE", Name: "One", RankedRating: 3250, ClanTag: "#CLAN", ClanName: "Clan", ClanBadgeID: 16000001},
	}
	if err := store.ReplaceTopSnapshot(ctx, players, syncedAt); err != nil {
		t.Fatalf("ReplaceTopSnapshot() error = %v", err)
	}
	loaded, err := store.ListTop(ctx)
	if err != nil {
		t.Fatalf("ListTop() error = %v", err)
	}
	if len(loaded) != 2 || loaded[0].Tag != "#ONE" || loaded[1].Tag != "#TWO" {
		t.Fatalf("top players = %#v", loaded)
	}
	if loaded[0].ClanBadgeID != 16000001 || !loaded[0].SyncedAt.Equal(syncedAt) {
		t.Fatalf("first top player = %#v", loaded[0])
	}
	if err := store.ClearTop(ctx); err != nil {
		t.Fatalf("ClearTop() error = %v", err)
	}
	loaded, err = store.ListTop(ctx)
	if err != nil || len(loaded) != 0 {
		t.Fatalf("ListTop() after clear = %#v, error %v", loaded, err)
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
