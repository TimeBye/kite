# RBAC Management

RBAC management APIs are under `/api/v1/admin/roles/`.

These endpoints require an authenticated admin user, or an API key with the `admin` role.

## List roles

```http
GET /api/v1/admin/roles/
```

Query parameters:

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `page`    | int  | 1       | Page number (1-based) |
| `size`    | int  | 20      | Page size |

Example:

```bash
curl \
  -H "Authorization: kite1-adminsecret" \
  https://kite.example.com/api/v1/admin/roles/?page=1&size=20
```

Response example:

```json
{
  "data": [
    {
      "id": 2,
      "name": "viewer",
      "description": "Viewer role with read-only access",
      "isSystem": true,
      "clusters": ["*"],
      "namespaces": ["*"],
      "resources": ["*"],
      "verbs": ["get", "log"],
      "assignments": []
    }
  ],
  "total": 1,
  "page": 1,
  "size": 20
}
```

## Get a single role

```http
GET /api/v1/admin/roles/:id
```

## Create a role

```http
POST /api/v1/admin/roles/
Content-Type: application/json
```

Request body:

```json
{
  "name": "namespace-editor",
  "description": "Edit workloads in dev",
  "clusters": ["demo-cluster"],
  "namespaces": ["dev"],
  "resources": ["deployments", "pods", "services"],
  "verbs": ["get", "create", "update", "delete", "log"]
}
```

Example:

```bash
curl \
  -X POST \
  -H "Authorization: kite1-adminsecret" \
  -H "Content-Type: application/json" \
  -d '{
    "name":"namespace-editor",
    "description":"Edit workloads in dev",
    "clusters":["demo-cluster"],
    "namespaces":["dev"],
    "resources":["deployments","pods","services"],
    "verbs":["get","create","update","delete","log"]
  }' \
  https://kite.example.com/api/v1/admin/roles/
```

## Update a role

```http
PUT /api/v1/admin/roles/:id
Content-Type: application/json
```

Request body uses the same fields as create.

## Delete a role

```http
DELETE /api/v1/admin/roles/:id
```

Example:

```bash
curl \
  -X DELETE \
  -H "Authorization: kite1-adminsecret" \
  https://kite.example.com/api/v1/admin/roles/3
```

Response example:

```json
{
  "success": true
}
```

## Assign a role

```http
POST /api/v1/admin/roles/:id/assign
Content-Type: application/json
```

Request body:

```json
{
  "subjectType": "user",
  "subject": "alice"
}
```

Supported `subjectType` values:

- `user` — a regular user or service account
- `group` — an OIDC group
- `apikey` — a standalone API key (stored as `user` internally). Only API keys without an owner (manually created) can be assigned roles. Kubeconfig-generated API keys inherit roles dynamically from their owner and cannot be assigned roles directly.

Example:

```bash
curl \
  -X POST \
  -H "Authorization: kite1-adminsecret" \
  -H "Content-Type: application/json" \
  -d '{"subjectType":"group","subject":"platform-team"}' \
  https://kite.example.com/api/v1/admin/roles/3/assign
```

## Unassign a role

```http
DELETE /api/v1/admin/roles/:id/assign?subjectType=user&subject=alice
```

Example:

```bash
curl \
  -X DELETE \
  -H "Authorization: kite1-adminsecret" \
  "https://kite.example.com/api/v1/admin/roles/3/assign?subjectType=group&subject=platform-team"
```
