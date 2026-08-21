package battlelog

import (
	"context"
	"errors"
	"testing"

	"royalescout/internal/platform/clashapi"
)

func TestLatestRankedDecksFiltersSortsAndDoesNotMergeDuplicates(t *testing.T) {
	identicalCards := makeCards(8)
	api := fakeAPI{battles: []clashapi.Battle{
		{Type: "challenge", BattleTime: "20260820T130000.000Z", Team: []clashapi.BattleParticipant{{Cards: identicalCards}}},
		{Type: rankedBattleType, BattleTime: "20260820T120000.000Z", Team: []clashapi.BattleParticipant{{Cards: identicalCards}}},
		{Type: rankedBattleType, BattleTime: "20260820T123000.000Z", Team: []clashapi.BattleParticipant{{Cards: identicalCards}}},
		{Type: rankedBattleType, BattleTime: "20260820T121500.000Z", Team: []clashapi.BattleParticipant{{Cards: identicalCards}}},
		{Type: rankedBattleType, BattleTime: "20260820T110000.000Z", Team: []clashapi.BattleParticipant{{Cards: identicalCards}}},
	}}
	service := NewService(api, fakeImageCache{})

	result, err := service.LatestRankedDecks(context.Background(), "#PLAYER")
	if err != nil {
		t.Fatalf("LatestRankedDecks() error = %v", err)
	}
	if len(result.Matches) != 3 {
		t.Fatalf("matches = %d, want 3", len(result.Matches))
	}
	if result.Matches[0].BattleTime != "2026-08-20T12:30:00Z" {
		t.Fatalf("first battleTime = %q", result.Matches[0].BattleTime)
	}
	for _, match := range result.Matches {
		if len(match.Cards) != 8 {
			t.Fatalf("cards = %d, want 8", len(match.Cards))
		}
	}
}

func TestEvolutionImageIsPreferredWhenUsed(t *testing.T) {
	api := fakeAPI{battles: []clashapi.Battle{{
		Type:       rankedBattleType,
		BattleTime: "20260820T120000.000Z",
		Team: []clashapi.BattleParticipant{{Cards: []clashapi.BattleCard{{
			ID:             42,
			Name:           "Evolved",
			EvolutionLevel: 1,
			IconURLs: clashapi.IconURLs{
				Medium:          "https://example.test/normal.png",
				EvolutionMedium: "https://example.test/evolution.png",
			},
		}}}},
	}}}
	cache := &recordingImageCache{}
	service := NewService(api, cache)

	if _, err := service.LatestRankedDecks(context.Background(), "#PLAYER"); err != nil {
		t.Fatalf("LatestRankedDecks() error = %v", err)
	}
	if cache.request.Key != "42-evolution" || cache.request.URL != "https://example.test/evolution.png" {
		t.Fatalf("image request = %#v", cache.request)
	}
}

func TestHeroImageIsPreferredOnlyWhenHeroFormIsUsed(t *testing.T) {
	api := fakeAPI{battles: []clashapi.Battle{{
		Type:       rankedBattleType,
		BattleTime: "20260820T120000.000Z",
		Team: []clashapi.BattleParticipant{{Cards: []clashapi.BattleCard{{
			ID:             27000009,
			Name:           "Tombstone",
			EvolutionLevel: 2,
			IconURLs: clashapi.IconURLs{
				Medium:     "https://example.test/normal.png",
				HeroMedium: "https://example.test/hero.png",
			},
		}}}},
	}}}
	cache := &recordingImageCache{}
	service := NewService(api, cache)

	result, err := service.LatestRankedDecks(context.Background(), "#PLAYER")
	if err != nil {
		t.Fatalf("LatestRankedDecks() error = %v", err)
	}
	if cache.request.Key != "27000009-hero" || cache.request.URL != "https://example.test/hero.png" {
		t.Fatalf("image request = %#v", cache.request)
	}
	if result.Matches[0].Cards[0].Form != "hero" {
		t.Fatalf("card form = %q, want hero", result.Matches[0].Cards[0].Form)
	}
}

func TestAvailableFormDoesNotReplaceNormalCardWhenNotSelected(t *testing.T) {
	api := fakeAPI{battles: []clashapi.Battle{{
		Type:       rankedBattleType,
		BattleTime: "20260820T120000.000Z",
		Team: []clashapi.BattleParticipant{{Cards: []clashapi.BattleCard{{
			ID:             42,
			Name:           "Normal",
			EvolutionLevel: 0,
			IconURLs: clashapi.IconURLs{
				Medium:          "https://example.test/normal.png",
				EvolutionMedium: "https://example.test/evolution.png",
				HeroMedium:      "https://example.test/hero.png",
			},
		}}}},
	}}}
	cache := &recordingImageCache{}
	service := NewService(api, cache)

	result, err := service.LatestRankedDecks(context.Background(), "#PLAYER")
	if err != nil {
		t.Fatalf("LatestRankedDecks() error = %v", err)
	}
	if cache.request.Key != "42" || cache.request.URL != "https://example.test/normal.png" {
		t.Fatalf("image request = %#v", cache.request)
	}
	if result.Matches[0].Cards[0].Form != "normal" {
		t.Fatalf("card form = %q, want normal", result.Matches[0].Cards[0].Form)
	}
}

func TestAllCurrentSpellFamilyCardsAreClassifiedForProminentDisplay(t *testing.T) {
	tests := []struct {
		id   int64
		name string
		want bool
	}{
		{28000000, "Fireball", true},
		{28000001, "Arrows", true},
		{28000002, "Rage", true},
		{28000003, "Rocket", true},
		{28000004, "Goblin Barrel", false},
		{28000005, "Freeze", true},
		{28000006, "Mirror", false},
		{28000007, "Lightning", true},
		{28000008, "Zap", true},
		{28000009, "Poison", true},
		{28000010, "Graveyard", false},
		{28000011, "The Log", true},
		{28000012, "Tornado", true},
		{28000013, "Clone", true},
		{28000014, "Earthquake", true},
		{28000015, "Barbarian Barrel", true},
		{28000016, "Heal Spirit", false},
		{28000017, "Giant Snowball", true},
		{28000018, "Royal Delivery", true},
		{28000023, "Void", true},
		{28000024, "Goblin Curse", true},
		{28000025, "Spirit Empress", false},
		{28000026, "Vines", true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isSpell(test.id); got != test.want {
				t.Fatalf("isSpell(%d) = %t, want %t", test.id, got, test.want)
			}
		})
	}
	if len(prominentSpellIDs) != 18 {
		t.Fatalf("prominent spell allowlist has %d cards, want 18", len(prominentSpellIDs))
	}
}

func TestNonSpellCardsNeverEnterProminentSpellDisplay(t *testing.T) {
	for id, name := range map[int64]string{
		26000032: "Miner",
		27000009: "Tombstone",
	} {
		t.Run(name, func(t *testing.T) {
			if isSpell(id) {
				t.Fatalf("isSpell(%d) = true, want false", id)
			}
		})
	}
}

func TestBattleLogMarksOnlyProminentSpellsForTheInterface(t *testing.T) {
	api := fakeAPI{battles: []clashapi.Battle{{
		Type:       rankedBattleType,
		BattleTime: "20260820T120000.000Z",
		Team: []clashapi.BattleParticipant{{Cards: []clashapi.BattleCard{
			{ID: 28000001, Name: "Arrows", IconURLs: clashapi.IconURLs{Medium: "https://example.test/arrows.png"}},
			{ID: 28000004, Name: "Goblin Barrel", IconURLs: clashapi.IconURLs{Medium: "https://example.test/barrel.png"}},
			{ID: 28000010, Name: "Graveyard", IconURLs: clashapi.IconURLs{Medium: "https://example.test/graveyard.png"}},
			{ID: 26000032, Name: "Miner", IconURLs: clashapi.IconURLs{Medium: "https://example.test/miner.png"}},
		}}},
	}}}

	result, err := NewService(api, fakeImageCache{}).LatestRankedDecks(context.Background(), "#PLAYER")
	if err != nil {
		t.Fatalf("LatestRankedDecks() error = %v", err)
	}
	want := map[string]bool{
		"Arrows":        true,
		"Goblin Barrel": false,
		"Graveyard":     false,
		"Miner":         false,
	}
	for _, card := range result.Matches[0].Cards {
		if card.IsSpell != want[card.Name] {
			t.Errorf("%s IsSpell = %t, want %t", card.Name, card.IsSpell, want[card.Name])
		}
	}
}

func TestSpecialFormKeepsBadgeAndFallsBackToNormalArtWhenFormAssetFails(t *testing.T) {
	api := fakeAPI{battles: []clashapi.Battle{{
		Type:       rankedBattleType,
		BattleTime: "20260820T120000.000Z",
		Team: []clashapi.BattleParticipant{{Cards: []clashapi.BattleCard{{
			ID:             42,
			Name:           "Evolved",
			EvolutionLevel: 1,
			IconURLs: clashapi.IconURLs{
				Medium:          "https://example.test/normal.png",
				EvolutionMedium: "https://example.test/evolution.png",
			},
		}}}},
	}}}
	cache := &failoverImageCache{}
	result, err := NewService(api, cache).LatestRankedDecks(context.Background(), "#PLAYER")
	if err != nil {
		t.Fatalf("LatestRankedDecks() error = %v", err)
	}
	if len(cache.requests) != 2 || cache.requests[1].Key != "42" {
		t.Fatalf("image requests = %#v", cache.requests)
	}
	card := result.Matches[0].Cards[0]
	if card.Form != "evolution" || card.ImageURL == "" {
		t.Fatalf("card = %#v", card)
	}
}

type fakeAPI struct {
	battles []clashapi.Battle
}

func (api fakeAPI) BattleLog(context.Context, string) ([]clashapi.Battle, clashapi.ResponseMeta, error) {
	return api.battles, clashapi.ResponseMeta{StatusCode: 200}, nil
}

type fakeImageCache struct{}

func (fakeImageCache) DataURL(_ context.Context, request CardImageRequest) (string, error) {
	return "data:image/png;base64," + request.Key, nil
}

type recordingImageCache struct {
	request CardImageRequest
}

type failoverImageCache struct {
	requests []CardImageRequest
}

func (cache *failoverImageCache) DataURL(_ context.Context, request CardImageRequest) (string, error) {
	cache.requests = append(cache.requests, request)
	if len(cache.requests) == 1 {
		return "", errors.New("form image unavailable")
	}
	return "data:image/png;base64,normal", nil
}

func (cache *recordingImageCache) DataURL(_ context.Context, request CardImageRequest) (string, error) {
	cache.request = request
	return "data:image/png;base64,test", nil
}

func makeCards(count int) []clashapi.BattleCard {
	cards := make([]clashapi.BattleCard, count)
	for index := range cards {
		cards[index] = clashapi.BattleCard{
			ID:   int64(index + 1),
			Name: "Card",
			IconURLs: clashapi.IconURLs{
				Medium: "https://example.test/card.png",
			},
		}
	}
	return cards
}
