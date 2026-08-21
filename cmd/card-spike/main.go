package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"royalescout/internal/application/opponentscout"
	"royalescout/internal/infrastructure/sqlite"
	"royalescout/internal/platform/appdata"
	"royalescout/internal/platform/clashapi"
	"royalescout/internal/platform/config"
)

type observation struct {
	tag                string
	name               string
	evolutionLevel     int
	maxEvolutionLevel  int
	hasEvolutionMedium bool
	hasHeroMedium      bool
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Card spike failed:", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
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
	api, err := clashapi.New(localConfig.ClashRoyaleAPIToken)
	if err != nil {
		return err
	}

	search, err := opponentscout.NewSearchService(store).Search(ctx, opponentscout.SearchRequest{Input: "2500"})
	if err != nil {
		return err
	}
	limit := min(len(search.Candidates), 160)
	jobs := make(chan string)
	results := make(chan observation, limit)
	var workers sync.WaitGroup
	for range 4 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for tag := range jobs {
				battles, _, loadErr := api.BattleLog(ctx, tag)
				if loadErr != nil {
					continue
				}
				for _, battle := range battles {
					for _, participant := range append(battle.Team, battle.Opponent...) {
						for _, card := range participant.Cards {
							if card.Name != "Elite Barbarians" && card.Name != "Tombstone" {
								continue
							}
							results <- observation{
								tag:                participant.Tag,
								name:               card.Name,
								evolutionLevel:     card.EvolutionLevel,
								maxEvolutionLevel:  card.MaxEvolutionLevel,
								hasEvolutionMedium: card.IconURLs.EvolutionMedium != "",
								hasHeroMedium:      card.IconURLs.HeroMedium != "",
							}
						}
					}
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, candidate := range search.Candidates[:limit] {
			jobs <- candidate.PlayerTag
		}
	}()
	go func() {
		workers.Wait()
		close(results)
	}()

	unique := make(map[string]observation)
	for found := range results {
		key := fmt.Sprintf("%s/%d/%d/%t/%t", found.name, found.evolutionLevel, found.maxEvolutionLevel, found.hasEvolutionMedium, found.hasHeroMedium)
		unique[key] = found
	}
	for _, found := range unique {
		fmt.Printf("tag=%q name=%q evolutionLevel=%d maxEvolutionLevel=%d evolutionImage=%t heroImage=%t\n",
			found.tag,
			found.name,
			found.evolutionLevel,
			found.maxEvolutionLevel,
			found.hasEvolutionMedium,
			found.hasHeroMedium,
		)
	}
	return nil
}
