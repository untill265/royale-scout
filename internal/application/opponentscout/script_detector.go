package opponentscout

import "unicode"

type ScriptDetectionService struct{}

func (ScriptDetectionService) ContainsTargetScript(value string) bool {
	for _, character := range value {
		if unicode.Is(unicode.Han, character) ||
			unicode.Is(unicode.Hiragana, character) ||
			unicode.Is(unicode.Katakana, character) ||
			unicode.Is(unicode.Hangul, character) ||
			unicode.Is(unicode.Arabic, character) {
			return true
		}
	}
	return false
}
