# API Key Management

API key management APIs are under `/api/v1/admin/apikeys/`.

These endpoints require an authenticated admin user, or an API key with the `admin` role.

## List API keys

```http
GET /api/v1/admin/apikeys/
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
  https://kite.example.com/api/v1/admin/apikeys/?page=1&size=20
```

Response example:

```json
{
  "data": [
    {
      "id": 5,
      "username": "kite5-ci-bot",
      "provider": "apikey",
      "apiKey": "kite5-abc123def456",
      "roles": [
        {
          "id": 2,
          "name": "viewer"
        }
      ],
      "owner": null,
      "createdAt": "2026-01-15T10:30:00Z",
      "updatedAt": "2026-01-15T10:30:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "size": 20
}
```

## List independent API keys

Returns API keys without an owner (manually created). Used by the RBAC assignment dialog.

```http
GET /api/v1/admin/apikeys/independent
```

Example:

```bash
curl \
  -H "Authorization: kite1-adminsecret" \
  https://kite.example.com/api/v1/admin/apikeys/independent
```

Response example:

```json
{
  "apiKeys": [
    {
      "id": 5,
      "username": "kite5-ci-bot",
      "provider": "apikey"
    }
  ]
}
```

## Create an API key

```http
POST /api/v1/admin/apikeys/
Content-Type: application/json
```

Request body:

```json
{
  "name": "ci-bot"
}
```

Example:

```bash
curl \
  -X POST \
  -H "Authorization: kite1-adminsecret" \
  -H "Content-Type: application/json" \
  -d '{"name":"ci-bot"}' \
  https://kite.example.com/api/v1/admin/apikeys/
```

Response example:

```json
{
  "apiKey": {
    "id": 6,
    "username": "kite6-ci-bot",
    "provider": "apikey",
    "apiKey": "kite6-xyz789ghi012",
    "createdAt": "2026-01-15T11:00:00Z",
    "updatedAt": "2026-01-15T11:00:00Z"
  }
}
```

## Delete an API key

```http
DELETE /api/v1/admin/apikeys/:id
```

Example:

```bash
curl \
  -X DELETE \
  -H "Authorization: kite1-adminsecret" \
  https://kite.example.com/api/v1/admin/apikeys/6
```

Response example:

```json
{
  "message": "API key deleted successfully"
}
```
