package helm

import (
	"testing"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	repo "helm.sh/helm/v4/pkg/repo/v1"
)

func newChartVersion(version string) *repo.ChartVersion {
	return &repo.ChartVersion{
		Metadata: &chart.Metadata{
			Name:    "myapp",
			Version: version,
		},
		URLs: []string{"https://example.com/charts/myapp-" + version + ".tgz"},
	}
}

func TestGetChartVersion(t *testing.T) {
	tests := []struct {
		name        string
		entries     []*repo.ChartVersion
		version     string
		wantVersion string
		wantErr     bool
	}{
		{
			name: "stable version exists",
			entries: []*repo.ChartVersion{
				newChartVersion("1.0.0"),
				newChartVersion("2.0.0"),
			},
			version:     "",
			wantVersion: "2.0.0",
		},
		{
			name: "only prerelease versions falls back to latest",
			entries: []*repo.ChartVersion{
				newChartVersion("2026.5.21-215920-main"),
				newChartVersion("2026.8.24-214415-main"),
			},
			version:     "",
			wantVersion: "2026.8.24-214415-main",
		},
		{
			name:    "chart not in entries returns error",
			entries: []*repo.ChartVersion{},
			version: "",
			wantErr: true,
		},
		{
			name: "exact version requested",
			entries: []*repo.ChartVersion{
				newChartVersion("1.0.0"),
				newChartVersion("2.0.0"),
			},
			version:     "1.0.0",
			wantVersion: "1.0.0",
		},
		{
			name: "nonexistent exact version returns error",
			entries: []*repo.ChartVersion{
				newChartVersion("1.0.0"),
			},
			version: "9.9.9",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			indexFile := repo.NewIndexFile()
			indexFile.Entries["myapp"] = tt.entries
			indexFile.SortEntries()

			got, err := getChartVersion(indexFile, "myapp", tt.version)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Version != tt.wantVersion {
				t.Fatalf("version = %q, want %q", got.Version, tt.wantVersion)
			}
		})
	}
}

func TestHelmChartMatchesQuery(t *testing.T) {
	chart := helmChart{
		Name:           "my-app",
		RepositoryName: "stable",
		Version:        "1.2.3",
		AppVersion:     "2.0.0",
		Description:    "A simple web application",
		Keywords:       []string{"web", "frontend", "monitoring"},
	}

	tests := []struct {
		name  string
		chart helmChart
		query string
		want  bool
	}{
		{
			name:  "matches chart name",
			chart: chart,
			query: "my-app",
			want:  true,
		},
		{
			name:  "matches partial chart name",
			chart: chart,
			query: "my-app",
			want:  true,
		},
		{
			name:  "matches repository name",
			chart: chart,
			query: "stable",
			want:  true,
		},
		{
			name:  "matches version",
			chart: chart,
			query: "1.2.3",
			want:  true,
		},
		{
			name:  "matches app version",
			chart: chart,
			query: "2.0.0",
			want:  true,
		},
		{
			name:  "matches description",
			chart: chart,
			query: "web application",
			want:  true,
		},
		{
			name:  "matches keyword",
			chart: chart,
			query: "frontend",
			want:  true,
		},
		{
			name:  "matches keyword partial",
			chart: chart,
			query: "monitor",
			want:  true,
		},
		{
			name:  "no match",
			chart: chart,
			query: "database",
			want:  false,
		},
		{
			name:  "empty query matches",
			chart: chart,
			query: "",
			want:  true,
		},
		{
			name: "matches with empty keywords",
			chart: helmChart{
				Name:        "blank",
				Description: "no keywords here",
			},
			query: "blank",
			want:  true,
		},
		{
			name: "partial match in description",
			chart: helmChart{
				Name:        "app",
				Description: "Kubernetes monitoring agent",
			},
			query: "kubernetes",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := helmChartMatchesQuery(tt.chart, tt.query)
			if got != tt.want {
				t.Fatalf("helmChartMatchesQuery(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestPaginate(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	tests := []struct {
		name   string
		items  []int
		limit  int
		offset int
		want   []int
	}{
		{
			name:   "no limit returns all",
			items:  items,
			limit:  0,
			offset: 0,
			want:   items,
		},
		{
			name:   "first page",
			items:  items,
			limit:  3,
			offset: 0,
			want:   []int{1, 2, 3},
		},
		{
			name:   "second page",
			items:  items,
			limit:  3,
			offset: 3,
			want:   []int{4, 5, 6},
		},
		{
			name:   "last partial page",
			items:  items,
			limit:  3,
			offset: 9,
			want:   []int{10},
		},
		{
			name:   "offset beyond total returns empty",
			items:  items,
			limit:  3,
			offset: 15,
			want:   []int{},
		},
		{
			name:   "limit larger than total",
			items:  items,
			limit:  20,
			offset: 0,
			want:   items,
		},
		{
			name:   "empty items",
			items:  []int{},
			limit:  5,
			offset: 0,
			want:   []int{},
		},
		{
			name:   "negative limit returns all",
			items:  items,
			limit:  -1,
			offset: 0,
			want:   items,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := paginate(tt.items, tt.limit, tt.offset)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("index %d = %d, want %d", i, got[i], tt.want[i])
				}
			}
		})
	}
}
