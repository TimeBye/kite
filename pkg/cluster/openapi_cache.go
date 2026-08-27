package cluster

import (
	"context"
	"strings"
	"time"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"
	"k8s.io/klog/v2"
)

// OpenAPIDefaultCache stores extracted default values from the OpenAPI v2 schema.
// It maps a GVK definition name (e.g. "io.k8s.api.core.v1.Pod") to a map of
// field paths to their default values.
type OpenAPIDefaultCache struct {
	// defaults maps definitionName -> fieldPath -> defaultValue
	defaults map[string]map[string]interface{}
}

// NewOpenAPIDefaultCache creates a cache from a pre-built defaults map.
// This is primarily used for testing.
func NewOpenAPIDefaultCache(defaults map[string]map[string]interface{}) *OpenAPIDefaultCache {
	return &OpenAPIDefaultCache{defaults: defaults}
}

// GetDefaultsForGVK returns the default values map for a given GVK definition name.
// The definition name follows the OpenAPI convention: "io.k8s.api.core.v1.Pod"
func (c *OpenAPIDefaultCache) GetDefaultsForGVK(gvk string) map[string]interface{} {
	if c == nil {
		return nil
	}
	return c.defaults[gvk]
}

// GVKToDefinitionName converts a Group/Version/Kind to an OpenAPI definition name.
// e.g. Group="", Version="v1", Kind="Pod" -> "io.k8s.api.core.v1.Pod"
// e.g. Group="apps", Version="v1", Kind="Deployment" -> "io.k8s.api.apps.v1.Deployment"
// e.g. Group="networking.k8s.io", Version="v1", Kind="Ingress" -> "io.k8s.api.networking.k8s.io.v1.Ingress"
func GVKToDefinitionName(group, version, kind string) string {
	var parts []string
	parts = append(parts, "io", "k8s", "api")

	if group == "" {
		parts = append(parts, "core")
	} else {
		groupParts := strings.Split(group, ".")
		parts = append(parts, groupParts...)
	}

	parts = append(parts, version)
	parts = append(parts, kind)

	return strings.Join(parts, ".")
}

// fetchOpenAPIDefaults fetches and parses the OpenAPI v2 schema from the cluster,
// extracting only the default values for each definition. The raw schema document
// is discarded after extraction to minimize memory usage.
func fetchOpenAPIDefaults(cs *ClientSet) *OpenAPIDefaultCache {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_ = ctx // discovery client uses its own context internally

	doc, err := cs.K8sClient.ClientSet.Discovery().OpenAPISchema()
	if err != nil {
		klog.Warningf("Failed to fetch OpenAPI schema for cluster %s: %v", cs.Name, err)
		return nil
	}

	cache := extractDefaults(doc)
	klog.Infof("Extracted OpenAPI defaults for cluster %s: %d definitions", cs.Name, len(cache.defaults))
	return cache
}

// extractDefaults walks the OpenAPI v2 document and extracts default values
// for each definition. It returns a compact cache that only stores field paths
// and their default values, discarding the rest of the schema.
func extractDefaults(doc *openapi_v2.Document) *OpenAPIDefaultCache {
	cache := &OpenAPIDefaultCache{
		defaults: make(map[string]map[string]interface{}),
	}

	if doc == nil || doc.GetDefinitions() == nil {
		return cache
	}

	for _, defPair := range doc.GetDefinitions().GetAdditionalProperties() {
		defName := defPair.GetName()
		schema := defPair.GetValue()
		if schema == nil {
			continue
		}

		defDefaults := extractSchemaDefaults(schema, "")
		if len(defDefaults) > 0 {
			cache.defaults[defName] = defDefaults
		}
	}

	return cache
}

// extractSchemaDefaults recursively extracts default values from a schema.
// It returns a map of dot-separated field paths to their default values.
func extractSchemaDefaults(schema *openapi_v2.Schema, prefix string) map[string]interface{} {
	result := make(map[string]interface{})

	if schema == nil {
		return result
	}

	// Check for a default value at this level
	if schema.Default != nil {
		yamlStr := schema.Default.GetYaml()
		if yamlStr != "" {
			result[prefix] = yamlStr
		}
	}

	// Process properties
	if schema.GetProperties() != nil {
		for _, propPair := range schema.GetProperties().GetAdditionalProperties() {
			propName := propPair.GetName()
			propSchema := propPair.GetValue()
			if propSchema == nil {
				continue
			}

			fieldPath := propName
			if prefix != "" {
				fieldPath = prefix + "." + propName
			}

			if propSchema.Default != nil {
				yamlStr := propSchema.Default.GetYaml()
				if yamlStr != "" {
					result[fieldPath] = yamlStr
				}
			}

			nested := extractSchemaDefaults(propSchema, fieldPath)
			for k, v := range nested {
				if k != fieldPath {
					result[k] = v
				}
			}
		}
	}

	// Process items (array element schemas)
	if schema.GetItems() != nil {
		for _, itemSchema := range schema.GetItems().GetSchema() {
			if itemSchema == nil {
				continue
			}
			nested := extractSchemaDefaults(itemSchema, prefix)
			for k, v := range nested {
				result[k] = v
			}
		}
	}

	return result
}
