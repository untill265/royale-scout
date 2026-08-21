package opponentscout

import (
	"context"
	"testing"
	"time"

	"royalescout/internal/domain/player"
)

func TestNicknameNormalization(t *testing.T) {
	normalizer := NewNicknameNormalizer()
	tests := map[string]string{
		"José":       "jose",
		"Суслик":     "suslik",
		"суслик":     "suslik",
		"SU✨SLIK":    "suslik",
		"  Pro  张伟 ": "pro张伟",
	}
	for input, expected := range tests {
		if actual := normalizer.Normalize(input); actual != expected {
			t.Errorf("Normalize(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestScriptDetection(t *testing.T) {
	detector := ScriptDetectionService{}
	tests := map[string]bool{
		"PRO":    false,
		"Суслик": false,
		"José":   false,
		"PRO张伟":  true,
		"かな":     true,
		"カナ":     true,
		"김민수":    true,
		"محمد":   true,
	}
	for input, expected := range tests {
		if actual := detector.ContainsTargetScript(input); actual != expected {
			t.Errorf("ContainsTargetScript(%q) = %t, want %t", input, actual, expected)
		}
	}
}

func TestFixed48RatingRange(t *testing.T) {
	minimum, maximum := (Fixed48RatingRangeStrategy{}).Bounds(2910)
	if minimum != 2862 || maximum != 2958 {
		t.Fatalf("Bounds(2910) = %d..%d", minimum, maximum)
	}
}

func TestSearchCheckboxIsPositiveOnlyFilter(t *testing.T) {
	normalizer := NewNicknameNormalizer()
	repository := &memoryRepository{players: []player.Player{
		{Tag: "#1", NicknameOriginal: "PRO", NicknameNormalized: normalizer.Normalize("PRO"), RankedRating: 2910},
		{Tag: "#2", NicknameOriginal: "PRO张伟", NicknameNormalized: normalizer.Normalize("PRO张伟"), RankedRating: 2911},
	}}
	service := NewSearchService(repository)

	withoutFilter, err := service.Search(context.Background(), SearchRequest{Input: "2910 pro"})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(withoutFilter.Candidates) != 2 {
		t.Fatalf("without filter candidates = %d, want 2", len(withoutFilter.Candidates))
	}

	withFilter, err := service.Search(context.Background(), SearchRequest{Input: "2910 pro", RequireTargetScript: true})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(withFilter.Candidates) != 1 || withFilter.Candidates[0].NicknameOriginal != "PRO张伟" {
		t.Fatalf("with filter candidates = %#v", withFilter.Candidates)
	}
}

func TestSearchAcceptsRatingAndCheckboxWithoutNickname(t *testing.T) {
	normalizer := NewNicknameNormalizer()
	repository := &memoryRepository{players: []player.Player{
		{Tag: "#1", NicknameOriginal: "Plain", NicknameNormalized: normalizer.Normalize("Plain"), RankedRating: 2910},
		{Tag: "#2", NicknameOriginal: "김민수", NicknameNormalized: normalizer.Normalize("김민수"), RankedRating: 2958},
	}}
	service := NewSearchService(repository)

	result, err := service.Search(context.Background(), SearchRequest{Input: "2910", RequireTargetScript: true})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].NicknameOriginal != "김민수" {
		t.Fatalf("candidates = %#v", result.Candidates)
	}
}

type memoryRepository struct {
	players []player.Player
}

func (repository *memoryRepository) SearchByRating(_ context.Context, minimum, maximum int) ([]player.Player, error) {
	result := make([]player.Player, 0, len(repository.players))
	for _, storedPlayer := range repository.players {
		if storedPlayer.RankedRating >= minimum && storedPlayer.RankedRating <= maximum {
			result = append(result, storedPlayer)
		}
	}
	return result, nil
}

func (*memoryRepository) ReplaceSnapshot(context.Context, []player.Player, time.Time) error {
	return nil
}

func (*memoryRepository) ClearActive(context.Context) error {
	return nil
}

func (repository *memoryRepository) Count(context.Context) (int, error) {
	return len(repository.players), nil
}
