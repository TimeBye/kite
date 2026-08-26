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
