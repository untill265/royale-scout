package topranking

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"royalescout/internal/application/battlelog"
	"royalescout/internal/application/ports"
)

const (
	maximumBatchSize = 8
	deckCacheTTL     = 2 * time.Minute
	deckConcurrency  = 2
)

type DeckProvider interface {
	LatestRankedDecksLimit(context.Context, string, int) (battlelog.Result, error)
}

type Player struct {
	PlayerTag    string `json:"playerTag"`
	Nickname     string `json:"nickname"`
	Rank         int    `json:"rank"`
	RankedRating int    `json:"rankedRating"`
	ClanTag      string `json:"clanTag"`
	ClanName     string `json:"clanName"`
	ClanBadgeID  int64  `json:"clanBadgeId"`
}

type Snapshot struct {
	Players   []Player `json:"players"`
	UpdatedAt string   `json:"updatedAt"`
}

type PlayerDecks struct {
	PlayerTag string                `json:"playerTag"`
	Matches   []battlelog.DeckMatch `json:"matches"`
	ErrorCode string                `json:"errorCode"`
}

type DeckBatch struct {
	Players []PlayerDecks `json:"players"`
}

type cacheEntry struct {
	result    battlelog.Result
	fetchedAt time.Time
}

type Service struct {
	repository ports.TopPlayerRepository
	decks      DeckProvider
	cacheMutex sync.RWMutex
	cache      map[string]cacheEntry
}

func New(repository ports.TopPlayerRepository, decks DeckProvider) *Service {
	return &Service{
		repository: repository,
		decks:      decks,
		cache:      make(map[string]cacheEntry),
	}
}

func (service *Service) Snapshot(ctx context.Context) (Snapshot, error) {
	storedPlayers, err := service.repository.ListTop(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	result := Snapshot{Players: make([]Player, len(storedPlayers))}
	for index, stored := range storedPlayers {
		result.Players[index] = Player{
			PlayerTag:    stored.Tag,
			Nickname:     stored.Name,
			Rank:         stored.Rank,
			RankedRating: stored.RankedRating,
			ClanTag:      stored.ClanTag,
			ClanName:     stored.ClanName,
			ClanBadgeID:  stored.ClanBadgeID,
		}
		if index == 0 && !stored.SyncedAt.IsZero() {
			result.UpdatedAt = stored.SyncedAt.Format(time.RFC3339)
		}
	}
	return result, nil
}

func (service *Service) Decks(ctx context.Context, playerTags []string) (DeckBatch, error) {
	tags, err := service.validatedTags(ctx, playerTags)
	if err != nil {
		return DeckBatch{}, err
	}
	result := DeckBatch{Players: make([]PlayerDecks, len(tags))}

	jobs := make(chan int)
	var workers sync.WaitGroup
	for range min(deckConcurrency, len(tags)) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range jobs {
				result.Players[index] = service.loadPlayerDecks(ctx, tags[index])
			}
		}()
	}
	for index := range tags {
		select {
		case jobs <- index:
		case <-ctx.Done():
			close(jobs)
			workers.Wait()
			return DeckBatch{}, ctx.Err()
		}
	}
	close(jobs)
	workers.Wait()
	if err := ctx.Err(); err != nil {
		return DeckBatch{}, err
	}
	return result, nil
}

func (service *Service) validatedTags(ctx context.Context, playerTags []string) ([]string, error) {
	if len(playerTags) == 0 || len(playerTags) > maximumBatchSize {
		return nil, fmt.Errorf("top deck batch must contain between 1 and %d players", maximumBatchSize)
	}
	storedPlayers, err := service.repository.ListTop(ctx)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]struct{}, len(storedPlayers))
	for _, stored := range storedPlayers {
		allowed[stored.Tag] = struct{}{}
	}

	seen := make(map[string]struct{}, len(playerTags))
	tags := make([]string, 0, len(playerTags))
	for _, playerTag := range playerTags {
		trimmed := strings.TrimSpace(playerTag)
		if trimmed == "" {
			return nil, errors.New("top player tag is empty")
		}
		if _, found := allowed[trimmed]; !found {
			return nil, fmt.Errorf("player %q is not in the current top ranking", trimmed)
		}
		if _, duplicate := seen[trimmed]; duplicate {
			continue
		}
		seen[trimmed] = struct{}{}
		tags = append(tags, trimmed)
	}
	if len(tags) == 0 {
		return nil, errors.New("top deck batch is empty after validation")
	}
	return tags, nil
}

func (service *Service) loadPlayerDecks(ctx context.Context, playerTag string) PlayerDecks {
	if cached, found := service.cached(playerTag); found {
		return PlayerDecks{PlayerTag: playerTag, Matches: cached.Matches}
	}
	loaded, err := service.decks.LatestRankedDecksLimit(ctx, playerTag, 1)
	if err != nil {
		return PlayerDecks{PlayerTag: playerTag, ErrorCode: "unavailable"}
	}
	service.cacheMutex.Lock()
	service.cache[playerTag] = cacheEntry{result: loaded, fetchedAt: time.Now().UTC()}
	service.cacheMutex.Unlock()
	return PlayerDecks{PlayerTag: playerTag, Matches: loaded.Matches}
}

func (service *Service) cached(playerTag string) (battlelog.Result, bool) {
	service.cacheMutex.RLock()
	cached, found := service.cache[playerTag]
	service.cacheMutex.RUnlock()
	if !found || time.Since(cached.fetchedAt) > deckCacheTTL {
		if found {
			service.cacheMutex.Lock()
			delete(service.cache, playerTag)
			service.cacheMutex.Unlock()
		}
		return battlelog.Result{}, false
	}
	return cached.result, true
}
