package opponentscout

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

var (
	ErrInvalidSearchInput = errors.New("search must start with Ranked medals")
	searchInputPattern    = regexp.MustCompile(`^\s*(\d+)(?:\s+(.*?))?\s*$`)
)

type ParsedQuery struct {
	Rating           int
	NicknameFragment string
}

func ParseQuery(input string) (ParsedQuery, error) {
	matches := searchInputPattern.FindStringSubmatch(input)
	if matches == nil {
		return ParsedQuery{}, ErrInvalidSearchInput
	}

	rating, err := strconv.Atoi(matches[1])
	if err != nil || rating < 0 || rating > 99999 {
		return ParsedQuery{}, ErrInvalidSearchInput
	}

	return ParsedQuery{
		Rating:           rating,
		NicknameFragment: strings.TrimSpace(matches[2]),
	}, nil
}
