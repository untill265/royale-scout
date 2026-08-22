package battlelog

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"royalescout/internal/application/cardforms"
	"royalescout/internal/platform/clashapi"
)

const (
	rankedBattleType = "pathOfLegend"
	battleTimeLayout = "20060102T150405.000Z"
	maximumBattles   = 3
)

type API interface {
	BattleLog(context.Context, string) ([]clashapi.Battle, clashapi.ResponseMeta, error)
}

type CardImageRequest struct {
	Key string
	URL string
}

type CardImageCache interface {
	DataURL(context.Context, CardImageRequest) (string, error)
}

type LocalCardImageCache interface {
	AssetURL(context.Context, CardImageRequest) (string, error)
}

type Service struct {
	api    API
	images CardImageCache
}

type Result struct {
	PlayerTag string      `json:"playerTag"`
	Matches   []DeckMatch `json:"matches"`
}

type DeckMatch struct {
	BattleTime    string     `json:"battleTime"`
	Cards         []DeckCard `json:"cards"`
	AverageElixir float64    `json:"averageElixir"`
}

type DeckCard struct {
	ID       int64          `json:"id"`
	Name     string         `json:"name"`
	ImageURL string         `json:"imageUrl"`
	Form     cardforms.Form `json:"form"`
	IsSpell  bool           `json:"isSpell"`
}

func NewService(api API, images CardImageCache) Service {
	return Service{api: api, images: images}
}

func (service Service) LatestRankedDecks(ctx context.Context, playerTag string) (Result, error) {
	return service.LatestRankedDecksLimit(ctx, playerTag, maximumBattles)
}

func (service Service) LatestRankedDecksLimit(ctx context.Context, playerTag string, limit int) (Result, error) {
	if playerTag == "" {
		return Result{}, errors.New("player tag is empty")
	}
	if limit < 1 || limit > maximumBattles {
		return Result{}, fmt.Errorf("ranked deck limit must be between 1 and %d", maximumBattles)
	}
	battles, _, err := service.api.BattleLog(ctx, playerTag)
	if err != nil {
		return Result{}, err
	}

	ranked := make([]clashapi.Battle, 0, len(battles))
	for _, battle := range battles {
		if battle.Type == rankedBattleType && len(battle.Team) > 0 {
			ranked = append(ranked, battle)
		}
	}
	sort.SliceStable(ranked, func(left, right int) bool {
		return parsedBattleTime(ranked[left].BattleTime).After(parsedBattleTime(ranked[right].BattleTime))
	})
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}

	result := Result{PlayerTag: playerTag, Matches: make([]DeckMatch, len(ranked))}
	for index, battle := range ranked {
		match, err := service.toDeckMatch(ctx, battle)
		if err != nil {
			return Result{}, fmt.Errorf("prepare Ranked deck %d: %w", index+1, err)
		}
		result.Matches[index] = match
	}
	return result, nil
}

func (service Service) toDeckMatch(ctx context.Context, battle clashapi.Battle) (DeckMatch, error) {
	playedAt := parsedBattleTime(battle.BattleTime)
	match := DeckMatch{Cards: make([]DeckCard, len(battle.Team[0].Cards))}
	elixirTotal := 0
	for _, card := range battle.Team[0].Cards {
		elixirTotal += card.ElixirCost
	}
	if len(battle.Team[0].Cards) > 0 {
		match.AverageElixir = math.Round((float64(elixirTotal)/float64(len(battle.Team[0].Cards)))*10) / 10
	}
	if !playedAt.IsZero() {
		match.BattleTime = playedAt.Format(time.RFC3339)
	}

	type cardResult struct {
		index int
		card  DeckCard
		err   error
	}
	results := make(chan cardResult, len(battle.Team[0].Cards))
	var workers sync.WaitGroup
	for index, card := range battle.Team[0].Cards {
		workers.Add(1)
		go func() {
			defer workers.Done()
			asset, form := cardforms.ResolveBattleCard(card)
			imageURL, err := service.imageURL(ctx, CardImageRequest{Key: asset.Key, URL: asset.URL})
			if err != nil && form != cardforms.Normal && card.IconURLs.Medium != "" {
				imageURL, err = service.imageURL(ctx, CardImageRequest{
					Key: fmt.Sprintf("%d", card.ID),
					URL: card.IconURLs.Medium,
				})
			}
			results <- cardResult{
				index: index,
				card: DeckCard{
					ID:       card.ID,
					Name:     card.Name,
					ImageURL: imageURL,
					Form:     form,
					IsSpell:  isSpell(card.ID),
				},
				err: err,
			}
		}()
	}
	go func() {
		workers.Wait()
		close(results)
	}()

	for result := range results {
		if result.err != nil {
			return DeckMatch{}, result.err
		}
		match.Cards[result.index] = result.card
	}
	return match, nil
}

func (service Service) imageURL(ctx context.Context, request CardImageRequest) (string, error) {
	if localCache, supported := service.images.(LocalCardImageCache); supported {
		return localCache.AssetURL(ctx, request)
	}
	return service.images.DataURL(ctx, request)
}

func isSpell(cardID int64) bool {
	_, listed := prominentSpellIDs[cardID]
	return listed
}

// prominentSpellIDs contains the cards players primarily need to track as
// direct damage, control, or support spells. The Clash Royale API's 28xxxxxx
// family is broader: it also contains win conditions and troop-like cards such
// as Goblin Barrel, Graveyard, Heal Spirit, and Spirit Empress. Those should stay
// in the eight-card deck only instead of being repeated in the spell summary.
var prominentSpellIDs = map[int64]struct{}{
	28000000: {}, // Fireball
	28000001: {}, // Arrows
	28000002: {}, // Rage
	28000003: {}, // Rocket
	28000005: {}, // Freeze
	28000007: {}, // Lightning
	28000008: {}, // Zap
	28000009: {}, // Poison
	28000011: {}, // The Log
	28000012: {}, // Tornado
	28000013: {}, // Clone
	28000014: {}, // Earthquake
	28000015: {}, // Barbarian Barrel
	28000017: {}, // Giant Snowball
	28000018: {}, // Royal Delivery
	28000023: {}, // Void
	28000024: {}, // Goblin Curse
	28000026: {}, // Vines
}

func parsedBattleTime(value string) time.Time {
	parsed, err := time.Parse(battleTimeLayout, value)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}
