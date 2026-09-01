package opponentscout

import (
	"unicode"

	"github.com/rivo/uniseg"
)

type ScriptDetectionService struct{}

func (ScriptDetectionService) ContainsTargetScript(value string) bool {
	return (ScriptDetectionService{}).Analyze(value).ContainsTargetScript
}

type NicknameFeatures struct {
	ContainsTargetScript bool
	TargetScriptCount    int
	ContainsEmoji        bool
	ContainsArabic       bool
}

func (ScriptDetectionService) Analyze(value string) NicknameFeatures {
	features := NicknameFeatures{}
	graphemes := uniseg.NewGraphemes(value)
	for graphemes.Next() {
		containsTargetScript := false
		for _, character := range graphemes.Runes() {
			if isTargetScript(character) {
				containsTargetScript = true
				features.ContainsTargetScript = true
			}
			if unicode.Is(unicode.Arabic, character) {
				features.ContainsArabic = true
			}
		}
		if containsTargetScript {
			features.TargetScriptCount++
		}
		if isEmojiCluster(graphemes.Runes()) {
			features.ContainsEmoji = true
		}
	}
	return features
}

func isTargetScript(character rune) bool {
	return unicode.Is(unicode.Han, character) ||
		unicode.Is(unicode.Hiragana, character) ||
		unicode.Is(unicode.Katakana, character) ||
		unicode.Is(unicode.Hangul, character) ||
		unicode.Is(unicode.Arabic, character) ||
		character == '\u30fc'
}

func isEmojiCluster(cluster []rune) bool {
	for _, character := range cluster {
		if character == '\ufe0f' || character == '\u20e3' ||
			(character >= '\U0001f000' && character <= '\U0001faff') ||
			(character >= '\U0001f1e6' && character <= '\U0001f1ff') ||
			(character >= '\U0001f3fb' && character <= '\U0001f3ff') ||
			isEmojiSymbol(character) || isDecorativeHeartOrStar(character) {
			return true
		}
	}
	return false
}

func isEmojiSymbol(character rune) bool {
	switch {
	case character == '\u00a9' || character == '\u00ae' || character == '\u203c' || character == '\u2049':
		return true
	case character == '\u2122' || character == '\u2139' || character == '\u2328' || character == '\u23cf':
		return true
	case character >= '\u2194' && character <= '\u2199':
		return true
	case character >= '\u21a9' && character <= '\u21aa':
		return true
	case character >= '\u231a' && character <= '\u231b':
		return true
	case character >= '\u23e9' && character <= '\u23f3':
		return true
	case character >= '\u23f8' && character <= '\u23fa':
		return true
	case character == '\u24c2' || character == '\u25b6' || character == '\u25c0':
		return true
	case character >= '\u25aa' && character <= '\u25ab':
		return true
	case character >= '\u25fb' && character <= '\u25fe':
		return true
	case character >= '\u2600' && character <= '\u27bf':
		return true
	case character >= '\u2b05' && character <= '\u2b07':
		return true
	case character >= '\u2b1b' && character <= '\u2b1c':
		return true
	case character == '\u2b50' || character == '\u2b55' || character == '\u3030' || character == '\u303d':
		return true
	case character == '\u3297' || character == '\u3299':
		return true
	default:
		return false
	}
}

func isDecorativeHeartOrStar(character rune) bool {
	switch character {
	case '♡', '♥', '❤', '❥', '❦', '❧',
		'★', '☆', '⋆', '✦', '✧', '✩', '✪', '✫', '✬', '✭', '✮', '✯', '✰':
		return true
	default:
		return false
	}
}
