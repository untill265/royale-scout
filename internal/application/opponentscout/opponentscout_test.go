package opponentscout

import (
	"context"
	"testing"
	"time"

	"royalescout/internal/application/ports"
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

func TestNicknameFeatureDetectionCountsOnlyTargetScriptGraphemes(t *testing.T) {
	detector := ScriptDetectionService{}
	tests := []struct {
		name      string
		nickname  string
		count     int
		hasEmoji  bool
		hasArabic bool
	}{
		{name: "Chinese with heart", nickname: "张伟强❤️", count: 3, hasEmoji: true},
		{name: "Chinese mixed with Latin and star", nickname: "PRO张伟★", count: 2, hasEmoji: true},
		{name: "Korean with digits", nickname: "김민수123", count: 3},
		{name: "Arabic with blue heart", nickname: "محمد💙", count: 4, hasEmoji: true, hasArabic: true},
		{name: "Japanese combining mark", nickname: "がく", count: 2},
		{name: "Japanese long vowel mark", nickname: "カー", count: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			features := detector.Analyze(test.nickname)
			if features.TargetScriptCount != test.count || features.ContainsEmoji != test.hasEmoji || features.ContainsArabic != test.hasArabic {
				t.Fatalf("Analyze(%q) = %#v", test.nickname, features)
			}
		})
	}
}

func TestEmojiDetectionHandlesPlayerNicknameSymbolsAndSequences(t *testing.T) {
	detector := ScriptDetectionService{}
	tests := map[string]bool{
		"❤️":       true,
		"♥":        true,
		"★":        true,
		"⭐":        true,
		"✨":        true,
		"🏳️‍🌈":     true,
		"👨‍👩‍👧‍👦":  true,
		"👍🏽":       true,
		"🇰🇷":       true,
		"1️⃣":      true,
		"张伟":       false,
		"محمد":     false,
		"PRO123#*": false,
	}
	for nickname, expected := range tests {
		if actual := detector.Analyze(nickname).ContainsEmoji; actual != expected {
			t.Errorf("ContainsEmoji(%q) = %t, want %t", nickname, actual, expected)
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

func TestSearchAppliesOptionalScriptFiltersIndependently(t *testing.T) {
	normalizer := NewNicknameNormalizer()
	repository := &memoryRepository{players: []player.Player{
		{Tag: "#ZH-EMOJI", NicknameOriginal: "张伟强❤️", NicknameNormalized: normalizer.Normalize("张伟强❤️"), RankedRating: 2910},
		{Tag: "#KO", NicknameOriginal: "김민수", NicknameNormalized: normalizer.Normalize("김민수"), RankedRating: 2910},
		{Tag: "#AR-EMOJI", NicknameOriginal: "محمد💙", NicknameNormalized: normalizer.Normalize("محمد💙"), RankedRating: 2910},
		{Tag: "#AR", NicknameOriginal: "محمد", NicknameNormalized: normalizer.Normalize("محمد"), RankedRating: 2910},
		{Tag: "#ZH-TWO", NicknameOriginal: "张伟", NicknameNormalized: normalizer.Normalize("张伟"), RankedRating: 2910},
	}}
	service := NewSearchService(repository)

	tests := []struct {
		name    string
		request SearchRequest
		wantTag string
	}{
		{
			name: "count and emoji without Arabic selection",
			request: SearchRequest{Input: "2910", RequireTargetScript: true, TargetScriptCount: 3,
				EmojiFilter: OptionalBooleanYes},
			wantTag: "#ZH-EMOJI",
		},
		{
			name: "Arabic and no emoji without count selection",
			request: SearchRequest{Input: "2910", RequireTargetScript: true,
				EmojiFilter: OptionalBooleanNo, ArabicFilter: OptionalBooleanYes},
			wantTag: "#AR",
		},
		{
			name: "non Arabic two symbols without emoji selection",
			request: SearchRequest{Input: "2910", RequireTargetScript: true, TargetScriptCount: 2,
				ArabicFilter: OptionalBooleanNo},
			wantTag: "#ZH-TWO",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := service.Search(context.Background(), test.request)
			if err != nil {
				t.Fatalf("Search() error = %v", err)
			}
			if len(result.Candidates) != 1 || result.Candidates[0].PlayerTag != test.wantTag {
				t.Fatalf("candidates = %#v, want %s", result.Candidates, test.wantTag)
			}
		})
	}
}

func TestSearchIgnoresHiddenAdditionalFilters(t *testing.T) {
	normalizer := NewNicknameNormalizer()
	repository := &memoryRepository{players: []player.Player{
		{Tag: "#PLAIN", NicknameOriginal: "Plain", NicknameNormalized: normalizer.Normalize("Plain"), RankedRating: 2910},
	}}
	service := NewSearchService(repository)

	result, err := service.Search(context.Background(), SearchRequest{
		Input: "2910", TargetScriptCount: 3, EmojiFilter: OptionalBooleanYes, ArabicFilter: OptionalBooleanYes,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].PlayerTag != "#PLAIN" {
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

func (*memoryRepository) ReplaceLocationSnapshots(context.Context, []ports.LocationPlayerSnapshot, time.Time) error {
	return nil
}

func (*memoryRepository) ClearActive(context.Context) error {
	return nil
}

func (repository *memoryRepository) Count(context.Context) (int, error) {
	return len(repository.players), nil
}
