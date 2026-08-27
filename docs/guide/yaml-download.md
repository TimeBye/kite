# YAML Download

Kite allows you to download Kubernetes resource definitions as YAML files, either individually or in bulk. You can choose between raw YAML (as returned by the cluster) or neat YAML (with Kubernetes-managed fields and default values removed).

## Feature Overview

- **Single download**: On any resource detail page, click the **Download** button in the header to download that resource as a `.yaml` file.
- **Batch download**: On any resource list page, select one or more rows via checkboxes, then click the **Download** button in the batch actions bar. Multiple resources are packaged into a `.zip` archive, with each resource as a separate `.yaml` file.
- **Two download modes**:
  - **Raw YAML**: Preserves all fields returned by the Kubernetes API (except `managedFields` and the `kubectl.kubernetes.io/last-applied-configuration` annotation, which are always removed).
  - **Neat YAML**: Additionally removes Kubernetes-managed fields, default values, and other noise to produce a clean, portable manifest.

## Supported Resource Types

YAML download works with all resource types in Kite:

| Resource Type | Scope | Notes |
|---------------|-------|-------|
| Built-in resources (Pods, Deployments, Services, ConfigMaps, etc.) | Namespace / Cluster | All built-in Kubernetes resources are supported |
| Versioned resources (Ingresses, CronJobs, HPAs, etc.) | Namespace / Cluster | Resources with multiple API versions are automatically resolved |
| Custom Resource Definitions (CRDs) | Namespace / Cluster | Custom resources are downloaded as-is via unstructured retrieval |
| Helm Releases | Namespace | Helm releases are serialized as pseudo-Kubernetes objects with `kind: HelmRelease` |

For cluster-scoped resources (Nodes, Namespaces, ClusterRoles, etc.), the namespace is omitted from the download filename. For namespace-scoped resources, the filename includes the namespace (e.g., `Pod-default-nginx.yaml`).

## Neat Mode

Neat mode cleans up the YAML by removing fields that are automatically populated by Kubernetes and are not useful when exporting a resource definition. This is similar to the [kubectl-neat](https://github.com/itaysk/kubectl-neat) tool.

### Fields removed in neat mode

| Field | Description |
|-------|-------------|
| `metadata.managedFields` | Server-side field tracking (always removed) |
| `metadata.annotations["kubectl.kubernetes.io/last-applied-configuration"]` | Last-applied config annotation (always removed) |
| `metadata.creationTimestamp` | When the resource was created |
| `metadata.generation` | Resource generation counter |
| `metadata.resourceVersion` | Server-side version tracking |
| `metadata.uid` | Unique identifier assigned by the cluster |
| `metadata.selfLink` | API path to the resource |
| `metadata.ownerReferences` | Commented out (preserved as YAML comments, not deleted) |
| `status` | Entire status section |
| Empty `annotations` / `labels` | Removed if empty after cleanup |
| OpenAPI default values | Fields whose values match the Kubernetes OpenAPI schema defaults |

### OpenAPI Schema Caching

Neat mode uses the cluster's OpenAPI v2 schema to identify and remove default values (e.g., `spec.restartPolicy: Always` for Pods). The schema is fetched and cached when a cluster connection is established, and is reused for all subsequent neat downloads. The cache is automatically refreshed when the cluster is reconnected.

If the schema cannot be fetched (e.g., due to network issues), neat mode falls back to field-level cleanup only, without default value removal.

### Example

A Pod created with the following YAML:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: nginx
  namespace: default
spec:
  containers:
  - name: nginx
    image: nginx:1.27
```

After being applied to a cluster, Kubernetes adds many fields (`status`, `metadata.uid`, `metadata.resourceVersion`, default `spec.restartPolicy`, etc.). Downloading as **Raw YAML** would include all of these. Downloading as **Neat YAML** strips them back out, producing a clean manifest close to the original.
