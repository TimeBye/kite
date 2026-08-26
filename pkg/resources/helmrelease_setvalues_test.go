package resources

import (
	"reflect"
	"testing"
)

func TestMergeSetValuesEmpty(t *testing.T) {
	result, err := mergeSetValues(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}

func TestMergeSetValuesNoSetValues(t *testing.T) {
	input := map[string]interface{}{"key": "value"}
	result, err := mergeSetValues(input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(result, input) {
		t.Fatalf("expected %v, got %v", input, result)
	}
}

func TestMergeSetValuesSimple(t *testing.T) {
	values := map[string]interface{}{"image": map[string]interface{}{"tag": "v1"}}
	setValues := []string{"image.tag=v2", "replicas=3"}
	result, err := mergeSetValues(values, setValues)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := map[string]interface{}{
		"image":    map[string]interface{}{"tag": "v2"},
		"replicas": int64(3),
	}
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("expected %v, got %v", expected, result)
	}
}

func TestMergeSetValuesWithNilValues(t *testing.T) {
	setValues := []string{"image.tag=v2"}
	result, err := mergeSetValues(nil, setValues)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := map[string]interface{}{
		"image": map[string]interface{}{"tag": "v2"},
	}
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("expected %v, got %v", expected, result)
	}
}

func TestMergeSetValuesInvalidFormat(t *testing.T) {
	values := map[string]interface{}{}
	// Non-numeric index produces a parse error
	_, err := mergeSetValues(values, []string{"key[abc]=v2"})
	if err == nil {
		t.Fatal("expected error for invalid set value, got nil")
	}
}

func TestMergeSetValuesIgnoresEmptyLines(t *testing.T) {
	values := map[string]interface{}{}
	setValues := []string{"", "  ", "key=value"}
	result, err := mergeSetValues(values, setValues)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := map[string]interface{}{
		"key": "value",
	}
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("expected %v, got %v", expected, result)
	}
}

func TestMergeSetValuesOverridesExisting(t *testing.T) {
	values := map[string]interface{}{
		"replicas": int64(1),
	}
	setValues := []string{"replicas=5"}
	result, err := mergeSetValues(values, setValues)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["replicas"] != int64(5) {
		t.Fatalf("expected replicas=5, got %v", result["replicas"])
	}
}

func TestMergeSetValuesListIndex(t *testing.T) {
	values := map[string]interface{}{}
	setValues := []string{"servers[0].port=8080", "servers[1].port=9090"}
	result, err := mergeSetValues(values, setValues)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	servers, ok := result["servers"].([]interface{})
	if !ok {
		t.Fatalf("expected servers to be a list, got %T", result["servers"])
	}
	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(servers))
	}
}

func TestMergeSetValuesDeepNested(t *testing.T) {
	values := map[string]interface{}{
		"config": map[string]interface{}{
			"database": map[string]interface{}{
				"host": "localhost",
			},
		},
	}
	setValues := []string{"config.database.port=5432", "config.database.host=remote"}
	result, err := mergeSetValues(values, setValues)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	db := result["config"].(map[string]interface{})["database"].(map[string]interface{})
	if db["host"] != "remote" {
		t.Fatalf("expected host=remote, got %v", db["host"])
	}
	if db["port"] != int64(5432) {
		t.Fatalf("expected port=5432, got %v", db["port"])
	}
}

func TestMergeSetValuesMultipleSetValues(t *testing.T) {
	values := map[string]interface{}{}
	setValues := []string{"a=1", "b=2", "c=3"}
	result, err := mergeSetValues(values, setValues)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["a"] != int64(1) || result["b"] != int64(2) || result["c"] != int64(3) {
		t.Fatalf("expected a=1, b=2, c=3, got %v", result)
	}
}

func TestMergeSetValuesPreservesExistingValues(t *testing.T) {
	values := map[string]interface{}{
		"image": map[string]interface{}{
			"repository": "nginx",
			"tag":        "v1",
		},
		"replicas": int64(2),
	}
	setValues := []string{"image.tag=v2"}
	result, err := mergeSetValues(values, setValues)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	image := result["image"].(map[string]interface{})
	if image["repository"] != "nginx" {
		t.Fatalf("expected repository=nginx, got %v", image["repository"])
	}
	if image["tag"] != "v2" {
		t.Fatalf("expected tag=v2, got %v", image["tag"])
	}
	if result["replicas"] != int64(2) {
		t.Fatalf("expected replicas=2, got %v", result["replicas"])
	}
}

func TestMergeSetValuesStringValues(t *testing.T) {
	values := map[string]interface{}{}
	setValues := []string{"name=my-release", "image=busybox:latest"}
	result, err := mergeSetValues(values, setValues)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["name"] != "my-release" {
		t.Fatalf("expected name=my-release, got %v", result["name"])
	}
	if result["image"] != "busybox:latest" {
		t.Fatalf("expected image=busybox:latest, got %v", result["image"])
	}
}

func TestMergeSetValuesTrimsWhitespace(t *testing.T) {
	values := map[string]interface{}{}
	setValues := []string{"  key=value  ", "  port=8080"}
	result, err := mergeSetValues(values, setValues)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["key"] != "value" {
		t.Fatalf("expected key=value, got %v", result["key"])
	}
	if result["port"] != int64(8080) {
		t.Fatalf("expected port=8080, got %v", result["port"])
	}
}
