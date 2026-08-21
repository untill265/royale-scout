package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"royalescout/internal/application/leaderboardsync"
	"royalescout/internal/application/opponentscout"
	"royalescout/internal/infrastructure/sqlite"
	"royalescout/internal/platform/clashapi"
	"royalescout/internal/platform/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Storage spike failed:", err)
		os.Exit(1)
	}
}

func run() error {
	databasePath := flag.String("db", "royale-scout-spike.db", "SQLite database path")
	searchIterations := flag.Int("searches", 1000, "number of search benchmark iterations")
	flag.Parse()

	localConfig, err := config.Load()
	if err != nil {
		return err
	}
	apiClient, err := clashapi.New(localConfig.ClashRoyaleAPIToken)
	if err != nil {
		return err
	}
	store, err := sqlite.Open(*databasePath)
	if err != nil {
		return err
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	syncService := leaderboardsync.New(apiClient, store, store, 4, time.Minute)
	startedAt := time.Now()
	if err := syncService.SyncOnce(ctx); err != nil {
		return err
	}
	syncDuration := time.Since(startedAt)
	count, err := store.Count(ctx)
	if err != nil {
		return err
	}

	searchService := opponentscout.NewSearchService(store)
	latencies := make([]time.Duration, 0, *searchIterations)
	totalCandidates := 0
	for index := range *searchIterations {
		rating := 1800 + (index % 1400)
		searchStartedAt := time.Now()
		result, err := searchService.Search(ctx, opponentscout.SearchRequest{Input: fmt.Sprintf("%d", rating)})
		if err != nil {
			return err
		}
		latencies = append(latencies, time.Since(searchStartedAt))
		totalCandidates += len(result.Candidates)
	}
	sort.Slice(latencies, func(left, right int) bool { return latencies[left] < latencies[right] })

	fmt.Printf("players=%d syncAndPublish=%s searches=%d p50=%s p95=%s averageCandidates=%.1f\n",
		count,
		syncDuration.Round(time.Millisecond),
		*searchIterations,
		percentile(latencies, 0.50),
		percentile(latencies, 0.95),
		float64(totalCandidates)/float64(*searchIterations),
	)
	return nil
}

func percentile(values []time.Duration, quantile float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	index := int(float64(len(values)-1) * quantile)
	return values[index].Round(time.Microsecond)
}
