# Related Resources

When viewing a resource, Kite shows you a list of related resources. This helps you to quickly navigate between related objects, for example, from a Deployment to its Pods, or from a Service to its backing Pods.

![Related Resources](/screenshots/related.png)

Not all resource types support related resources. If a resource type does not have a related-resources route registered, the request gracefully returns an empty list instead of showing an error. This means the related resources panel simply shows "No related resources" for unsupported types, without any error state.
