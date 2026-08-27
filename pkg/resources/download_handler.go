package resources

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zxh326/kite/pkg/cluster"
	"github.com/zxh326/kite/pkg/common"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
)

// downloadItem represents a resource to download.
type downloadItem struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// getKindFromObject extracts the Kind from a runtime object.
func getKindFromObject(obj interface{}) string {
	if robj, ok := obj.(runtime.Object); ok {
		gvk := getGVKFromObject(robj)
		return gvk.Kind
	}
	// Try from map
	if m, ok := obj.(map[string]interface{}); ok {
		if kind, ok := m["kind"].(string); ok {
			return kind
		}
	}
	return ""
}

// buildYAMLFileName generates a filename for a YAML download.
func buildYAMLFileName(kind, namespace, name string) string {
	if namespace != "" && namespace != common.AllNamespaces {
		return fmt.Sprintf("%s-%s-%s.yaml", kind, namespace, name)
	}
	return fmt.Sprintf("%s-%s.yaml", kind, name)
}

// DownloadSingle handles downloading a single resource as YAML.
func DownloadSingle(c *gin.Context) {
	resource := c.Param("resource")
	if resource == "" {
		// Try CRD param
		resource = c.Param("crd")
	}
	if resource == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "resource type is required"})
		return
	}

	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "resource name is required"})
		return
	}

	namespace := c.Param("namespace")
	neat := c.Query("neat") == "true"

	cs := c.MustGet("cluster").(*cluster.ClientSet)

	obj, err := GetResource(c, resource, namespace, name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	kind := getKindFromObject(obj)
	if kind == "" {
		// Try to get kind from resource registry
		if meta := common.LookupResource(resource); meta != nil {
			kind = meta.Kind
		}
	}
	if kind == "" {
		kind = resource
	}

	yamlContent, err := neatYAML(obj, cs, neat)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to generate YAML: %v", err)})
		return
	}

	fileName := buildYAMLFileName(kind, namespace, name)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Data(http.StatusOK, "application/yaml", []byte(yamlContent))
}

// DownloadBatch handles downloading multiple resources as a ZIP file.
func DownloadBatch(c *gin.Context) {
	resource := c.Param("resource")
	if resource == "" {
		resource = c.Param("crd")
	}
	if resource == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "resource type is required"})
		return
	}

	neat := c.Query("neat") == "true"

	var items []downloadItem
	if err := c.ShouldBindJSON(&items); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}
	if len(items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no items specified"})
		return
	}

	cs := c.MustGet("cluster").(*cluster.ClientSet)

	// Get the kind from registry or first fetched object
	kind := ""
	if meta := common.LookupResource(resource); meta != nil {
		kind = meta.Kind
	}

	// Create a buffer for the zip file
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	var failedItems []string
	for _, item := range items {
		obj, err := GetResource(c, resource, item.Namespace, item.Name)
		if err != nil {
			klog.Warningf("Failed to get resource %s/%s/%s for download: %v", resource, item.Namespace, item.Name, err)
			failedItems = append(failedItems, fmt.Sprintf("%s/%s", item.Namespace, item.Name))
			continue
		}

		// Update kind from the actual object if we don't have it yet
		if kind == "" {
			kind = getKindFromObject(obj)
		}

		yamlContent, err := neatYAML(obj, cs, neat)
		if err != nil {
			klog.Warningf("Failed to generate YAML for %s/%s: %v", item.Namespace, item.Name, err)
			failedItems = append(failedItems, fmt.Sprintf("%s/%s", item.Namespace, item.Name))
			continue
		}

		fileName := buildYAMLFileName(kind, item.Namespace, item.Name)
		w, err := zipWriter.Create(fileName)
		if err != nil {
			klog.Warningf("Failed to create zip entry for %s: %v", fileName, err)
			failedItems = append(failedItems, fmt.Sprintf("%s/%s", item.Namespace, item.Name))
			continue
		}
		if _, err := w.Write([]byte(yamlContent)); err != nil {
			klog.Warningf("Failed to write zip entry for %s: %v", fileName, err)
			failedItems = append(failedItems, fmt.Sprintf("%s/%s", item.Namespace, item.Name))
			continue
		}
	}

	// Add a summary file if there were failures
	if len(failedItems) > 0 {
		summary := fmt.Sprintf("Failed to download %d resource(s):\n%s\n", len(failedItems), strings.Join(failedItems, "\n"))
		w, err := zipWriter.Create("_download_errors.txt")
		if err == nil {
			_, _ = w.Write([]byte(summary))
		}
	}

	if err := zipWriter.Close(); err != nil {
		klog.Warningf("Failed to close zip writer: %v", err)
	}

	zipFileName := fmt.Sprintf("%s-%s.zip", resource, time.Now().Format("20060102-150405"))
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, zipFileName))
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Data(http.StatusOK, "application/zip", buf.Bytes())
}
