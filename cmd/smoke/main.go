package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"royalescout/internal/application/battlelog"
	"royalescout/internal/application/opponentscout"
	"royalescout/internal/infrastructure/cardcache"
	"royalescout/internal/infrastructure/sqlite"
	"royalescout/internal/platform/appdata"
	"royalescout/internal/platform/clashapi"
	"royalescout/internal/platform/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Smoke check failed:", err)
		os.Exit(1)
	}
}

func run() error {
	query := flag.String("query", "2500", "manual search query")
	playerTag := flag.String("tag", "", "optional player tag that bypasses candidate selection")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	localConfig, err := config.Load()
	if err != nil {
		return err
	}
	paths, err := appdata.Resolve()
	if err != nil {
		return err
	}
	store, err := sqlite.Open(paths.Database)
	if err != nil {
		return err
	}
	defer store.Close()

	candidateCount := 0
	selectedTag := strings.TrimSpace(*playerTag)
	if selectedTag == "" {
		players, searchErr := opponentscout.NewSearchService(store).Search(ctx, opponentscout.SearchRequest{Input: *query})
		if searchErr != nil {
			return searchErr
		}
		if len(players.Candidates) == 0 {
			return fmt.Errorf("query %q returned no candidates", *query)
		}
		candidateCount = len(players.Candidates)
		selectedTag = players.Candidates[0].PlayerTag
	}

	apiClient, err := clashapi.New(localConfig.ClashRoyaleAPIToken)
	if err != nil {
		return err
	}
	images, err := cardcache.New(paths.CardCache)
	if err != nil {
		return err
	}
	decks, err := battlelog.NewService(apiClient, images).LatestRankedDecks(ctx, selectedTag)
	if err != nil {
		return err
	}
	for _, match := range decks.Matches {
		if len(match.Cards) != 8 {
			return fmt.Errorf("deck has %d cards, want 8", len(match.Cards))
		}
		if match.AverageElixir <= 0 {
			return fmt.Errorf("deck has invalid average elixir %.1f", match.AverageElixir)
		}
		for _, card := range match.Cards {
			if !strings.HasPrefix(card.ImageURL, cardcache.AssetRoutePrefix) {
				return fmt.Errorf("card %d has no local cached asset URL", card.ID)
			}
		}
	}

	fmt.Printf("ok candidates=%d decks=%d cards=%d spells=%d evolutions=%d heroes=%d\n",
		candidateCount,
		len(decks.Matches),
		cardCount(decks.Matches),
		spellCount(decks.Matches),
		formCount(decks.Matches, "evolution"),
		formCount(decks.Matches, "hero"),
	)
	return nil
}

func spellCount(matches []battlelog.DeckMatch) int {
	total := 0
	for _, match := range matches {
		for _, card := range match.Cards {
			if card.IsSpell {
				total++
			}
		}
	}
	return total
}

func formCount(matches []battlelog.DeckMatch, form string) int {
	total := 0
	for _, match := range matches {
		for _, card := range match.Cards {
			if string(card.Form) == form {
				total++
			}
		}
	}
	return total
}

func cardCount(matches []battlelog.DeckMatch) int {
	total := 0
	for _, match := range matches {
		total += len(match.Cards)
	}
	return total
}
