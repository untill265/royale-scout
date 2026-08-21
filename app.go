package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"royalescout/internal/application/battlelog"
	"royalescout/internal/application/cardassets"
	"royalescout/internal/application/leaderboardsync"
	"royalescout/internal/application/opponentscout"
	"royalescout/internal/infrastructure/cardcache"
	"royalescout/internal/infrastructure/sqlite"
	"royalescout/internal/platform/appdata"
	"royalescout/internal/platform/clashapi"
	"royalescout/internal/platform/config"
	"royalescout/internal/platform/securetoken"
)

const (
	applicationVersion       = "0.2.1"
	leaderboardSyncInterval  = 5 * time.Minute
	tokenVerificationTimeout = 30 * time.Second
	ignoreLegacyConfigEnv    = "ROYALE_SCOUT_IGNORE_LEGACY_CONFIG"
)

var ErrDatabaseNotReady = errors.New("player database is not ready")

type AppStatus struct {
	State     string `json:"state"`
	HasToken  bool   `json:"hasToken"`
	ErrorCode string `json:"errorCode"`
}

type TokenSetupResult struct {
	Success bool   `json:"success"`
	Code    string `json:"code"`
}

type ActionResult struct {
	Success bool   `json:"success"`
	Code    string `json:"code"`
}

type AppInfo struct {
	Version       string `json:"version"`
	DataDirectory string `json:"dataDirectory"`
	TokenStorage  string `json:"tokenStorage"`
}

type App struct {
	ctx    context.Context
	cancel context.CancelFunc

	operationMutex sync.RWMutex
	stateMutex     sync.RWMutex

	paths      appdata.Paths
	store      *sqlite.Store
	imageCache *cardcache.Cache
	tokenStore *securetoken.Store

	servicesCancel context.CancelFunc
	services       sync.WaitGroup
	syncService    *leaderboardsync.Service
	searchService  opponentscout.SearchService
	battleService  battlelog.Service

	hasToken         bool
	startupError     error
	startupErrorCode string
}

func NewApp() *App {
	return &App{}
}

func (app *App) startup(wailsContext context.Context) {
	app.ctx, app.cancel = context.WithCancel(wailsContext)
	app.operationMutex.Lock()
	defer app.operationMutex.Unlock()
	if err := app.initialiseLocked(); err != nil {
		app.setStartupError("localStorage", err)
	}
}

func (app *App) initialiseLocked() error {
	if err := app.openStorageLocked(); err != nil {
		return err
	}

	token, err := app.tokenStore.Load()
	if errors.Is(err, securetoken.ErrNotFound) {
		if os.Getenv(ignoreLegacyConfigEnv) == "1" {
			app.setTokenState(false)
			return nil
		}
		legacy, legacyErr := config.Load()
		if errors.Is(legacyErr, config.ErrTokenMissing) {
			app.setTokenState(false)
			return nil
		}
		if legacyErr != nil {
			return fmt.Errorf("load legacy local API token: %w", legacyErr)
		}
		token = legacy.ClashRoyaleAPIToken
		if err := app.tokenStore.Save(token); err != nil {
			return fmt.Errorf("migrate API token to protected Windows storage: %w", err)
		}
	} else if err != nil {
		return err
	}

	return app.startServicesLocked(token)
}

func (app *App) openStorageLocked() error {
	paths, err := appdata.Resolve()
	if err != nil {
		return err
	}
	store, err := sqlite.Open(paths.Database)
	if err != nil {
		return err
	}
	imageCache, err := cardcache.New(paths.CardCache)
	if err != nil {
		_ = store.Close()
		return err
	}

	app.paths = paths
	app.store = store
	app.imageCache = imageCache
	app.tokenStore = securetoken.New(paths.Credentials)
	return nil
}

func (app *App) startServicesLocked(token string) error {
	apiClient, err := clashapi.New(token)
	if err != nil {
		return err
	}
	serviceContext, cancel := context.WithCancel(app.ctx)
	syncService := leaderboardsync.New(apiClient, app.store, app.store, 4, leaderboardSyncInterval)
	if playerCount, countErr := app.store.Count(serviceContext); countErr == nil {
		syncService.PrimeExistingSnapshot(playerCount)
	}

	app.servicesCancel = cancel
	app.stateMutex.Lock()
	app.syncService = syncService
	app.searchService = opponentscout.NewSearchService(app.store)
	app.battleService = battlelog.NewService(apiClient, app.imageCache)
	app.hasToken = true
	app.startupError = nil
	app.startupErrorCode = ""
	app.stateMutex.Unlock()

	app.services.Add(1)
	go func() {
		defer app.services.Done()
		syncService.Run(serviceContext)
	}()

	preloader := cardassets.NewPreloader(apiClient, app.imageCache, 8)
	app.services.Add(1)
	go func() {
		defer app.services.Done()
		_ = preloader.Run(serviceContext)
	}()
	return nil
}

func (app *App) stopServicesLocked() {
	if app.servicesCancel != nil {
		app.servicesCancel()
		app.servicesCancel = nil
	}
	app.services.Wait()
	app.stateMutex.Lock()
	app.syncService = nil
	app.searchService = opponentscout.SearchService{}
	app.battleService = battlelog.Service{}
	app.stateMutex.Unlock()
}

func (app *App) shutdown(_ context.Context) {
	if app.cancel != nil {
		app.cancel()
	}
	app.operationMutex.Lock()
	defer app.operationMutex.Unlock()
	app.stopServicesLocked()
	if app.store != nil {
		_ = app.store.Close()
		app.store = nil
	}
}

func (app *App) GetStatus() AppStatus {
	app.stateMutex.RLock()
	startupError := app.startupError
	errorCode := app.startupErrorCode
	hasToken := app.hasToken
	syncService := app.syncService
	app.stateMutex.RUnlock()

	if startupError != nil {
		return AppStatus{State: "error", HasToken: hasToken, ErrorCode: errorCode}
	}
	if !hasToken {
		return AppStatus{State: "setupRequired", HasToken: false}
	}
	if syncService == nil {
		return AppStatus{State: "initializing", HasToken: true}
	}
	status := syncService.Status()
	if status.Ready {
		return AppStatus{State: "ready", HasToken: true}
	}
	if status.WaitingForSeason {
		return AppStatus{State: "waitingForSeason", HasToken: true}
	}
	if status.LastError != "" && !status.Syncing {
		return AppStatus{State: "error", HasToken: true, ErrorCode: "apiConnection"}
	}
	return AppStatus{State: "initializing", HasToken: true}
}

func (app *App) GetAppInfo() AppInfo {
	app.operationMutex.RLock()
	defer app.operationMutex.RUnlock()
	return AppInfo{
		Version:       applicationVersion,
		DataDirectory: app.paths.Root,
		TokenStorage:  "windows-dpapi",
	}
}

func (app *App) ConfigureAPIToken(token string) TokenSetupResult {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" || strings.EqualFold(trimmed, "PASTE_TOKEN_HERE") {
		return TokenSetupResult{Code: "empty"}
	}
	if len(trimmed) > 8192 {
		return TokenSetupResult{Code: "invalid"}
	}

	apiClient, err := clashapi.New(trimmed)
	if err != nil {
		return TokenSetupResult{Code: "invalid"}
	}
	verificationContext, cancel := context.WithTimeout(app.ctx, tokenVerificationTimeout)
	defer cancel()
	if _, _, err := apiClient.Cards(verificationContext, 1, ""); err != nil {
		return TokenSetupResult{Code: classifyTokenError(err)}
	}

	app.operationMutex.Lock()
	defer app.operationMutex.Unlock()
	if app.tokenStore == nil {
		return TokenSetupResult{Code: "localStorage"}
	}
	if err := app.tokenStore.Save(trimmed); err != nil {
		return TokenSetupResult{Code: "localStorage"}
	}
	app.stopServicesLocked()
	if err := app.startServicesLocked(trimmed); err != nil {
		app.setStartupError("internal", err)
		return TokenSetupResult{Code: "internal"}
	}
	return TokenSetupResult{Success: true, Code: "ok"}
}

func (app *App) DeleteAllLocalData() ActionResult {
	app.operationMutex.Lock()
	defer app.operationMutex.Unlock()

	root := filepath.Clean(app.paths.Root)
	database := filepath.Clean(app.paths.Database)
	cardCache := filepath.Clean(app.paths.CardCache)
	credentials := filepath.Clean(app.paths.Credentials)
	if root == "." || root == "" ||
		!samePath(filepath.Dir(database), root) ||
		!samePath(filepath.Dir(cardCache), root) ||
		!samePath(filepath.Dir(credentials), root) {
		app.setStartupError("localStorage", errors.New("refusing to delete unexpected application data paths"))
		return ActionResult{Code: "localStorage"}
	}

	app.stopServicesLocked()
	if app.store != nil {
		if err := app.store.Close(); err != nil {
			app.setStartupError("localStorage", err)
			return ActionResult{Code: "localStorage"}
		}
		app.store = nil
	}
	files := []string{credentials, database, database + "-wal", database + "-shm"}
	for _, path := range files {
		if err := removeFileIfPresent(path); err != nil {
			app.setStartupError("localStorage", err)
			return ActionResult{Code: "localStorage"}
		}
	}
	if err := os.RemoveAll(cardCache); err != nil {
		app.setStartupError("localStorage", err)
		return ActionResult{Code: "localStorage"}
	}
	_ = os.Remove(root)
	if err := app.openStorageLocked(); err != nil {
		app.setStartupError("localStorage", err)
		return ActionResult{Code: "localStorage"}
	}
	app.stateMutex.Lock()
	app.hasToken = false
	app.startupError = nil
	app.startupErrorCode = ""
	app.stateMutex.Unlock()
	return ActionResult{Success: true, Code: "ok"}
}

func removeFileIfPresent(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func samePath(left, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

func (app *App) Search(input string, requireTargetScript bool) (opponentscout.SearchResult, error) {
	app.operationMutex.RLock()
	defer app.operationMutex.RUnlock()
	if err := app.ensureReady(); err != nil {
		return opponentscout.SearchResult{}, err
	}
	app.stateMutex.RLock()
	service := app.searchService
	app.stateMutex.RUnlock()
	return service.Search(app.ctx, opponentscout.SearchRequest{
		Input:               input,
		RequireTargetScript: requireTargetScript,
	})
}

func (app *App) GetDecks(playerTag string) (battlelog.Result, error) {
	app.operationMutex.RLock()
	defer app.operationMutex.RUnlock()
	if err := app.ensureReady(); err != nil {
		return battlelog.Result{}, err
	}
	app.stateMutex.RLock()
	service := app.battleService
	app.stateMutex.RUnlock()
	return service.LatestRankedDecks(app.ctx, playerTag)
}

func (app *App) ensureReady() error {
	app.stateMutex.RLock()
	defer app.stateMutex.RUnlock()
	if app.startupError != nil {
		return fmt.Errorf("application startup failed: %w", app.startupError)
	}
	if !app.hasToken {
		return config.ErrTokenMissing
	}
	if app.syncService == nil || !app.syncService.Status().Ready {
		return ErrDatabaseNotReady
	}
	return nil
}

func (app *App) setStartupError(code string, err error) {
	app.stateMutex.Lock()
	defer app.stateMutex.Unlock()
	app.startupError = err
	app.startupErrorCode = code
}

func (app *App) setTokenState(hasToken bool) {
	app.stateMutex.Lock()
	defer app.stateMutex.Unlock()
	app.hasToken = hasToken
	app.startupError = nil
	app.startupErrorCode = ""
}

func classifyTokenError(err error) string {
	var apiError *clashapi.APIError
	if errors.As(err, &apiError) {
		switch apiError.StatusCode {
		case http.StatusUnauthorized:
			return "invalid"
		case http.StatusForbidden:
			message := strings.ToLower(apiError.Message + " " + apiError.Reason)
			if strings.Contains(message, "ip") {
				return "ipNotAllowed"
			}
			return "invalid"
		case http.StatusTooManyRequests:
			return "rateLimited"
		default:
			if apiError.StatusCode >= 500 {
				return "serviceUnavailable"
			}
			return "invalid"
		}
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "network"
	}
	var networkError net.Error
	if errors.As(err, &networkError) {
		return "network"
	}
	return "verificationFailed"
}
