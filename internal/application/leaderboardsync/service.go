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
	"royalescout/internal/domain/topplayer"
	"royalescout/internal/platform/clashapi"
)

const (
	seasonMetadataKey    = "ranked_season_marker"
	requiredGlobalTop    = 1000
	visibleGlobalTop     = 250
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
	api          API
	players      ports.PlayerRepository
	topPlayers   ports.TopPlayerRepository
	metadata     ports.MetadataRepository
	normalizer   opponentscout.NicknameNormalizer
	concurrency  int
	interval     time.Duration
	topSyncMutex sync.Mutex
	statusMutex  sync.RWMutex
	status       Status
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
	service := &Service{
		api:         api,
		players:     players,
		metadata:    metadata,
		normalizer:  opponentscout.NewNicknameNormalizer(),
		concurrency: concurrency,
		interval:    interval,
	}
	if topPlayers, supported := players.(ports.TopPlayerRepository); supported {
		service.topPlayers = topPlayers
	}
	return service
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
		if service.topPlayers != nil {
			if err := service.topPlayers.ClearTop(ctx); err != nil {
				return fmt.Errorf("clear top players for new season: %w", err)
			}
		}
		service.markWaitingForSeason()
	}
	if !seasonFound || seasonChanged {
		if err := service.metadata.Set(ctx, seasonMetadataKey, seasonMarker); err != nil {
			return err
		}
	}

	existingPlayerCount, err := service.players.Count(ctx)
	if err != nil {
		return fmt.Errorf("count existing players: %w", err)
	}
	waitForNewSeasonLeaderboard := existingPlayerCount == 0

	globalItems, globalComplete, globalError := service.syncGlobalTop(ctx)
	if waitForNewSeasonLeaderboard && (globalError != nil || !globalComplete) {
		service.markWaitingForSeason()
		return globalError
	}
	cycleErrors := make([]error, 0)
	if globalError != nil {
		cycleErrors = append(cycleErrors, globalError)
	} else if !globalComplete {
		cycleErrors = append(cycleErrors, errors.New("global Path of Legend leaderboard is incomplete"))
	}

	locationsPage, _, err := service.api.Locations(ctx, 1000, "")
	if err != nil {
		cycleErrors = append(cycleErrors, fmt.Errorf("load locations: %w", err))
		return errors.Join(cycleErrors...)
	}
	countries := make([]clashapi.Location, 0, len(locationsPage.Items))
	for _, location := range locationsPage.Items {
		if location.IsCountry {
			countries = append(countries, location)
		}
	}
	if len(countries) == 0 {
		cycleErrors = append(cycleErrors, errors.New("country location list is empty"))
		return errors.Join(cycleErrors...)
	}

	snapshots, countryErrors, err := service.collectLocationSnapshots(ctx, countries)
	if err != nil {
		cycleErrors = append(cycleErrors, err)
		return errors.Join(cycleErrors...)
	}
	cycleErrors = append(cycleErrors, countryErrors...)
	if len(snapshots) == 0 {
		cycleErrors = append(cycleErrors, errors.New("no country leaderboard was refreshed"))
		return errors.Join(cycleErrors...)
	}
	if globalComplete {
		snapshots = append([]ports.LocationPlayerSnapshot{service.globalSearchSnapshot(globalItems)}, snapshots...)
	}
	syncedAt := time.Now().UTC()
	if err := service.players.ReplaceLocationSnapshots(ctx, snapshots, syncedAt); err != nil {
		return fmt.Errorf("publish leaderboard snapshot: %w", err)
	}
	playerCount, err := service.players.Count(ctx)
	if err != nil {
		return fmt.Errorf("count published players: %w", err)
	}

	service.updateStatus(func(status *Status) {
		status.Ready = true
		status.WaitingForSeason = false
		status.PlayerCount = playerCount
	})
	return errors.Join(cycleErrors...)
}

// RefreshTop fetches and publishes only the global top ranking. It intentionally
// skips the much heavier country-by-country player sync used by opponent search.
func (service *Service) RefreshTop(ctx context.Context) error {
	_, complete, err := service.syncGlobalTop(ctx)
	if err != nil {
		return err
	}
	if !complete {
		return fmt.Errorf("global Path of Legend leaderboard is incomplete")
	}
	return nil
}

func (service *Service) syncGlobalTop(ctx context.Context) ([]clashapi.LeaderboardPlayer, bool, error) {
	service.topSyncMutex.Lock()
	defer service.topSyncMutex.Unlock()

	globalPage, _, err := service.api.PathOfLegendPlayers(ctx, "global", requiredGlobalTop, "")
	if err != nil {
		return nil, false, fmt.Errorf("load global Path of Legend leaderboard: %w", err)
	}
	if len(globalPage.Items) < requiredGlobalTop {
		return nil, false, nil
	}
	if err := service.publishGlobalTop(ctx, globalPage.Items); err != nil {
		return nil, false, err
	}
	return globalPage.Items, true, nil
}

func (service *Service) publishGlobalTop(ctx context.Context, items []clashapi.LeaderboardPlayer) error {
	if service.topPlayers == nil {
		return nil
	}
	limit := min(visibleGlobalTop, len(items))
	players := make([]topplayer.Player, 0, limit)
	for _, apiPlayer := range items[:limit] {
		stored := topplayer.Player{
			Tag:          apiPlayer.Tag,
			Name:         apiPlayer.Name,
			Rank:         apiPlayer.Rank,
			RankedRating: apiPlayer.EloRating,
		}
		if stored.Rank == 0 {
			stored.Rank = len(players) + 1
		}
		if apiPlayer.Clan != nil {
			stored.ClanTag = apiPlayer.Clan.Tag
			stored.ClanName = apiPlayer.Clan.Name
			stored.ClanBadgeID = apiPlayer.Clan.BadgeID
		}
		players = append(players, stored)
	}
	syncedAt := time.Now().UTC()
	if err := service.topPlayers.ReplaceTopSnapshot(ctx, players, syncedAt); err != nil {
		return fmt.Errorf("publish global top player snapshot: %w", err)
	}
	return nil
}

func (service *Service) globalSearchSnapshot(items []clashapi.LeaderboardPlayer) ports.LocationPlayerSnapshot {
	players := make([]player.Player, 0, len(items))
	for _, apiPlayer := range items {
		players = append(players, player.Player{
			Tag:                apiPlayer.Tag,
			NicknameOriginal:   apiPlayer.Name,
			NicknameNormalized: service.normalizer.Normalize(apiPlayer.Name),
			RankedRating:       apiPlayer.EloRating,
			CountryID:          0,
		})
	}
	return ports.LocationPlayerSnapshot{LocationID: 0, Players: players}
}

func (service *Service) collectLocationSnapshots(ctx context.Context, countries []clashapi.Location) ([]ports.LocationPlayerSnapshot, []error, error) {
	type result struct {
		country clashapi.Location
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
				results <- result{country: country, players: players, err: err}
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

	snapshots := make([]ports.LocationPlayerSnapshot, 0, len(countries))
	failures := make([]error, 0)
	for loaded := range results {
		if loaded.err != nil {
			failures = append(failures, loaded.err)
			continue
		}
		snapshots = append(snapshots, ports.LocationPlayerSnapshot{
			LocationID: loaded.country.ID,
			Players:    loaded.players,
		})
	}
	if err := ctx.Err(); err != nil {
		return nil, failures, err
	}

	sort.Slice(snapshots, func(left, right int) bool { return snapshots[left].LocationID < snapshots[right].LocationID })
	return snapshots, failures, nil
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
