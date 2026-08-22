package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"royalescout/internal/domain/player"
	"royalescout/internal/domain/topplayer"

	_ "modernc.org/sqlite"
)

type Store struct {
	database *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	database, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open SQLite database: %w", err)
	}
	database.SetMaxOpenConns(4)
	database.SetMaxIdleConns(4)

	store := &Store{database: database}
	if err := store.prepare(context.Background()); err != nil {
		_ = database.Close()
		return nil, err
	}
	return store, nil
}

func (store *Store) Close() error {
	return store.database.Close()
}

func (store *Store) prepare(ctx context.Context) error {
	statements := []string{
		`PRAGMA journal_mode = WAL`,
		`PRAGMA synchronous = NORMAL`,
		`PRAGMA busy_timeout = 5000`,
		`CREATE TABLE IF NOT EXISTS players (
			tag TEXT PRIMARY KEY,
			nickname_original TEXT NOT NULL,
			nickname_normalized TEXT NOT NULL,
			ranked_rating INTEGER NOT NULL,
			country_id INTEGER NOT NULL,
			country_code TEXT NOT NULL,
			synced_at_unix INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_players_ranked_rating ON players(ranked_rating)`,
		`CREATE TABLE IF NOT EXISTS players_stage (
			tag TEXT PRIMARY KEY,
			nickname_original TEXT NOT NULL,
			nickname_normalized TEXT NOT NULL,
			ranked_rating INTEGER NOT NULL,
			country_id INTEGER NOT NULL,
			country_code TEXT NOT NULL,
			synced_at_unix INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS metadata (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS top_players (
			rank INTEGER PRIMARY KEY,
			tag TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			ranked_rating INTEGER NOT NULL,
			clan_tag TEXT NOT NULL,
			clan_name TEXT NOT NULL,
			clan_badge_id INTEGER NOT NULL,
			synced_at_unix INTEGER NOT NULL
		)`,
	}
	for _, statement := range statements {
		if _, err := store.database.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("prepare SQLite database: %w", err)
		}
	}
	return nil
}

func (store *Store) ReplaceTopSnapshot(ctx context.Context, players []topplayer.Player, syncedAt time.Time) error {
	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin top player snapshot transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()

	if _, err := transaction.ExecContext(ctx, `DELETE FROM top_players`); err != nil {
		return fmt.Errorf("clear top player snapshot: %w", err)
	}
	insert, err := transaction.PrepareContext(ctx, `
		INSERT INTO top_players (
			rank, tag, name, ranked_rating, clan_tag, clan_name,
			clan_badge_id, synced_at_unix
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare top player insert: %w", err)
	}
	defer insert.Close()

	syncedAtUnix := syncedAt.UTC().Unix()
	for _, storedPlayer := range players {
		if storedPlayer.Rank < 1 || storedPlayer.Rank > 250 || storedPlayer.Tag == "" {
			return fmt.Errorf("invalid top player at rank %d", storedPlayer.Rank)
		}
		if _, err := insert.ExecContext(
			ctx,
			storedPlayer.Rank,
			storedPlayer.Tag,
			storedPlayer.Name,
			storedPlayer.RankedRating,
			storedPlayer.ClanTag,
			storedPlayer.ClanName,
			storedPlayer.ClanBadgeID,
			syncedAtUnix,
		); err != nil {
			return fmt.Errorf("insert top player %q: %w", storedPlayer.Tag, err)
		}
	}
	if err := insert.Close(); err != nil {
		return fmt.Errorf("close top player insert: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit top player snapshot: %w", err)
	}
	return nil
}

func (store *Store) ListTop(ctx context.Context) ([]topplayer.Player, error) {
	rows, err := store.database.QueryContext(ctx, `
		SELECT rank, tag, name, ranked_rating, clan_tag, clan_name,
		       clan_badge_id, synced_at_unix
		FROM top_players
		ORDER BY rank
	`)
	if err != nil {
		return nil, fmt.Errorf("query top players: %w", err)
	}
	defer rows.Close()

	players := make([]topplayer.Player, 0, 250)
	for rows.Next() {
		var stored topplayer.Player
		var syncedAtUnix int64
		if err := rows.Scan(
			&stored.Rank,
			&stored.Tag,
			&stored.Name,
			&stored.RankedRating,
			&stored.ClanTag,
			&stored.ClanName,
			&stored.ClanBadgeID,
			&syncedAtUnix,
		); err != nil {
			return nil, fmt.Errorf("scan top player: %w", err)
		}
		stored.SyncedAt = time.Unix(syncedAtUnix, 0).UTC()
		players = append(players, stored)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate top players: %w", err)
	}
	return players, nil
}

func (store *Store) ClearTop(ctx context.Context) error {
	if _, err := store.database.ExecContext(ctx, `DELETE FROM top_players`); err != nil {
		return fmt.Errorf("clear top players: %w", err)
	}
	return nil
}

func (store *Store) SearchByRating(ctx context.Context, minimum, maximum int) ([]player.Player, error) {
	rows, err := store.database.QueryContext(ctx, `
		SELECT tag, nickname_original, nickname_normalized, ranked_rating,
		       country_id, country_code, synced_at_unix
		FROM players
		WHERE ranked_rating BETWEEN ? AND ?
	`, minimum, maximum)
	if err != nil {
		return nil, fmt.Errorf("query players by rating: %w", err)
	}
	defer rows.Close()

	players := make([]player.Player, 0)
	for rows.Next() {
		var stored player.Player
		var syncedAtUnix int64
		if err := rows.Scan(
			&stored.Tag,
			&stored.NicknameOriginal,
			&stored.NicknameNormalized,
			&stored.RankedRating,
			&stored.CountryID,
			&stored.CountryCode,
			&syncedAtUnix,
		); err != nil {
			return nil, fmt.Errorf("scan player: %w", err)
		}
		stored.SyncedAt = time.Unix(syncedAtUnix, 0).UTC()
		players = append(players, stored)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate players: %w", err)
	}
	return players, nil
}

func (store *Store) ReplaceSnapshot(ctx context.Context, players []player.Player, syncedAt time.Time) error {
	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin player snapshot transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()

	if _, err := transaction.ExecContext(ctx, `DELETE FROM players_stage`); err != nil {
		return fmt.Errorf("clear player staging table: %w", err)
	}
	insert, err := transaction.PrepareContext(ctx, `
		INSERT INTO players_stage (
			tag, nickname_original, nickname_normalized, ranked_rating,
			country_id, country_code, synced_at_unix
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tag) DO UPDATE SET
			nickname_original = excluded.nickname_original,
			nickname_normalized = excluded.nickname_normalized,
			ranked_rating = excluded.ranked_rating,
			country_id = excluded.country_id,
			country_code = excluded.country_code,
			synced_at_unix = excluded.synced_at_unix
	`)
	if err != nil {
		return fmt.Errorf("prepare player staging insert: %w", err)
	}

	syncedAtUnix := syncedAt.UTC().Unix()
	for _, storedPlayer := range players {
		if _, err := insert.ExecContext(
			ctx,
			storedPlayer.Tag,
			storedPlayer.NicknameOriginal,
			storedPlayer.NicknameNormalized,
			storedPlayer.RankedRating,
			storedPlayer.CountryID,
			storedPlayer.CountryCode,
			syncedAtUnix,
		); err != nil {
			_ = insert.Close()
			return fmt.Errorf("stage player %q: %w", storedPlayer.Tag, err)
		}
	}
	if err := insert.Close(); err != nil {
		return fmt.Errorf("close player staging insert: %w", err)
	}

	if _, err := transaction.ExecContext(ctx, `DELETE FROM players`); err != nil {
		return fmt.Errorf("clear active players: %w", err)
	}
	if _, err := transaction.ExecContext(ctx, `
		INSERT INTO players (
			tag, nickname_original, nickname_normalized, ranked_rating,
			country_id, country_code, synced_at_unix
		)
		SELECT tag, nickname_original, nickname_normalized, ranked_rating,
		       country_id, country_code, synced_at_unix
		FROM players_stage
	`); err != nil {
		return fmt.Errorf("publish player snapshot: %w", err)
	}

	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit player snapshot: %w", err)
	}
	return nil
}

func (store *Store) ClearActive(ctx context.Context) error {
	if _, err := store.database.ExecContext(ctx, `DELETE FROM players`); err != nil {
		return fmt.Errorf("clear active players: %w", err)
	}
	return nil
}

func (store *Store) Count(ctx context.Context) (int, error) {
	var count int
	if err := store.database.QueryRowContext(ctx, `SELECT COUNT(*) FROM players`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count active players: %w", err)
	}
	return count, nil
}

func (store *Store) Get(ctx context.Context, key string) (string, bool, error) {
	var value string
	err := store.database.QueryRowContext(ctx, `SELECT value FROM metadata WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get metadata %q: %w", key, err)
	}
	return value, true, nil
}

func (store *Store) Set(ctx context.Context, key, value string) error {
	_, err := store.database.ExecContext(ctx, `
		INSERT INTO metadata (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	if err != nil {
		return fmt.Errorf("set metadata %q: %w", key, err)
	}
	return nil
}
