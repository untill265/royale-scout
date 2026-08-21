package cardassets

import (
	"context"
	"fmt"

	"royalescout/internal/application/battlelog"
	"royalescout/internal/application/cardforms"
	"royalescout/internal/platform/clashapi"
)

type API interface {
	Cards(context.Context, int, string) (clashapi.Page[clashapi.Card], clashapi.ResponseMeta, error)
}

type Cache interface {
	Preload(context.Context, []battlelog.CardImageRequest, int) error
}

type Preloader struct {
	api         API
	cache       Cache
	concurrency int
}

func NewPreloader(api API, cache Cache, concurrency int) Preloader {
	if concurrency < 1 {
		concurrency = 1
	}
	return Preloader{api: api, cache: cache, concurrency: concurrency}
}

func (preloader Preloader) Run(ctx context.Context) error {
	page, _, err := preloader.api.Cards(ctx, 1000, "")
	if err != nil {
		return fmt.Errorf("load card catalog: %w", err)
	}
	requests := make([]battlelog.CardImageRequest, 0, len(page.Items)*3)
	for _, card := range page.Items {
		for _, asset := range cardforms.CatalogAssets(card) {
			requests = append(requests, battlelog.CardImageRequest{
				Key: asset.Key,
				URL: asset.URL,
			})
		}
	}
	if err := preloader.cache.Preload(ctx, requests, preloader.concurrency); err != nil {
		return fmt.Errorf("preload card images: %w", err)
	}
	return nil
}
