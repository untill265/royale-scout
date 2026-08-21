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
}

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
		if request.RequireTargetScript && !service.scriptDetector.ContainsTargetScript(storedPlayer.NicknameOriginal) {
			continue
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

func absolute(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
