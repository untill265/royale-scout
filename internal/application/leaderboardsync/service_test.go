package leaderboardsync

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"royalescout/internal/domain/player"
	"royalescout/internal/domain/topplayer"
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
	if len(repository.topPlayers) != visibleGlobalTop || repository.topPlayers[1].Tag != "#global0" {
		t.Fatalf("top players = %d, first = %#v", len(repository.topPlayers), repository.topPlayers[1])
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

func TestRefreshTopPublishesGlobalRankingWithoutReplacingSearchSnapshot(t *testing.T) {
	repository := newMemoryStore()
	repository.players["#SEARCH"] = player.Player{Tag: "#SEARCH"}
	api := &fakeAPI{global: makeLeaderboardPlayers(requiredGlobalTop, "fresh")}
	service := New(api, repository, repository, 1, time.Minute)

	if err := service.RefreshTop(context.Background()); err != nil {
		t.Fatalf("RefreshTop() error = %v", err)
	}
	if len(repository.players) != 1 || repository.players["#SEARCH"].Tag != "#SEARCH" {
		t.Fatalf("search snapshot was changed: %#v", repository.players)
	}
	if len(repository.topPlayers) != visibleGlobalTop || repository.topPlayers[1].Tag != "#fresh0" {
		t.Fatalf("top players = %d, first = %#v", len(repository.topPlayers), repository.topPlayers[1])
	}
}

func TestRefreshTopKeepsExistingRankingWhenResponseIsIncomplete(t *testing.T) {
	repository := newMemoryStore()
	repository.topPlayers[1] = topplayer.Player{Tag: "#OLD", Rank: 1}
	api := &fakeAPI{global: makeLeaderboardPlayers(requiredGlobalTop-1, "partial")}
	service := New(api, repository, repository, 1, time.Minute)

	if err := service.RefreshTop(context.Background()); err == nil {
		t.Fatal("RefreshTop() error = nil")
	}
	if len(repository.topPlayers) != 1 || repository.topPlayers[1].Tag != "#OLD" {
		t.Fatalf("old top ranking was replaced: %#v", repository.topPlayers)
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
		players[index] = clashapi.LeaderboardPlayer{
			Tag:       fmt.Sprintf("#%s%s", prefix, strconv.Itoa(index)),
			Name:      fmt.Sprintf("Player %d", index+1),
			Rank:      index + 1,
			EloRating: 3300 - index,
		}
	}
	return players
}

type memoryStore struct {
	players    map[string]player.Player
	topPlayers map[int]topplayer.Player
	metadata   map[string]string
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		players:    make(map[string]player.Player),
		topPlayers: make(map[int]topplayer.Player),
		metadata:   make(map[string]string),
	}
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

func (store *memoryStore) ReplaceTopSnapshot(_ context.Context, players []topplayer.Player, syncedAt time.Time) error {
	store.topPlayers = make(map[int]topplayer.Player, len(players))
	for _, stored := range players {
		stored.SyncedAt = syncedAt
		store.topPlayers[stored.Rank] = stored
	}
	return nil
}

func (store *memoryStore) ListTop(context.Context) ([]topplayer.Player, error) {
	result := make([]topplayer.Player, 0, len(store.topPlayers))
	for rank := 1; rank <= len(store.topPlayers); rank++ {
		result = append(result, store.topPlayers[rank])
	}
	return result, nil
}

func (store *memoryStore) ClearTop(context.Context) error {
	store.topPlayers = make(map[int]topplayer.Player)
	return nil
}

func (store *memoryStore) Get(_ context.Context, key string) (string, bool, error) {
	value, found := store.metadata[key]
	return value, found, nil
}

func (store *memoryStore) Set(_ context.Context, key, value string) error {
	store.metadata[key] = value
	return nil
}
