package cardcache

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"royalescout/internal/application/battlelog"
	"royalescout/internal/platform/netpolicy"
)

const maximumImageBytes = 5 << 20

type downloadFlight struct {
	done chan struct{}
	err  error
}

type Cache struct {
	directory string
	http      *http.Client
	mutex     sync.Mutex
	memory    map[string]string
	flights   map[string]*downloadFlight
}

func New(directory string) (*Cache, error) {
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, fmt.Errorf("create card image cache directory: %w", err)
	}
	return &Cache{
		directory: directory,
		http:      netpolicy.NewHTTPSClient(20*time.Second, "api-assets.clashroyale.com"),
		memory:    make(map[string]string),
		flights:   make(map[string]*downloadFlight),
	}, nil
}

func (cache *Cache) DataURL(ctx context.Context, request battlelog.CardImageRequest) (string, error) {
	if request.Key == "" || request.URL == "" {
		return "", errors.New("card image key and URL are required")
	}
	if !safeKey(request.Key) {
		return "", errors.New("card image key contains unsupported characters")
	}

	cache.mutex.Lock()
	if dataURL, found := cache.memory[request.Key]; found {
		cache.mutex.Unlock()
		return dataURL, nil
	}
	cache.mutex.Unlock()

	if err := cache.ensureFile(ctx, request); err != nil {
		return "", err
	}
	contents, err := os.ReadFile(cache.path(request.Key))
	if err != nil {
		return "", fmt.Errorf("read cached card image: %w", err)
	}
	contentType := http.DetectContentType(contents)
	if !strings.HasPrefix(contentType, "image/") {
		return "", fmt.Errorf("cached card asset %q is not an image", request.Key)
	}
	dataURL := "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(contents)

	cache.mutex.Lock()
	cache.memory[request.Key] = dataURL
	cache.mutex.Unlock()
	return dataURL, nil
}

func (cache *Cache) Preload(ctx context.Context, requests []battlelog.CardImageRequest, concurrency int) error {
	if concurrency < 1 {
		concurrency = 1
	}
	jobs := make(chan battlelog.CardImageRequest)
	errorsChannel := make(chan error, len(requests))
	var workers sync.WaitGroup
	for range concurrency {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for request := range jobs {
				if _, err := cache.DataURL(ctx, request); err != nil {
					errorsChannel <- err
				}
			}
		}()
	}
	for _, request := range requests {
		select {
		case jobs <- request:
		case <-ctx.Done():
			close(jobs)
			workers.Wait()
			return ctx.Err()
		}
	}
	close(jobs)
	workers.Wait()
	close(errorsChannel)

	var firstError error
	for err := range errorsChannel {
		if firstError == nil {
			firstError = err
		}
	}
	return firstError
}

func (cache *Cache) ensureFile(ctx context.Context, request battlelog.CardImageRequest) error {
	if _, err := os.Stat(cache.path(request.Key)); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect cached card image: %w", err)
	}

	cache.mutex.Lock()
	if flight, found := cache.flights[request.Key]; found {
		cache.mutex.Unlock()
		select {
		case <-flight.done:
			return flight.err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	flight := &downloadFlight{done: make(chan struct{})}
	cache.flights[request.Key] = flight
	cache.mutex.Unlock()

	flight.err = cache.download(ctx, request)
	cache.mutex.Lock()
	delete(cache.flights, request.Key)
	close(flight.done)
	cache.mutex.Unlock()
	return flight.err
}

func (cache *Cache) download(ctx context.Context, image battlelog.CardImageRequest) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, image.URL, nil)
	if err != nil {
		return fmt.Errorf("create card image request: %w", err)
	}
	response, err := cache.http.Do(request)
	if err != nil {
		return fmt.Errorf("download card image: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("download card image: HTTP %d", response.StatusCode)
	}

	contents, err := io.ReadAll(io.LimitReader(response.Body, maximumImageBytes+1))
	if err != nil {
		return fmt.Errorf("read card image: %w", err)
	}
	if len(contents) > maximumImageBytes {
		return errors.New("card image exceeds size limit")
	}
	contentType := response.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(contents)
	}
	if !strings.HasPrefix(contentType, "image/") {
		return fmt.Errorf("card asset %q is not an image", image.Key)
	}

	temporary, err := os.CreateTemp(cache.directory, image.Key+"-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary card image: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(contents); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary card image: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary card image: %w", err)
	}
	if err := os.Rename(temporaryPath, cache.path(image.Key)); err != nil {
		return fmt.Errorf("publish cached card image: %w", err)
	}
	return nil
}

func (cache *Cache) path(key string) string {
	return filepath.Join(cache.directory, key+".image")
}

func safeKey(key string) bool {
	numericKey := key
	for _, suffix := range []string{"-evolution", "-hero"} {
		if strings.HasSuffix(numericKey, suffix) {
			numericKey = strings.TrimSuffix(numericKey, suffix)
			break
		}
	}
	if numericKey == "" {
		return false
	}
	for _, character := range numericKey {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}
