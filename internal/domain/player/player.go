package player

import "time"

type Player struct {
	Tag                string
	NicknameOriginal   string
	NicknameNormalized string
	RankedRating       int
	CountryID          int64
	CountryCode        string
	SyncedAt           time.Time
}
