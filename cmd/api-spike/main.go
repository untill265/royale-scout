package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"royalescout/internal/platform/clashapi"
	"royalescout/internal/platform/config"
)

const rankedBattleType = "pathOfLegend"

type report struct {
	GeneratedAt time.Time        `json:"generatedAt"`
	Concurrency int              `json:"concurrency"`
	Locations   locationsReport  `json:"locations"`
	Global      globalReport     `json:"globalLeaderboard"`
	Pagination  paginationReport `json:"pagination"`
	Refresh     refreshReport    `json:"countryRefresh"`
	BattleLog   battleLogReport  `json:"battleLog"`
	Cards       cardsReport      `json:"cards"`
	Images      imagesReport     `json:"imagePreload"`
}

type locationsReport struct {
	Total     int `json:"total"`
	Countries int `json:"countries"`
}

type globalReport struct {
	Players int  `json:"players"`
	HasNext bool `json:"hasNext"`
}

type paginationReport struct {
	FirstPagePlayers  int `json:"firstPagePlayers"`
	SecondPagePlayers int `json:"secondPagePlayers"`
	Overlap           int `json:"overlap"`
	FirstRank         int `json:"firstRank"`
	LastRank          int `json:"lastRank"`
}

type refreshReport struct {
	DurationMilliseconds int64          `json:"durationMilliseconds"`
	Requests             int            `json:"requests"`
	SuccessfulRequests   int            `json:"successfulRequests"`
	FailedRequests       int            `json:"failedRequests"`
	RequestsPerSecond    float64        `json:"requestsPerSecond"`
	StatusCounts         map[string]int `json:"statusCounts"`
	TotalPlayerEntries   int            `json:"totalPlayerEntries"`
	UniquePlayers        int            `json:"uniquePlayers"`
	DuplicateEntries     int            `json:"duplicateEntries"`
	EmptyCountries       int            `json:"emptyCountries"`
	MinPlayersPerCountry int            `json:"minPlayersPerCountry"`
	MaxPlayersPerCountry int            `json:"maxPlayersPerCountry"`
	CountriesWithNext    int            `json:"countriesWithNextCursor"`
	LatencyP50Millis     int64          `json:"latencyP50Milliseconds"`
	LatencyP95Millis     int64          `json:"latencyP95Milliseconds"`
	ErrorSamples         []string       `json:"errorSamples,omitempty"`
}

type battleLogReport struct {
	Entries                 int            `json:"entries"`
	TypeCounts              map[string]int `json:"typeCounts"`
	RankedEntries           int            `json:"rankedEntries"`
	LatestRankedCards       int            `json:"latestRankedCards"`
	BattleTimePresent       bool           `json:"battleTimePresent"`
	BattleTimeParsesAsUTC   bool           `json:"battleTimeParsesAsUtc"`
	OfficialCardImagesFound int            `json:"officialCardImagesFound"`
}

type cardsReport struct {
	Total              int `json:"total"`
	WithMediumImage    int `json:"withMediumImage"`
	WithEvolutionImage int `json:"withEvolutionImage"`
}

type imagesReport struct {
	Requested            int   `json:"requested"`
	Successful           int   `json:"successful"`
	Bytes                int64 `json:"bytes"`
	DurationMilliseconds int64 `json:"durationMilliseconds"`
	Errors               int   `json:"errors"`
}

type countryResult struct {
	location clashapi.Location
	page     clashapi.Page[clashapi.LeaderboardPlayer]
	meta     clashapi.ResponseMeta
	err      error
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "API spike failed:", err)
		os.Exit(1)
	}
}

func run() error {
	concurrency := flag.Int("concurrency", 1, "number of concurrent country leaderboard requests")
	timeout := flag.Duration("timeout", 10*time.Minute, "overall spike timeout")
	outputPath := flag.String("out", "", "optional JSON report path")
	flag.Parse()
	if *concurrency < 1 || *concurrency > 32 {
		return errors.New("concurrency must be between 1 and 32")
	}

	localConfig, err := config.Load()
	if err != nil {
		return err
	}
	client, err := clashapi.New(localConfig.ClashRoyaleAPIToken)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	result := report{GeneratedAt: time.Now().UTC(), Concurrency: *concurrency}

	locationsPage, _, err := client.Locations(ctx, 1000, "")
	if err != nil {
		return fmt.Errorf("load locations: %w", err)
	}
	countries := make([]clashapi.Location, 0, len(locationsPage.Items))
	for _, location := range locationsPage.Items {
		if location.IsCountry {
			countries = append(countries, location)
		}
	}
	result.Locations = locationsReport{Total: len(locationsPage.Items), Countries: len(countries)}

	globalPage, _, err := client.PathOfLegendPlayers(ctx, "global", 1000, "")
	if err != nil {
		return fmt.Errorf("load global Path of Legend leaderboard: %w", err)
	}
	result.Global = globalReport{Players: len(globalPage.Items), HasNext: globalPage.Paging.Cursors.After != ""}
	if len(globalPage.Items) == 0 {
		return errors.New("global Path of Legend leaderboard is empty")
	}

	firstPage, _, err := client.PathOfLegendPlayers(ctx, "global", 50, "")
	if err != nil {
		return fmt.Errorf("load first pagination sample: %w", err)
	}
	secondPage, _, err := client.PathOfLegendPlayers(ctx, "global", 50, firstPage.Paging.Cursors.After)
	if err != nil {
		return fmt.Errorf("load second pagination sample: %w", err)
	}
	result.Pagination = paginationStats(firstPage.Items, secondPage.Items)

	result.Refresh = scanCountries(ctx, client, countries, *concurrency)

	battles, _, err := client.BattleLog(ctx, globalPage.Items[0].Tag)
	if err != nil {
		return fmt.Errorf("load battle log sample: %w", err)
	}
	result.BattleLog, result.Images = inspectBattleLogAndImages(ctx, battles)

	cardsPage, _, err := client.Cards(ctx, 1000, "")
	if err != nil {
		return fmt.Errorf("load card catalog: %w", err)
	}
	result.Cards = inspectCards(cardsPage.Items)

	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("encode spike report: %w", err)
	}
	if *outputPath != "" {
		if err := writeReport(*outputPath, encoded); err != nil {
			return err
		}
	}

	fmt.Printf("locations=%d countries=%d global=%d\n", result.Locations.Total, result.Locations.Countries, result.Global.Players)
	fmt.Printf("refresh requests=%d success=%d failed=%d duration=%s rps=%.2f uniquePlayers=%d\n",
		result.Refresh.Requests,
		result.Refresh.SuccessfulRequests,
		result.Refresh.FailedRequests,
		time.Duration(result.Refresh.DurationMilliseconds)*time.Millisecond,
		result.Refresh.RequestsPerSecond,
		result.Refresh.UniquePlayers,
	)
	fmt.Printf("battleLog=%d ranked=%d cards=%d officialImages=%d preload=%d/%d\n",
		result.BattleLog.Entries,
		result.BattleLog.RankedEntries,
		result.BattleLog.LatestRankedCards,
		result.BattleLog.OfficialCardImagesFound,
		result.Images.Successful,
		result.Images.Requested,
	)
	if *outputPath != "" {
		fmt.Println("report=" + *outputPath)
	}

	return nil
}

func paginationStats(first, second []clashapi.LeaderboardPlayer) paginationReport {
	firstTags := make(map[string]struct{}, len(first))
	for _, player := range first {
		firstTags[player.Tag] = struct{}{}
	}
	overlap := 0
	for _, player := range second {
		if _, exists := firstTags[player.Tag]; exists {
			overlap++
		}
	}

	stats := paginationReport{
		FirstPagePlayers:  len(first),
		SecondPagePlayers: len(second),
		Overlap:           overlap,
	}
	if len(first) > 0 {
		stats.FirstRank = first[0].Rank
	}
	if len(second) > 0 {
		stats.LastRank = second[len(second)-1].Rank
	}
	return stats
}

func scanCountries(ctx context.Context, client *clashapi.Client, countries []clashapi.Location, concurrency int) refreshReport {
	jobs := make(chan clashapi.Location)
	results := make(chan countryResult)
	var workers sync.WaitGroup

	for range concurrency {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for location := range jobs {
				page, meta, err := client.PathOfLegendPlayers(ctx, strconv.FormatInt(location.ID, 10), 1000, "")
				results <- countryResult{location: location, page: page, meta: meta, err: err}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, country := range countries {
			select {
			case jobs <- country:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		workers.Wait()
		close(results)
	}()

	startedAt := time.Now()
	stats := refreshReport{
		Requests:             len(countries),
		StatusCounts:         make(map[string]int),
		MinPlayersPerCountry: -1,
	}
	uniquePlayers := make(map[string]struct{})
	latencies := make([]time.Duration, 0, len(countries))

	for result := range results {
		status := strconv.Itoa(result.meta.StatusCode)
		stats.StatusCounts[status]++
		latencies = append(latencies, result.meta.Duration)
		if result.err != nil {
			stats.FailedRequests++
			if len(stats.ErrorSamples) < 10 {
				stats.ErrorSamples = append(stats.ErrorSamples, result.location.CountryCode+": "+result.err.Error())
			}
			continue
		}

		stats.SuccessfulRequests++
		playerCount := len(result.page.Items)
		stats.TotalPlayerEntries += playerCount
		if playerCount == 0 {
			stats.EmptyCountries++
		}
		if stats.MinPlayersPerCountry == -1 || playerCount < stats.MinPlayersPerCountry {
			stats.MinPlayersPerCountry = playerCount
		}
		if playerCount > stats.MaxPlayersPerCountry {
			stats.MaxPlayersPerCountry = playerCount
		}
		if result.page.Paging.Cursors.After != "" {
			stats.CountriesWithNext++
		}
		for _, player := range result.page.Items {
			uniquePlayers[player.Tag] = struct{}{}
		}
	}

	duration := time.Since(startedAt)
	stats.DurationMilliseconds = duration.Milliseconds()
	if duration > 0 {
		stats.RequestsPerSecond = float64(stats.SuccessfulRequests+stats.FailedRequests) / duration.Seconds()
	}
	stats.UniquePlayers = len(uniquePlayers)
	stats.DuplicateEntries = stats.TotalPlayerEntries - stats.UniquePlayers
	stats.LatencyP50Millis = percentile(latencies, 0.50).Milliseconds()
	stats.LatencyP95Millis = percentile(latencies, 0.95).Milliseconds()
	return stats
}

func inspectBattleLogAndImages(ctx context.Context, battles []clashapi.Battle) (battleLogReport, imagesReport) {
	stats := battleLogReport{Entries: len(battles), TypeCounts: make(map[string]int)}
	var latestRanked *clashapi.Battle
	for index := range battles {
		battle := &battles[index]
		stats.TypeCounts[battle.Type]++
		if battle.Type != rankedBattleType {
			continue
		}
		stats.RankedEntries++
		if latestRanked == nil || battle.BattleTime > latestRanked.BattleTime {
			latestRanked = battle
		}
	}
	if latestRanked == nil {
		return stats, imagesReport{}
	}

	stats.BattleTimePresent = latestRanked.BattleTime != ""
	_, parseErr := time.Parse("20060102T150405.000Z", latestRanked.BattleTime)
	stats.BattleTimeParsesAsUTC = parseErr == nil
	if len(latestRanked.Team) == 0 {
		return stats, imagesReport{}
	}
	stats.LatestRankedCards = len(latestRanked.Team[0].Cards)
	imageURLs := make([]string, 0, len(latestRanked.Team[0].Cards))
	for _, card := range latestRanked.Team[0].Cards {
		if card.IconURLs.Medium != "" {
			stats.OfficialCardImagesFound++
			imageURLs = append(imageURLs, card.IconURLs.Medium)
		}
	}
	return stats, preloadImages(ctx, imageURLs)
}

func inspectCards(cards []clashapi.Card) cardsReport {
	stats := cardsReport{Total: len(cards)}
	for _, card := range cards {
		if card.IconURLs.Medium != "" {
			stats.WithMediumImage++
		}
		if card.IconURLs.EvolutionMedium != "" {
			stats.WithEvolutionImage++
		}
	}
	return stats
}

func preloadImages(ctx context.Context, imageURLs []string) imagesReport {
	stats := imagesReport{Requested: len(imageURLs)}
	if len(imageURLs) == 0 {
		return stats
	}

	type imageResult struct {
		bytes int64
		err   error
	}
	results := make(chan imageResult, len(imageURLs))
	startedAt := time.Now()
	for _, imageURL := range imageURLs {
		go func() {
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
			if err != nil {
				results <- imageResult{err: err}
				return
			}
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				results <- imageResult{err: err}
				return
			}
			defer response.Body.Close()
			if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
				results <- imageResult{err: fmt.Errorf("HTTP %d", response.StatusCode)}
				return
			}
			bytes, err := io.Copy(io.Discard, response.Body)
			results <- imageResult{bytes: bytes, err: err}
		}()
	}

	for range imageURLs {
		result := <-results
		if result.err != nil {
			stats.Errors++
			continue
		}
		stats.Successful++
		stats.Bytes += result.bytes
	}
	stats.DurationMilliseconds = time.Since(startedAt).Milliseconds()
	return stats
}

func percentile(values []time.Duration, quantile float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), values...)
	sort.Slice(sorted, func(left, right int) bool { return sorted[left] < sorted[right] })
	index := int(float64(len(sorted)-1) * quantile)
	return sorted[index]
}

func writeReport(path string, contents []byte) error {
	directory := filepath.Dir(path)
	if directory != "." {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return fmt.Errorf("create report directory: %w", err)
		}
	}
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		return fmt.Errorf("write spike report: %w", err)
	}
	return nil
}
