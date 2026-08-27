package resources

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zxh326/kite/pkg/cluster"
	"github.com/zxh326/kite/pkg/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

// neatMetadataFields are the metadata fields to remove in neat mode.
var neatMetadataFields = []string{
	"creationTimestamp",
	"generation",
	"resourceVersion",
	"uid",
	"selfLink",
}

// objectToUnstructured converts any runtime object to unstructured.Unstructured.
func objectToUnstructured(obj interface{}) (*unstructured.Unstructured, error) {
	// Fast path: already unstructured
	if u, ok := obj.(*unstructured.Unstructured); ok {
		return u.DeepCopy(), nil
	}

	data, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal object: %w", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to unmarshal object: %w", err)
	}
	return &unstructured.Unstructured{Object: m}, nil
}

// getGVKFromObject extracts the GroupVersionKind from a runtime.Object.
func getGVKFromObject(obj runtime.Object) schema.GroupVersionKind {
	if u, ok := obj.(*unstructured.Unstructured); ok {
		return u.GroupVersionKind()
	}
	gvks, _, err := kube.GetScheme().ObjectKinds(obj)
	if err != nil || len(gvks) == 0 {
		return schema.GroupVersionKind{}
	}
	return gvks[0]
}

// neatYAML converts a runtime.Object to a neatened YAML string.
// If neat is false, only managedFields and kubectl annotation are removed.
// If neat is true, additional fields are removed and ownerReferences are commented out.
func neatYAML(obj interface{}, cs *cluster.ClientSet, neat bool) (string, error) {
	u, err := objectToUnstructured(obj)
	if err != nil {
		return "", err
	}

	// Always clean managedFields and kubectl annotation
	u.SetManagedFields(nil)
	annotations := u.GetAnnotations()
	if annotations != nil {
		delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
		if len(annotations) == 0 {
			annotations = nil
		}
		u.SetAnnotations(annotations)
	}

	if !neat {
		yamlBytes, err := yaml.Marshal(u)
		return string(yamlBytes), err
	}

	// Neat mode: extract ownerReferences before removing (for comment preservation)
	ownerRefs := u.GetOwnerReferences()

	// Remove neat metadata fields
	metadata, ok := u.Object["metadata"].(map[string]interface{})
	if ok {
		for _, field := range neatMetadataFields {
			delete(metadata, field)
		}
		delete(metadata, "ownerReferences")
	}
	// Remove status
	delete(u.Object, "status")
	// Clean empty annotations and labels
	if metadata != nil {
		if ann, ok := metadata["annotations"].(map[string]interface{}); ok && len(ann) == 0 {
			delete(metadata, "annotations")
		}
		if labels, ok := metadata["labels"].(map[string]interface{}); ok && len(labels) == 0 {
			delete(metadata, "labels")
		}
	}

	// Remove OpenAPI default values
	removeDefaultValues(u, cs)

	// Marshal to YAML
	yamlBytes, err := yaml.Marshal(u)
	if err != nil {
		return "", err
	}

	yamlStr := string(yamlBytes)

	// If there were ownerReferences, add them as comments
	if len(ownerRefs) > 0 {
		yamlStr = commentOutOwnerReferences(yamlStr, ownerRefs)
	}

	return yamlStr, nil
}

// removeDefaultValues recursively removes fields that match OpenAPI schema defaults.
func removeDefaultValues(u *unstructured.Unstructured, cs *cluster.ClientSet) {
	if cs == nil || cs.OpenAPIDefaults == nil {
		return
	}

	gvk := u.GroupVersionKind()
	if gvk.Kind == "" {
		return
	}

	defName := cluster.GVKToDefinitionName(gvk.Group, gvk.Version, gvk.Kind)
	defDefaults := cs.OpenAPIDefaults.GetDefaultsForGVK(defName)
	if len(defDefaults) == 0 {
		return
	}

	removeDefaultFields(u.Object, defDefaults, "")
}

// removeDefaultFields recursively walks the object and removes fields matching default values.
func removeDefaultFields(obj map[string]interface{}, defaults map[string]interface{}, prefix string) {
	for key, val := range obj {
		fieldPath := key
		if prefix != "" {
			fieldPath = prefix + "." + key
		}

		switch v := val.(type) {
		case map[string]interface{}:
			// Recurse into nested objects
			removeDefaultFields(v, defaults, fieldPath)
			// Remove empty maps after cleanup
			if len(v) == 0 && key != "metadata" {
				delete(obj, key)
			}
		case []interface{}:
			// For arrays, check each element
			for i, elem := range v {
				if elemMap, ok := elem.(map[string]interface{}); ok {
					removeDefaultFields(elemMap, defaults, fieldPath)
					// Remove empty maps in arrays
					if len(elemMap) == 0 {
						v[i] = nil
					}
				}
			}
			// Filter out nil elements
			var filtered []interface{}
			for _, elem := range v {
				if elem != nil {
					filtered = append(filtered, elem)
				}
			}
			if len(filtered) == 0 {
				delete(obj, key)
			} else {
				obj[key] = filtered
			}
		default:
			// Check if this field has a default value and matches
			if defVal, exists := defaults[fieldPath]; exists {
				if fmt.Sprintf("%v", val) == fmt.Sprintf("%v", defVal) {
					delete(obj, key)
				}
			}
		}
	}
}

// commentOutOwnerReferences takes a YAML string and adds ownerReferences
// as comments in the metadata section.
func commentOutOwnerReferences(yamlStr string, ownerRefs []metav1.OwnerReference) string {
	// First, serialize the ownerReferences to YAML
	refMap := map[string]interface{}{
		"ownerReferences": ownerRefs,
	}
	refYAML, err := yaml.Marshal(refMap)
	if err != nil {
		return yamlStr
	}

	lines := strings.Split(yamlStr, "\n")
	refLines := strings.Split(string(refYAML), "\n")

	// Find the metadata section and its indentation
	metadataIdx := -1
	metadataIndent := ""
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " ")
		if trimmed == "metadata:" {
			metadataIdx = i
			metadataIndent = line[:len(line)-len(trimmed)]
			break
		}
	}

	if metadataIdx == -1 {
		return yamlStr
	}

	// Find the end of the metadata section
	metadataEnd := len(lines)
	for i := metadataIdx + 1; i < len(lines); i++ {
		line := lines[i]
		if line == "" {
			continue
		}
		trimmed := strings.TrimLeft(line, " ")
		if trimmed == line { // No indentation, new top-level section
			metadataEnd = i
			break
		}
		// Check if indentation is same or less than metadata (new section at same level)
		indent := line[:len(line)-len(trimmed)]
		if len(indent) <= len(metadataIndent) && !strings.HasPrefix(line, metadataIndent+"  ") {
			metadataEnd = i
			break
		}
	}

	// Build comment lines from the refYAML
	var comments []string
	for _, line := range refLines {
		if line == "" {
			continue
		}
		// Comment out each line with the metadata field indentation (2 spaces under metadata:)
		comments = append(comments, metadataIndent+"  # "+line)
	}

	// Insert comments at the end of the metadata section
	result := make([]string, 0, len(lines)+len(comments))
	result = append(result, lines[:metadataEnd]...)
	result = append(result, comments...)
	result = append(result, lines[metadataEnd:]...)

	return strings.Join(result, "\n")
}
