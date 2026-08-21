package cardforms

import (
	"testing"

	"royalescout/internal/platform/clashapi"
)

func TestResolveBattleCardUsesOnlySelectedForm(t *testing.T) {
	card := clashapi.BattleCard{
		ID:             42,
		EvolutionLevel: 0,
		IconURLs: clashapi.IconURLs{
			Medium:          "normal.png",
			EvolutionMedium: "evolution.png",
			HeroMedium:      "hero.png",
		},
	}
	asset, form := ResolveBattleCard(card)
	if form != Normal || asset.Key != "42" || asset.URL != "normal.png" {
		t.Fatalf("ResolveBattleCard() = %#v, %q", asset, form)
	}
}

func TestResolveBattleCardDistinguishesEvolutionAndHero(t *testing.T) {
	images := clashapi.IconURLs{Medium: "normal.png", EvolutionMedium: "evolution.png", HeroMedium: "hero.png"}
	tests := []struct {
		level int
		form  Form
		key   string
		url   string
	}{
		{level: 1, form: Evolution, key: "42-evolution", url: "evolution.png"},
		{level: 2, form: Hero, key: "42-hero", url: "hero.png"},
	}
	for _, test := range tests {
		asset, form := ResolveBattleCard(clashapi.BattleCard{ID: 42, EvolutionLevel: test.level, IconURLs: images})
		if form != test.form || asset.Key != test.key || asset.URL != test.url {
			t.Fatalf("level %d resolved to %#v, %q", test.level, asset, form)
		}
	}
}

func TestResolveBattleCardUsesCurrentEliteBarbariansFallback(t *testing.T) {
	asset, form := ResolveBattleCard(clashapi.BattleCard{
		ID:             eliteBarbariansID,
		EvolutionLevel: 1,
		IconURLs:       clashapi.IconURLs{Medium: "normal.png"},
	})
	if form != Evolution || asset.Key != "26000043-evolution" || asset.URL == "" || asset.URL == "normal.png" {
		t.Fatalf("ResolveBattleCard() = %#v, %q", asset, form)
	}
}
