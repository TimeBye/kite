package helm

import (
	"testing"
	"time"

	"github.com/zxh326/kite/pkg/model"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	repo "helm.sh/helm/v4/pkg/repo/v1"
)

func TestLoadRepositoryIndexCacheHit(t *testing.T) {
	h := NewHelmChartHandler()
	repository := testRepository()
	cachedIndex := repo.NewIndexFile()
	cachedIndex.Entries["cached-chart"] = []*repo.ChartVersion{
		newChartVersion("1.0.0"),
	}
	cacheKey := repositoryIndexCacheKey(repository)
	h.indexCache[cacheKey] = cachedRepositoryIndex{
		indexFile: cachedIndex,
		expiresAt: time.Now().Add(1 * time.Minute),
	}

	// refresh=false: should return cached index without network call
	got, err := h.loadRepositoryIndex(repository, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := got.Entries["cached-chart"]; !ok {
		t.Fatal("expected cached chart entry, got none")
	}
}

func TestLoadRepositoryIndexRefreshSkipsCache(t *testing.T) {
	h := NewHelmChartHandler()
	repository := testRepository()
	cachedIndex := repo.NewIndexFile()
	cachedIndex.Entries["cached-chart"] = []*repo.ChartVersion{
		newChartVersion("1.0.0"),
	}
	cacheKey := repositoryIndexCacheKey(repository)
	h.indexCache[cacheKey] = cachedRepositoryIndex{
		indexFile: cachedIndex,
		expiresAt: time.Now().Add(1 * time.Minute),
	}

	// refresh=true: should skip cache and attempt network fetch,
	// which will fail because the URL is not a real Helm repository
	_, err := h.loadRepositoryIndex(repository, true)
	if err == nil {
		t.Fatal("expected error from network fetch when refresh=true, got nil")
	}

	// cached entry should still be present (refresh updates cache on
	// success, but should not clear it on failure)
	h.indexCacheMu.Lock()
	_, ok := h.indexCache[cacheKey]
	h.indexCacheMu.Unlock()
	if !ok {
		t.Fatal("cache entry should still exist after failed refresh")
	}
}

func TestLoadRepositoryIndexExpiredCacheFetches(t *testing.T) {
	h := NewHelmChartHandler()
	repository := testRepository()
	cachedIndex := repo.NewIndexFile()
	cachedIndex.Entries["expired-chart"] = []*repo.ChartVersion{
		newChartVersion("1.0.0"),
	}
	cacheKey := repositoryIndexCacheKey(repository)
	h.indexCache[cacheKey] = cachedRepositoryIndex{
		indexFile: cachedIndex,
		expiresAt: time.Now().Add(-1 * time.Minute), // already expired
	}

	// expired cache should be skipped, triggering network fetch which fails
	_, err := h.loadRepositoryIndex(repository, false)
	if err == nil {
		t.Fatal("expected error from network fetch when cache expired, got nil")
	}
}

func TestLoadChartContentCacheHit(t *testing.T) {
	h := NewHelmChartHandler()
	repository := testRepository()
	entry := newChartVersion("1.0.0")
	cachedContent := helmChartContent{
		Readme: "cached readme",
		Values: "cached values",
	}
	cacheKey := chartContentCacheKey(repository, entry)
	h.contentCache[cacheKey] = cachedChartContent{
		content:   cachedContent,
		expiresAt: time.Now().Add(1 * time.Minute),
	}

	// refresh=false: should return cached content without network call
	got, err := h.loadChartContent(repository, entry, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Readme != "cached readme" {
		t.Fatalf("Readme = %q, want %q", got.Readme, "cached readme")
	}
	if got.Values != "cached values" {
		t.Fatalf("Values = %q, want %q", got.Values, "cached values")
	}
}

func TestLoadChartContentRefreshSkipsCache(t *testing.T) {
	h := NewHelmChartHandler()
	repository := testRepository()
	entry := newChartVersion("1.0.0")
	cachedContent := helmChartContent{
		Readme: "cached readme",
		Values: "cached values",
	}
	cacheKey := chartContentCacheKey(repository, entry)
	h.contentCache[cacheKey] = cachedChartContent{
		content:   cachedContent,
		expiresAt: time.Now().Add(1 * time.Minute),
	}

	// refresh=true: should skip cache and attempt network fetch,
	// which will fail because the URL is not a real chart archive
	_, err := h.loadChartContent(repository, entry, true)
	if err == nil {
		t.Fatal("expected error from network fetch when refresh=true, got nil")
	}
}

func TestLoadChartContentExpiredCacheFetches(t *testing.T) {
	h := NewHelmChartHandler()
	repository := testRepository()
	entry := newChartVersion("1.0.0")
	cachedContent := helmChartContent{
		Readme: "expired readme",
	}
	cacheKey := chartContentCacheKey(repository, entry)
	h.contentCache[cacheKey] = cachedChartContent{
		content:   cachedContent,
		expiresAt: time.Now().Add(-1 * time.Minute), // already expired
	}

	// expired cache should be skipped, triggering network fetch which fails
	_, err := h.loadChartContent(repository, entry, false)
	if err == nil {
		t.Fatal("expected error from network fetch when cache expired, got nil")
	}
}

func TestLoadChartContentNoURLsReturnsEmpty(t *testing.T) {
	h := NewHelmChartHandler()
	repository := testRepository()
	entry := &repo.ChartVersion{
		Metadata: &chart.Metadata{
			Name:    "myapp",
			Version: "1.0.0",
		},
		URLs: []string{}, // no URLs
	}

	got, err := h.loadChartContent(repository, entry, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Readme != "" || got.Values != "" {
		t.Fatal("expected empty content when no URLs")
	}
}

func testRepository() model.HelmRepository {
	return model.HelmRepository{
		Name: "test-repo",
		URL:  "https://invalid.example.com/charts",
	}
}
