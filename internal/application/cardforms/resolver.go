package cardforms

import (
	"fmt"

	"royalescout/internal/platform/clashapi"
)

type Form string

const (
	Normal    Form = "normal"
	Evolution Form = "evolution"
	Hero      Form = "hero"
)

type Asset struct {
	Key string
	URL string
}

const eliteBarbariansID int64 = 26000043

var missingOfficialAssets = map[int64]map[Form]string{
	eliteBarbariansID: {
		Evolution: "https://cdn.royaleapi.com/static/img/cards/elite-barbarians-ev1.png",
	},
}

func ResolveBattleCard(card clashapi.BattleCard) (Asset, Form) {
	form := selectedForm(card.EvolutionLevel)
	url := formURL(form, card.IconURLs)
	if url == "" {
		url = fallbackURL(card.ID, form)
	}
	if url == "" {
		return Asset{Key: fmt.Sprintf("%d", card.ID), URL: card.IconURLs.Medium}, form
	}
	return Asset{Key: assetKey(card.ID, form), URL: url}, form
}

func CatalogAssets(card clashapi.Card) []Asset {
	assets := make([]Asset, 0, 3)
	if card.IconURLs.Medium != "" {
		assets = append(assets, Asset{Key: assetKey(card.ID, Normal), URL: card.IconURLs.Medium})
	}
	for _, form := range []Form{Evolution, Hero} {
		url := formURL(form, card.IconURLs)
		if url == "" {
			url = fallbackURL(card.ID, form)
		}
		if url != "" {
			assets = append(assets, Asset{Key: assetKey(card.ID, form), URL: url})
		}
	}
	return assets
}

func selectedForm(evolutionLevel int) Form {
	switch evolutionLevel {
	case 1:
		return Evolution
	case 2:
		return Hero
	default:
		if evolutionLevel > 2 {
			return Hero
		}
		return Normal
	}
}

func formURL(form Form, urls clashapi.IconURLs) string {
	switch form {
	case Evolution:
		return urls.EvolutionMedium
	case Hero:
		return urls.HeroMedium
	default:
		return urls.Medium
	}
}

func fallbackURL(cardID int64, form Form) string {
	forms, found := missingOfficialAssets[cardID]
	if !found {
		return ""
	}
	return forms[form]
}

func assetKey(cardID int64, form Form) string {
	switch form {
	case Evolution:
		return fmt.Sprintf("%d-evolution", cardID)
	case Hero:
		return fmt.Sprintf("%d-hero", cardID)
	default:
		return fmt.Sprintf("%d", cardID)
	}
}
