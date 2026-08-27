package cluster

import (
	"testing"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"
	"github.com/stretchr/testify/assert"
)

func TestGVKToDefinitionName(t *testing.T) {
	tests := []struct {
		name    string
		group   string
		version string
		kind    string
		want    string
	}{
		{
			name:    "core resource (empty group)",
			group:   "",
			version: "v1",
			kind:    "Pod",
			want:    "io.k8s.api.core.v1.Pod",
		},
		{
			name:    "apps group",
			group:   "apps",
			version: "v1",
			kind:    "Deployment",
			want:    "io.k8s.api.apps.v1.Deployment",
		},
		{
			name:    "batch group",
			group:   "batch",
			version: "v1",
			kind:    "Job",
			want:    "io.k8s.api.batch.v1.Job",
		},
		{
			name:    "multi-dot group",
			group:   "flowcontrol.apiserver.k8s.io",
			version: "v1beta3",
			kind:    "FlowSchema",
			want:    "io.k8s.api.flowcontrol.apiserver.k8s.io.v1beta3.FlowSchema",
		},
		{
			name:    "networking group",
			group:   "networking.k8s.io",
			version: "v1",
			kind:    "Ingress",
			want:    "io.k8s.api.networking.k8s.io.v1.Ingress",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GVKToDefinitionName(tt.group, tt.version, tt.kind)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetDefaultsForGVK(t *testing.T) {
	t.Run("nil cache returns nil", func(t *testing.T) {
		var c *OpenAPIDefaultCache
		assert.Nil(t, c.GetDefaultsForGVK("anything"))
	})

	t.Run("existing key returns defaults", func(t *testing.T) {
		c := &OpenAPIDefaultCache{
			defaults: map[string]map[string]interface{}{
				"io.k8s.api.core.v1.Pod": {
					"spec.restartPolicy": "Always",
				},
			},
		}
		result := c.GetDefaultsForGVK("io.k8s.api.core.v1.Pod")
		assert.Equal(t, "Always", result["spec.restartPolicy"])
	})

	t.Run("non-existing key returns nil", func(t *testing.T) {
		c := &OpenAPIDefaultCache{
			defaults: map[string]map[string]interface{}{},
		}
		assert.Nil(t, c.GetDefaultsForGVK("nonexistent"))
	})
}

func TestExtractDefaults(t *testing.T) {
	t.Run("nil document returns empty cache", func(t *testing.T) {
		cache := extractDefaults(nil)
		assert.NotNil(t, cache)
		assert.Empty(t, cache.defaults)
	})

	t.Run("document with no definitions returns empty cache", func(t *testing.T) {
		doc := &openapi_v2.Document{}
		cache := extractDefaults(doc)
		assert.NotNil(t, cache)
		assert.Empty(t, cache.defaults)
	})

	t.Run("extracts defaults from definitions", func(t *testing.T) {
		// Build a minimal OpenAPI v2 document with a definition that has defaults
		doc := &openapi_v2.Document{
			Definitions: &openapi_v2.Definitions{
				AdditionalProperties: []*openapi_v2.NamedSchema{
					{
						Name: "io.k8s.api.core.v1.Pod",
						Value: &openapi_v2.Schema{
							Properties: &openapi_v2.Properties{
								AdditionalProperties: []*openapi_v2.NamedSchema{
									{
										Name: "spec",
										Value: &openapi_v2.Schema{
											Properties: &openapi_v2.Properties{
												AdditionalProperties: []*openapi_v2.NamedSchema{
													{
														Name: "restartPolicy",
														Value: &openapi_v2.Schema{
															Default: &openapi_v2.Any{
																Yaml: "Always",
															},
														},
													},
													{
														Name: "dnsPolicy",
														Value: &openapi_v2.Schema{
															Default: &openapi_v2.Any{
																Yaml: "ClusterFirst",
															},
														},
													},
													{
														Name: "containers",
														Value: &openapi_v2.Schema{
															Items: &openapi_v2.ItemsItem{
																Schema: []*openapi_v2.Schema{
																	{
																		Properties: &openapi_v2.Properties{
																			AdditionalProperties: []*openapi_v2.NamedSchema{
																				{
																					Name: "imagePullPolicy",
																					Value: &openapi_v2.Schema{
																						Default: &openapi_v2.Any{
																							Yaml: "IfNotPresent",
																						},
																					},
																				},
																			},
																		},
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		cache := extractDefaults(doc)

		podDefaults := cache.GetDefaultsForGVK("io.k8s.api.core.v1.Pod")
		assert.NotNil(t, podDefaults)
		assert.Equal(t, "Always", podDefaults["spec.restartPolicy"])
		assert.Equal(t, "ClusterFirst", podDefaults["spec.dnsPolicy"])
		// Items defaults should be extracted with the prefix of the array field
		assert.Equal(t, "IfNotPresent", podDefaults["spec.containers.imagePullPolicy"])
	})

	t.Run("skips definitions without defaults", func(t *testing.T) {
		doc := &openapi_v2.Document{
			Definitions: &openapi_v2.Definitions{
				AdditionalProperties: []*openapi_v2.NamedSchema{
					{
						Name: "io.k8s.api.core.v1.Node",
						Value: &openapi_v2.Schema{
							Properties: &openapi_v2.Properties{
								AdditionalProperties: []*openapi_v2.NamedSchema{
									{
										Name: "spec",
										Value: &openapi_v2.Schema{
											// No defaults
											Properties: &openapi_v2.Properties{
												AdditionalProperties: []*openapi_v2.NamedSchema{
													{
														Name:  "podCIDR",
														Value: &openapi_v2.Schema{
															// No default
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		cache := extractDefaults(doc)
		// Node has no defaults, should not be in the cache
		assert.Nil(t, cache.GetDefaultsForGVK("io.k8s.api.core.v1.Node"))
	})
}
