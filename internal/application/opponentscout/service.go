package opponentscout

import (
	"context"
	"sort"
	"strings"

	"royalescout/internal/application/ports"
)

type SearchRequest struct {
	Input               string
	RequireTargetScript bool
	TargetScriptCount   int
	EmojiFilter         OptionalBooleanFilter
	ArabicFilter        OptionalBooleanFilter
}

type OptionalBooleanFilter int

const (
	OptionalBooleanAny OptionalBooleanFilter = iota
	OptionalBooleanYes
	OptionalBooleanNo
)

type Candidate struct {
	PlayerTag        string `json:"playerTag"`
	NicknameOriginal string `json:"nicknameOriginal"`
	RankedRating     int    `json:"rankedRating"`
	CountryCode      string `json:"countryCode"`
}

type SearchResult struct {
	Candidates []Candidate `json:"candidates"`
}

type SearchService struct {
	repository     ports.PlayerRepository
	rangeStrategy  RatingRangeStrategy
	normalizer     NicknameNormalizer
	scriptDetector ScriptDetectionService
}

func NewSearchService(repository ports.PlayerRepository) SearchService {
	return SearchService{
		repository:     repository,
		rangeStrategy:  Fixed48RatingRangeStrategy{},
		normalizer:     NewNicknameNormalizer(),
		scriptDetector: ScriptDetectionService{},
	}
}

func (service SearchService) Search(ctx context.Context, request SearchRequest) (SearchResult, error) {
	query, err := ParseQuery(request.Input)
	if err != nil {
		return SearchResult{}, err
	}
	minimum, maximum := service.rangeStrategy.Bounds(query.Rating)
	players, err := service.repository.SearchByRating(ctx, minimum, maximum)
	if err != nil {
		return SearchResult{}, err
	}

	normalizedFragment := service.normalizer.Normalize(query.NicknameFragment)
	candidates := make([]Candidate, 0, len(players))
	for _, storedPlayer := range players {
		if normalizedFragment != "" && !strings.Contains(storedPlayer.NicknameNormalized, normalizedFragment) {
			continue
		}
		if request.RequireTargetScript {
			features := service.scriptDetector.Analyze(storedPlayer.NicknameOriginal)
			if !features.ContainsTargetScript ||
				!matchesTargetScriptCount(features.TargetScriptCount, request.TargetScriptCount) ||
				!request.EmojiFilter.matches(features.ContainsEmoji) ||
				!request.ArabicFilter.matches(features.ContainsArabic) {
				continue
			}
		}
		candidates = append(candidates, Candidate{
			PlayerTag:        storedPlayer.Tag,
			NicknameOriginal: storedPlayer.NicknameOriginal,
			RankedRating:     storedPlayer.RankedRating,
			CountryCode:      storedPlayer.CountryCode,
		})
	}

	sort.SliceStable(candidates, func(left, right int) bool {
		leftDistance := absolute(candidates[left].RankedRating - query.Rating)
		rightDistance := absolute(candidates[right].RankedRating - query.Rating)
		if leftDistance != rightDistance {
			return leftDistance < rightDistance
		}
		return candidates[left].NicknameOriginal < candidates[right].NicknameOriginal
	})

	return SearchResult{Candidates: candidates}, nil
}

func matchesTargetScriptCount(actual, selected int) bool {
	switch selected {
	case 0:
		return true
	case 1, 2, 3, 4:
		return actual == selected
	case 5:
		return actual >= selected
	default:
		return false
	}
}

func (filter OptionalBooleanFilter) matches(actual bool) bool {
	switch filter {
	case OptionalBooleanAny:
		return true
	case OptionalBooleanYes:
		return actual
	case OptionalBooleanNo:
		return !actual
	default:
		return false
	}
}

func absolute(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
