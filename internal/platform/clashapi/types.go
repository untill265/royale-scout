package clashapi

type Page[T any] struct {
	Items  []T    `json:"items"`
	Paging Paging `json:"paging"`
}

type Paging struct {
	Cursors Cursors `json:"cursors"`
}

type Cursors struct {
	Before string `json:"before"`
	After  string `json:"after"`
}

type Location struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	IsCountry   bool   `json:"isCountry"`
	CountryCode string `json:"countryCode"`
}

type Season struct {
	ID string `json:"id"`
}

type LeaderboardPlayer struct {
	Tag       string `json:"tag"`
	Name      string `json:"name"`
	ExpLevel  int    `json:"expLevel"`
	EloRating int    `json:"eloRating"`
	Rank      int    `json:"rank"`
	Clan      *Clan  `json:"clan"`
}

type Clan struct {
	Tag     string `json:"tag"`
	Name    string `json:"name"`
	BadgeID int64  `json:"badgeId"`
}

type Battle struct {
	Type       string              `json:"type"`
	BattleTime string              `json:"battleTime"`
	GameMode   NamedResource       `json:"gameMode"`
	Team       []BattleParticipant `json:"team"`
	Opponent   []BattleParticipant `json:"opponent"`
}

type NamedResource struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type BattleParticipant struct {
	Tag   string       `json:"tag"`
	Name  string       `json:"name"`
	Cards []BattleCard `json:"cards"`
}

type BattleCard struct {
	ID                int64    `json:"id"`
	Name              string   `json:"name"`
	Level             int      `json:"level"`
	MaxLevel          int      `json:"maxLevel"`
	EvolutionLevel    int      `json:"evolutionLevel"`
	MaxEvolutionLevel int      `json:"maxEvolutionLevel"`
	ElixirCost        int      `json:"elixirCost"`
	Rarity            string   `json:"rarity"`
	IconURLs          IconURLs `json:"iconUrls"`
}

type Card struct {
	ID                int64    `json:"id"`
	Name              string   `json:"name"`
	MaxLevel          int      `json:"maxLevel"`
	MaxEvolutionLevel int      `json:"maxEvolutionLevel"`
	ElixirCost        int      `json:"elixirCost"`
	Rarity            string   `json:"rarity"`
	IconURLs          IconURLs `json:"iconUrls"`
}

type IconURLs struct {
	Medium          string `json:"medium"`
	EvolutionMedium string `json:"evolutionMedium"`
	HeroMedium      string `json:"heroMedium"`
}
