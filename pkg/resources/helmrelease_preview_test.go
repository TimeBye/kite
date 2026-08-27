package resources

import (
	"testing"

	"helm.sh/helm/v4/pkg/chart/common/util"
	chart "helm.sh/helm/v4/pkg/chart/v2"
)

func newTestChart(defaultValues map[string]interface{}) *chart.Chart {
	return &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    "test-chart",
			Version: "1.0.0",
		},
		Values: defaultValues,
	}
}

func TestCoalescePreviewBasics(t *testing.T) {
	chrt := newTestChart(map[string]interface{}{
		"replicaCount": 1,
		"image": map[string]interface{}{
			"repository": "nginx",
			"tag":        "latest",
		},
		"service": map[string]interface{}{
			"type": "ClusterIP",
			"port": 80,
		},
	})

	tests := []struct {
		name       string
		userValues map[string]interface{}
		wantKey    string
		wantValue  interface{}
	}{
		{
			name:       "user override scalar",
			userValues: map[string]interface{}{"replicaCount": 3},
			wantKey:    "replicaCount",
			wantValue:  3,
		},
		{
			name:       "user override nested",
			userValues: map[string]interface{}{"image": map[string]interface{}{"tag": "v2.0.0"}},
			wantKey:    "image.tag",
			wantValue:  "v2.0.0",
		},
		{
			name:       "default preserved when not overridden",
			userValues: map[string]interface{}{"replicaCount": 3},
			wantKey:    "service.port",
			wantValue:  80,
		},
		{
			name:       "user override scalar over default",
			userValues: map[string]interface{}{"replicaCount": 5},
			wantKey:    "replicaCount",
			wantValue:  5,
		},
		{
			name:       "user override nested over default",
			userValues: map[string]interface{}{"image": map[string]interface{}{"tag": "v3.0.0"}},
			wantKey:    "image.tag",
			wantValue:  "v3.0.0",
		},
		{
			name:       "user values merge scalar and nested",
			userValues: map[string]interface{}{"replicaCount": 2, "image": map[string]interface{}{"tag": "v4.0.0"}},
			wantKey:    "image.tag",
			wantValue:  "v4.0.0",
		},
		{
			name:       "user override replaces previous user value",
			userValues: map[string]interface{}{"replicaCount": 7},
			wantKey:    "replicaCount",
			wantValue:  7,
		},
		{
			name:       "user values preserved when overriding different key",
			userValues: map[string]interface{}{"replicaCount": 4, "image": map[string]interface{}{"tag": "v5.0.0"}},
			wantKey:    "replicaCount",
			wantValue:  4,
		},
		{
			name:       "multiple nested user overrides",
			userValues: map[string]interface{}{"image": map[string]interface{}{"tag": "v6.0.0"}, "service": map[string]interface{}{"port": 443}},
			wantKey:    "service.port",
			wantValue:  443,
		},
		{
			name:       "empty user values returns defaults",
			userValues: map[string]interface{}{},
			wantKey:    "replicaCount",
			wantValue:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := tt.userValues
			if values == nil {
				values = map[string]interface{}{}
			}

			merged, err := util.CoalesceValues(chrt, values)
			if err != nil {
				t.Fatalf("CoalesceValues error: %v", err)
			}

			got := getNestedValue(merged, tt.wantKey)
			if got != tt.wantValue {
				t.Fatalf("key %q = %v (%T), want %v (%T)", tt.wantKey, got, got, tt.wantValue, tt.wantValue)
			}
		})
	}
}

func getNestedValue(values map[string]interface{}, key string) interface{} {
	parts := splitKey(key)
	var current interface{} = values
	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current, ok = m[part]
		if !ok {
			return nil
		}
	}
	return current
}

func splitKey(key string) []string {
	var parts []string
	current := ""
	for _, c := range key {
		if c == '.' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
