package leaderboardsync

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"royalescout/internal/application/opponentscout"
	"royalescout/internal/application/ports"
	"royalescout/internal/domain/player"
	"royalescout/internal/platform/clashapi"
)

const (
	seasonMetadataKey    = "ranked_season_marker"
	requiredGlobalTop    = 1000
	leaderboardPageLimit = 1000
)

type API interface {
	Locations(context.Context, int, string) (clashapi.Page[clashapi.Location], clashapi.ResponseMeta, error)
	Seasons(context.Context, int, string) (clashapi.Page[clashapi.Season], clashapi.ResponseMeta, error)
	PathOfLegendPlayers(context.Context, string, int, string) (clashapi.Page[clashapi.LeaderboardPlayer], clashapi.ResponseMeta, error)
}

type Status struct {
	Ready            bool
	Syncing          bool
	WaitingForSeason bool
	PlayerCount      int
	LastError        string
	LastCycle        time.Duration
}

type Service struct {
	api         API
	players     ports.PlayerRepository
	metadata    ports.MetadataRepository
	normalizer  opponentscout.NicknameNormalizer
	concurrency int
	interval    time.Duration
	statusMutex sync.RWMutex
	status      Status
}

func New(
	api API,
	players ports.PlayerRepository,
	metadata ports.MetadataRepository,
	concurrency int,
	interval time.Duration,
) *Service {
	if concurrency < 1 {
		concurrency = 1
	}
	if interval <= 0 {
		interval = time.Minute
	}
	return &Service{
		api:         api,
		players:     players,
		metadata:    metadata,
		normalizer:  opponentscout.NewNicknameNormalizer(),
		concurrency: concurrency,
		interval:    interval,
	}
}

func (service *Service) Run(ctx context.Context) {
	for {
		cycleStartedAt := time.Now()
		err := service.SyncOnce(ctx)
		service.finishCycle(ctx, time.Since(cycleStartedAt), err)

		wait := time.Until(cycleStartedAt.Add(service.interval))
		if wait < 0 {
			wait = 0
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (service *Service) Status() Status {
	service.statusMutex.RLock()
	defer service.statusMutex.RUnlock()
	return service.status
}

func (service *Service) PrimeExistingSnapshot(playerCount int) {
	if playerCount <= 0 {
		return
	}
	service.updateStatus(func(status *Status) {
		status.Ready = true
		status.PlayerCount = playerCount
	})
}

func (service *Service) SyncOnce(ctx context.Context) error {
	service.updateStatus(func(status *Status) {
		status.Syncing = true
		status.LastError = ""
	})

	seasonPage, _, err := service.api.Seasons(ctx, 200, "")
	if err != nil {
		return fmt.Errorf("load seasons: %w", err)
	}
	seasonMarker, err := latestSeason(seasonPage.Items)
	if err != nil {
		return err
	}

	storedSeason, seasonFound, err := service.metadata.Get(ctx, seasonMetadataKey)
	if err != nil {
		return err
	}
	seasonChanged := seasonFound && storedSeason != seasonMarker
	if seasonChanged {
		if err := service.players.ClearActive(ctx); err != nil {
			return fmt.Errorf("clear players for new season: %w", err)
		}
		service.markWaitingForSeason()
	}
	if !seasonFound || seasonChanged {
		if err := service.metadata.Set(ctx, seasonMetadataKey, seasonMarker); err != nil {
			return err
		}
	}

	globalPage, _, err := service.api.PathOfLegendPlayers(ctx, "global", requiredGlobalTop, "")
	if err != nil {
		return fmt.Errorf("load global Path of Legend leaderboard: %w", err)
	}
	if len(globalPage.Items) < requiredGlobalTop {
		if err := service.players.ClearActive(ctx); err != nil {
			return fmt.Errorf("clear players while global leaderboard is incomplete: %w", err)
		}
		service.markWaitingForSeason()
		return nil
	}

	locationsPage, _, err := service.api.Locations(ctx, 1000, "")
	if err != nil {
		return fmt.Errorf("load locations: %w", err)
	}
	countries := make([]clashapi.Location, 0, len(locationsPage.Items))
	for _, location := range locationsPage.Items {
		if location.IsCountry {
			countries = append(countries, location)
		}
	}

	snapshot, err := service.collectSnapshot(ctx, countries)
	if err != nil {
		return err
	}
	syncedAt := time.Now().UTC()
	if err := service.players.ReplaceSnapshot(ctx, snapshot, syncedAt); err != nil {
		return fmt.Errorf("publish leaderboard snapshot: %w", err)
	}

	service.updateStatus(func(status *Status) {
		status.Ready = true
		status.WaitingForSeason = false
		status.PlayerCount = len(snapshot)
	})
	return nil
}

func (service *Service) collectSnapshot(ctx context.Context, countries []clashapi.Location) ([]player.Player, error) {
	type result struct {
		players []player.Player
		err     error
	}

	jobs := make(chan clashapi.Location)
	results := make(chan result)
	var workers sync.WaitGroup
	for range service.concurrency {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for country := range jobs {
				players, err := service.loadCountry(ctx, country)
				results <- result{players: players, err: err}
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

	byTag := make(map[string]player.Player)
	var firstError error
	for loaded := range results {
		if loaded.err != nil {
			if firstError == nil {
				firstError = loaded.err
			}
			continue
		}
		for _, storedPlayer := range loaded.players {
			byTag[storedPlayer.Tag] = storedPlayer
		}
	}
	if firstError != nil {
		return nil, firstError
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	snapshot := make([]player.Player, 0, len(byTag))
	for _, storedPlayer := range byTag {
		snapshot = append(snapshot, storedPlayer)
	}
	sort.Slice(snapshot, func(left, right int) bool { return snapshot[left].Tag < snapshot[right].Tag })
	return snapshot, nil
}

func (service *Service) loadCountry(ctx context.Context, country clashapi.Location) ([]player.Player, error) {
	locationID := strconv.FormatInt(country.ID, 10)
	after := ""
	seenCursors := make(map[string]struct{})
	players := make([]player.Player, 0, leaderboardPageLimit)

	for {
		page, _, err := service.api.PathOfLegendPlayers(ctx, locationID, leaderboardPageLimit, after)
		if err != nil {
			return nil, fmt.Errorf("load Path of Legend leaderboard for %s: %w", country.CountryCode, err)
		}
		for _, apiPlayer := range page.Items {
			players = append(players, player.Player{
				Tag:                apiPlayer.Tag,
				NicknameOriginal:   apiPlayer.Name,
				NicknameNormalized: service.normalizer.Normalize(apiPlayer.Name),
				RankedRating:       apiPlayer.EloRating,
				CountryID:          country.ID,
				CountryCode:        country.CountryCode,
			})
		}

		next := page.Paging.Cursors.After
		if next == "" {
			break
		}
		if _, duplicate := seenCursors[next]; duplicate {
			return nil, fmt.Errorf("pagination cursor loop for country %s", country.CountryCode)
		}
		seenCursors[next] = struct{}{}
		after = next
	}
	return players, nil
}

func latestSeason(seasons []clashapi.Season) (string, error) {
	latest := ""
	for _, season := range seasons {
		if season.ID > latest {
			latest = season.ID
		}
	}
	if latest == "" {
		return "", errors.New("season list is empty")
	}
	return latest, nil
}

func (service *Service) markWaitingForSeason() {
	service.updateStatus(func(status *Status) {
		status.Ready = false
		status.WaitingForSeason = true
		status.PlayerCount = 0
	})
}

func (service *Service) finishCycle(ctx context.Context, duration time.Duration, cycleError error) {
	count, countError := service.players.Count(ctx)
	service.updateStatus(func(status *Status) {
		status.Syncing = false
		status.LastCycle = duration
		if countError == nil {
			status.PlayerCount = count
			status.Ready = count > 0 && !status.WaitingForSeason
		}
		if cycleError != nil {
			status.LastError = cycleError.Error()
		}
	})
}

func (service *Service) updateStatus(update func(*Status)) {
	service.statusMutex.Lock()
	defer service.statusMutex.Unlock()
	update(&service.status)
}
