package resources

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zxh326/kite/pkg/cluster"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func makeTestPodWithDefaults() *corev1.Pod {
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              "nginx",
			Namespace:         "default",
			UID:               "abc-123",
			ResourceVersion:   "12345",
			Generation:        1,
			CreationTimestamp: metav1.Time{},
			ManagedFields: []metav1.ManagedFieldsEntry{
				{
					Manager:    "kubectl-create",
					Operation:  "Update",
					APIVersion: "v1",
				},
			},
			Annotations: map[string]string{
				"kubectl.kubernetes.io/last-applied-configuration": `{"apiVersion":"v1","kind":"Pod"}`,
			},
			Labels: map[string]string{},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       "nginx-rs",
					UID:        "rs-uid-123",
					Controller: boolPtr(true),
				},
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "nginx",
					Image:           "nginx:1.27",
					ImagePullPolicy: corev1.PullIfNotPresent,
				},
			},
			RestartPolicy: corev1.RestartPolicyAlways,
			DNSPolicy:     corev1.DNSClusterFirst,
			SchedulerName: "default-scheduler",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func TestNeatYAML_RawMode(t *testing.T) {
	pod := makeTestPodWithDefaults()

	yamlStr, err := neatYAML(pod, nil, false)
	require.NoError(t, err)

	// Raw mode: managedFields and kubectl annotation should be removed
	assert.NotContains(t, yamlStr, "managedFields")
	assert.NotContains(t, yamlStr, "kubectl.kubernetes.io/last-applied-configuration")

	// Raw mode: system fields should be preserved
	assert.Contains(t, yamlStr, "resourceVersion")
	assert.Contains(t, yamlStr, "uid: abc-123")
	assert.Contains(t, yamlStr, "generation:")

	// Raw mode: status should be preserved
	assert.Contains(t, yamlStr, "status:")

	// Raw mode: ownerReferences should be preserved (not commented)
	assert.Contains(t, yamlStr, "ownerReferences:")
	assert.NotContains(t, yamlStr, "# ownerReferences:")
}

func TestNeatYAML_NeatMode(t *testing.T) {
	pod := makeTestPodWithDefaults()

	yamlStr, err := neatYAML(pod, nil, true)
	require.NoError(t, err)

	// Neat mode: system metadata fields should be removed
	assert.NotContains(t, yamlStr, "creationTimestamp")
	assert.NotContains(t, yamlStr, "resourceVersion")
	// uid: might appear in commented ownerReferences, so check for the metadata uid field
	assert.NotContains(t, yamlStr, "\n  uid:")
	assert.NotContains(t, yamlStr, "generation:")
	assert.NotContains(t, yamlStr, "selfLink")

	// Neat mode: managedFields and kubectl annotation should be removed
	assert.NotContains(t, yamlStr, "managedFields")
	assert.NotContains(t, yamlStr, "kubectl.kubernetes.io/last-applied-configuration")

	// Neat mode: status should be removed
	assert.NotContains(t, yamlStr, "status:")

	// Neat mode: ownerReferences should be commented out
	assert.Contains(t, yamlStr, "# ownerReferences:")
	lines := strings.Split(yamlStr, "\n")
	foundCommentedRef := false
	for _, line := range lines {
		if strings.Contains(line, "# ownerReferences:") {
			foundCommentedRef = true
			break
		}
	}
	assert.True(t, foundCommentedRef, "ownerReferences should be present as a comment")

	// The ownerReferences content should be in comments, not active YAML
	assert.Contains(t, yamlStr, "# - apiVersion: apps/v1")
	assert.Contains(t, yamlStr, "#   kind: ReplicaSet")
	assert.Contains(t, yamlStr, "#   name: nginx-rs")

	// Neat mode: empty labels should be removed
	assert.NotContains(t, yamlStr, "labels: {}")
}

func TestNeatYAML_NeatMode_NoOwnerReferences(t *testing.T) {
	// A top-level resource like Deployment has no ownerReferences
	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              "nginx",
			Namespace:         "default",
			UID:               "abc-123",
			ResourceVersion:   "12345",
			CreationTimestamp: metav1.Time{},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
	}

	yamlStr, err := neatYAML(deploy, nil, true)
	require.NoError(t, err)

	// No ownerReferences at all, so no comment either
	assert.NotContains(t, yamlStr, "ownerReferences")
	assert.NotContains(t, yamlStr, "# ownerReferences")

	// System fields should still be removed
	assert.NotContains(t, yamlStr, "creationTimestamp")
	assert.NotContains(t, yamlStr, "resourceVersion")
	assert.NotContains(t, yamlStr, "uid:")
}

func TestNeatYAML_WithOpenAPIDefaults(t *testing.T) {
	pod := makeTestPodWithDefaults()

	cs := &cluster.ClientSet{
		OpenAPIDefaults: cluster.NewOpenAPIDefaultCache(map[string]map[string]interface{}{
			"io.k8s.api.core.v1.Pod": {
				"spec.restartPolicy":              "Always",
				"spec.dnsPolicy":                  "ClusterFirst",
				"spec.schedulerName":              "default-scheduler",
				"spec.containers.imagePullPolicy": "IfNotPresent",
			},
		}),
	}

	yamlStr, err := neatYAML(pod, cs, true)
	require.NoError(t, err)

	// Default values should be removed
	assert.NotContains(t, yamlStr, "restartPolicy: Always")
	assert.NotContains(t, yamlStr, "dnsPolicy: ClusterFirst")
	assert.NotContains(t, yamlStr, "schedulerName: default-scheduler")
	assert.NotContains(t, yamlStr, "imagePullPolicy: IfNotPresent")

	// Non-default values should be preserved
	assert.Contains(t, yamlStr, "nginx:1.27")
}

func TestNeatYAML_WithOpenAPIDefaults_NonDefaultValuePreserved(t *testing.T) {
	pod := makeTestPodWithDefaults()
	// Set a non-default restart policy
	pod.Spec.RestartPolicy = corev1.RestartPolicyNever

	cs := &cluster.ClientSet{
		OpenAPIDefaults: cluster.NewOpenAPIDefaultCache(map[string]map[string]interface{}{
			"io.k8s.api.core.v1.Pod": {
				"spec.restartPolicy": "Always",
			},
		}),
	}

	yamlStr, err := neatYAML(pod, cs, true)
	require.NoError(t, err)

	// Non-default value should be preserved
	assert.Contains(t, yamlStr, "restartPolicy: Never")
}

func TestNeatYAML_NilOpenAPIDefaults(t *testing.T) {
	pod := makeTestPodWithDefaults()

	// With nil OpenAPIDefaults, should still work (just skip default removal)
	yamlStr, err := neatYAML(pod, nil, true)
	require.NoError(t, err)

	// Neat mode fields should still be cleaned
	assert.NotContains(t, yamlStr, "creationTimestamp")
	assert.NotContains(t, yamlStr, "status:")
}

func TestNeatYAML_ClusterScopedResource(t *testing.T) {
	node := &corev1.Node{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "node-1",
			UID:             "node-uid",
			ResourceVersion: "999",
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	yamlStr, err := neatYAML(node, nil, true)
	require.NoError(t, err)

	assert.NotContains(t, yamlStr, "status:")
	assert.NotContains(t, yamlStr, "resourceVersion")
	assert.NotContains(t, yamlStr, "uid:")
	assert.Contains(t, yamlStr, "name: node-1")
}

func TestNeatYAML_EmptyAnnotationsAndLabels(t *testing.T) {
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				"kubectl.kubernetes.io/last-applied-configuration": "{}",
			},
			Labels: map[string]string{},
		},
	}

	yamlStr, err := neatYAML(pod, nil, true)
	require.NoError(t, err)

	// After removing kubectl annotation, annotations map is empty -> should be removed
	assert.NotContains(t, yamlStr, "annotations:")
	assert.NotContains(t, yamlStr, "annotations: {}")
	// Empty labels should also be removed
	assert.NotContains(t, yamlStr, "labels: {}")
}

func TestNeatYAML_PreservesNonEmptyAnnotationsAndLabels(t *testing.T) {
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				"custom.annotation/example": "value",
			},
			Labels: map[string]string{
				"app": "test",
			},
		},
	}

	yamlStr, err := neatYAML(pod, nil, true)
	require.NoError(t, err)

	assert.Contains(t, yamlStr, "custom.annotation/example: value")
	assert.Contains(t, yamlStr, "app: test")
}

func TestObjectToUnstructured(t *testing.T) {
	t.Run("from typed object", func(t *testing.T) {
		pod := &corev1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}
		u, err := objectToUnstructured(pod)
		require.NoError(t, err)
		assert.Equal(t, "Pod", u.GetKind())
		assert.Equal(t, "test", u.GetName())
	})

	t.Run("from unstructured (deep copy)", func(t *testing.T) {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"})
		u.SetName("test")

		u2, err := objectToUnstructured(u)
		require.NoError(t, err)
		assert.Equal(t, "test", u2.GetName())
		// Verify it's a copy
		u2.SetName("modified")
		assert.Equal(t, "test", u.GetName(), "original should be unmodified")
	})
}

func TestGetGVKFromObject(t *testing.T) {
	t.Run("from unstructured", func(t *testing.T) {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
		gvk := getGVKFromObject(u)
		assert.Equal(t, "Deployment", gvk.Kind)
		assert.Equal(t, "apps", gvk.Group)
		assert.Equal(t, "v1", gvk.Version)
	})
}

func TestCommentOutOwnerReferences(t *testing.T) {
	t.Run("with ownerReferences", func(t *testing.T) {
		yamlStr := `apiVersion: v1
kind: Pod
metadata:
  name: nginx
  namespace: default
  ownerReferences:
  - apiVersion: apps/v1
    kind: ReplicaSet
    name: nginx-rs
    uid: rs-uid
  resourceVersion: "123"
spec:
  containers:
  - name: nginx
`
		ownerRefs := []metav1.OwnerReference{
			{
				APIVersion: "apps/v1",
				Kind:       "ReplicaSet",
				Name:       "nginx-rs",
				UID:        "rs-uid",
			},
		}

		result := commentOutOwnerReferences(yamlStr, ownerRefs)

		// ownerReferences should be commented
		assert.Contains(t, result, "# ownerReferences:")
		assert.Contains(t, result, "# - apiVersion: apps/v1")
		assert.Contains(t, result, "#   kind: ReplicaSet")
		assert.Contains(t, result, "#   name: nginx-rs")

		// Other fields should be preserved
		assert.Contains(t, result, "name: nginx")
		assert.Contains(t, result, "resourceVersion: \"123\"")
		assert.Contains(t, result, "spec:")
	})

	t.Run("without metadata section", func(t *testing.T) {
		yamlStr := `apiVersion: v1
kind: ConfigMap
data:
  key: value
`
		ownerRefs := []metav1.OwnerReference{
			{Kind: "Deployment", Name: "test"},
		}

		result := commentOutOwnerReferences(yamlStr, ownerRefs)
		// Should return original if no metadata section
		assert.Equal(t, yamlStr, result)
	})
}

func int32Ptr(i int32) *int32 {
	return &i
}
