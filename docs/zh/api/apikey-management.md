# API 密钥管理

API 密钥管理接口位于 `/api/v1/admin/apikeys/`。

这些接口要求调用方已经是管理员用户，或者使用一个拥有 `admin` 角色的 API 密钥。

## 获取 API 密钥列表

```http
GET /api/v1/admin/apikeys/
```

查询参数：

| 参数   | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `page` | int  | 1      | 页码（从 1 开始） |
| `size` | int  | 20     | 每页条数 |

示例：

```bash
curl \
  -H "Authorization: kite1-adminsecret" \
  https://kite.example.com/api/v1/admin/apikeys/?page=1&size=20
```

响应示例：

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

## 获取独立 API 密钥列表

返回没有 owner 的 API 密钥（手动创建的），用于 RBAC 分配对话框。

```http
GET /api/v1/admin/apikeys/independent
```

示例：

```bash
curl \
  -H "Authorization: kite1-adminsecret" \
  https://kite.example.com/api/v1/admin/apikeys/independent
```

响应示例：

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

## 创建 API 密钥

```http
POST /api/v1/admin/apikeys/
Content-Type: application/json
```

请求体：

```json
{
  "name": "ci-bot"
}
```

示例：

```bash
curl \
  -X POST \
  -H "Authorization: kite1-adminsecret" \
  -H "Content-Type: application/json" \
  -d '{"name":"ci-bot"}' \
  https://kite.example.com/api/v1/admin/apikeys/
```

响应示例：

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

## 删除 API 密钥

```http
DELETE /api/v1/admin/apikeys/:id
```

示例：

```bash
curl \
  -X DELETE \
  -H "Authorization: kite1-adminsecret" \
  https://kite.example.com/api/v1/admin/apikeys/6
```

响应示例：

```json
{
  "message": "API key deleted successfully"
}
```
