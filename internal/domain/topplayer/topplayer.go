package topplayer

import "time"

type Player struct {
	Tag          string
	Name         string
	Rank         int
	RankedRating int
	ClanTag      string
	ClanName     string
	ClanBadgeID  int64
	SyncedAt     time.Time
}
