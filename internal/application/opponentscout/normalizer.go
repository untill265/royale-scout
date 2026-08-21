package opponentscout

import (
	"strings"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

type NicknameNormalizer struct {
	fold cases.Caser
}

func NewNicknameNormalizer() NicknameNormalizer {
	return NicknameNormalizer{fold: cases.Fold()}
}

func (normalizer NicknameNormalizer) Normalize(value string) string {
	folded := normalizer.fold.String(value)
	decomposed := norm.NFKD.String(folded)

	var normalized strings.Builder
	for _, character := range decomposed {
		if unicode.Is(unicode.Mn, character) || unicode.Is(unicode.Me, character) {
			continue
		}
		if replacement, exists := cyrillicToLatin[character]; exists {
			normalized.WriteString(replacement)
			continue
		}
		if unicode.IsLetter(character) || unicode.IsNumber(character) {
			normalized.WriteRune(character)
		}
	}

	return normalized.String()
}

var cyrillicToLatin = map[rune]string{
	'а': "a",
	'б': "b",
	'в': "v",
	'г': "g",
	'д': "d",
	'е': "e",
	'ё': "yo",
	'ж': "zh",
	'з': "z",
	'и': "i",
	'й': "y",
	'к': "k",
	'л': "l",
	'м': "m",
	'н': "n",
	'о': "o",
	'п': "p",
	'р': "r",
	'с': "s",
	'т': "t",
	'у': "u",
	'ф': "f",
	'х': "kh",
	'ц': "ts",
	'ч': "ch",
	'ш': "sh",
	'щ': "shch",
	'ъ': "",
	'ы': "y",
	'ь': "",
	'э': "e",
	'ю': "yu",
	'я': "ya",
	'і': "i",
	'ї': "yi",
	'є': "ye",
	'ґ': "g",
}
