package leaderboardsync

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"royalescout/internal/domain/player"
	"royalescout/internal/platform/clashapi"
)

func TestSyncOncePublishesCompleteCountrySnapshot(t *testing.T) {
	repository := newMemoryStore()
	api := &fakeAPI{
		seasons: []clashapi.Season{{ID: "2026-07"}},
		locations: []clashapi.Location{
			{ID: 1, IsCountry: true, CountryCode: "AA"},
			{ID: 2, IsCountry: true, CountryCode: "BB"},
		},
		global: makeLeaderboardPlayers(requiredGlobalTop, "global"),
		countries: map[string][]clashapi.LeaderboardPlayer{
			"1": {{Tag: "#ONE", Name: "Суслик", EloRating: 2910}},
			"2": {{Tag: "#TWO", Name: "PRO张伟", EloRating: 2920}},
		},
	}
	service := New(api, repository, repository, 2, time.Minute)

	if err := service.SyncOnce(context.Background()); err != nil {
		t.Fatalf("SyncOnce() error = %v", err)
	}
	if len(repository.players) != 2 {
		t.Fatalf("players = %d", len(repository.players))
	}
	if repository.players["#ONE"].NicknameNormalized != "suslik" {
		t.Fatalf("normalized nickname = %q", repository.players["#ONE"].NicknameNormalized)
	}
}

func TestSyncOnceKeepsOldSnapshotWhenCountryFails(t *testing.T) {
	repository := newMemoryStore()
	repository.players["#OLD"] = player.Player{Tag: "#OLD"}
	repository.metadata[seasonMetadataKey] = "2026-07"
	api := &fakeAPI{
		seasons: []clashapi.Season{{ID: "2026-07"}},
		locations: []clashapi.Location{
			{ID: 1, IsCountry: true, CountryCode: "AA"},
			{ID: 2, IsCountry: true, CountryCode: "BB"},
		},
		global: makeLeaderboardPlayers(requiredGlobalTop, "global"),
		countries: map[string][]clashapi.LeaderboardPlayer{
			"1": {{Tag: "#ONE", Name: "One", EloRating: 2910}},
		},
		countryErrors: map[string]error{"2": errors.New("temporary failure")},
	}
	service := New(api, repository, repository, 2, time.Minute)

	if err := service.SyncOnce(context.Background()); err == nil {
		t.Fatal("SyncOnce() error = nil")
	}
	if len(repository.players) != 1 || repository.players["#OLD"].Tag != "#OLD" {
		t.Fatalf("old snapshot was replaced: %#v", repository.players)
	}
}

func TestSyncOnceClearsPlayersAndWaitsForNewGlobalTop(t *testing.T) {
	repository := newMemoryStore()
	repository.players["#OLD"] = player.Player{Tag: "#OLD"}
	repository.metadata[seasonMetadataKey] = "2026-06"
	api := &fakeAPI{
		seasons: []clashapi.Season{{ID: "2026-07"}},
		global:  makeLeaderboardPlayers(999, "global"),
	}
	service := New(api, repository, repository, 1, time.Minute)

	if err := service.SyncOnce(context.Background()); err != nil {
		t.Fatalf("SyncOnce() error = %v", err)
	}
	if len(repository.players) != 0 {
		t.Fatalf("players = %#v, want empty", repository.players)
	}
	status := service.Status()
	if !status.WaitingForSeason || status.Ready {
		t.Fatalf("status = %#v", status)
	}
}

type fakeAPI struct {
	seasons       []clashapi.Season
	locations     []clashapi.Location
	global        []clashapi.LeaderboardPlayer
	countries     map[string][]clashapi.LeaderboardPlayer
	countryErrors map[string]error
}

func (api *fakeAPI) Locations(context.Context, int, string) (clashapi.Page[clashapi.Location], clashapi.ResponseMeta, error) {
	return clashapi.Page[clashapi.Location]{Items: api.locations}, clashapi.ResponseMeta{StatusCode: 200}, nil
}

func (api *fakeAPI) Seasons(context.Context, int, string) (clashapi.Page[clashapi.Season], clashapi.ResponseMeta, error) {
	return clashapi.Page[clashapi.Season]{Items: api.seasons}, clashapi.ResponseMeta{StatusCode: 200}, nil
}

func (api *fakeAPI) PathOfLegendPlayers(_ context.Context, locationID string, _ int, _ string) (clashapi.Page[clashapi.LeaderboardPlayer], clashapi.ResponseMeta, error) {
	if locationID == "global" {
		return clashapi.Page[clashapi.LeaderboardPlayer]{Items: api.global}, clashapi.ResponseMeta{StatusCode: 200}, nil
	}
	if err := api.countryErrors[locationID]; err != nil {
		return clashapi.Page[clashapi.LeaderboardPlayer]{}, clashapi.ResponseMeta{StatusCode: 503}, err
	}
	return clashapi.Page[clashapi.LeaderboardPlayer]{Items: api.countries[locationID]}, clashapi.ResponseMeta{StatusCode: 200}, nil
}

func makeLeaderboardPlayers(count int, prefix string) []clashapi.LeaderboardPlayer {
	players := make([]clashapi.LeaderboardPlayer, count)
	for index := range players {
		players[index] = clashapi.LeaderboardPlayer{Tag: fmt.Sprintf("#%s%s", prefix, strconv.Itoa(index))}
	}
	return players
}

type memoryStore struct {
	players  map[string]player.Player
	metadata map[string]string
}

func newMemoryStore() *memoryStore {
	return &memoryStore{players: make(map[string]player.Player), metadata: make(map[string]string)}
}

func (store *memoryStore) SearchByRating(_ context.Context, minimum, maximum int) ([]player.Player, error) {
	var result []player.Player
	for _, stored := range store.players {
		if stored.RankedRating >= minimum && stored.RankedRating <= maximum {
			result = append(result, stored)
		}
	}
	return result, nil
}

func (store *memoryStore) ReplaceSnapshot(_ context.Context, players []player.Player, _ time.Time) error {
	store.players = make(map[string]player.Player, len(players))
	for _, stored := range players {
		store.players[stored.Tag] = stored
	}
	return nil
}

func (store *memoryStore) ClearActive(context.Context) error {
	store.players = make(map[string]player.Player)
	return nil
}

func (store *memoryStore) Count(context.Context) (int, error) {
	return len(store.players), nil
}

func (store *memoryStore) Get(_ context.Context, key string) (string, bool, error) {
	value, found := store.metadata[key]
	return value, found, nil
}

func (store *memoryStore) Set(_ context.Context, key, value string) error {
	store.metadata[key] = value
	return nil
}
