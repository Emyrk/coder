# Anthropic

## List Anthropic agents the user can see

### Code samples

```shell
# Example request using curl
curl -X GET http://coder-server:8080/api/v2/anthropic/{organization}/agents/{user} \
  -H 'Accept: application/json' \
  -H 'Coder-Session-Token: API_KEY'
```

`GET /api/v2/anthropic/{organization}/agents/{user}`

### Parameters

| Name           | In   | Type   | Required | Description              |
|----------------|------|--------|----------|--------------------------|
| `organization` | path | string | true     | Organization ID          |
| `user`         | path | string | true     | User ID, username, or me |

### Example responses

> 200 Response

```json
{
  "agents": [
    {
      "archived": true,
      "created_at": "2019-08-24T14:15:22Z",
      "description": "string",
      "id": "string",
      "model": "string",
      "name": "string",
      "updated_at": "2019-08-24T14:15:22Z",
      "version": 0
    }
  ]
}
```

### Responses

| Status | Meaning                                                 | Description | Schema                                                                         |
|--------|---------------------------------------------------------|-------------|--------------------------------------------------------------------------------|
| 200    | [OK](https://tools.ietf.org/html/rfc7231#section-6.3.1) | OK          | [codersdk.AnthropicAgentsResponse](schemas.md#codersdkanthropicagentsresponse) |

To perform this operation, you must be authenticated. [Learn more](authentication.md).

## Create an Anthropic session for the user

### Code samples

```shell
# Example request using curl
curl -X POST http://coder-server:8080/api/v2/anthropic/{organization}/sessions/{user} \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Coder-Session-Token: API_KEY'
```

`POST /api/v2/anthropic/{organization}/sessions/{user}`

> Body parameter

```json
{
  "agent_id": "string",
  "metadata": {
    "property1": "string",
    "property2": "string"
  },
  "title": "string"
}
```

### Parameters

| Name           | In   | Type                                                                                       | Required | Description              |
|----------------|------|--------------------------------------------------------------------------------------------|----------|--------------------------|
| `organization` | path | string                                                                                     | true     | Organization ID          |
| `user`         | path | string                                                                                     | true     | User ID, username, or me |
| `body`         | body | [codersdk.CreateAnthropicSessionRequest](schemas.md#codersdkcreateanthropicsessionrequest) | true     | Create session request   |

### Example responses

> 201 Response

```json
{
  "agent_id": "string",
  "coder_user_id": "fd606ca1-5b9c-46b3-9534-63f05144fb86",
  "created_at": "2019-08-24T14:15:22Z",
  "environment_id": "string",
  "id": "string",
  "metadata": {
    "property1": "string",
    "property2": "string"
  },
  "title": "string"
}
```

### Responses

| Status | Meaning                                                      | Description | Schema                                                           |
|--------|--------------------------------------------------------------|-------------|------------------------------------------------------------------|
| 201    | [Created](https://tools.ietf.org/html/rfc7231#section-6.3.2) | Created     | [codersdk.AnthropicSession](schemas.md#codersdkanthropicsession) |

To perform this operation, you must be authenticated. [Learn more](authentication.md).

## Send an event to an Anthropic session

### Code samples

```shell
# Example request using curl
curl -X POST http://coder-server:8080/api/v2/anthropic/{organization}/sessions/{user}/{session}/events \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Coder-Session-Token: API_KEY'
```

`POST /api/v2/anthropic/{organization}/sessions/{user}/{session}/events`

> Body parameter

```json
{
  "text": "string"
}
```

### Parameters

| Name           | In   | Type                                                                               | Required | Description              |
|----------------|------|------------------------------------------------------------------------------------|----------|--------------------------|
| `organization` | path | string                                                                             | true     | Organization ID          |
| `user`         | path | string                                                                             | true     | User ID, username, or me |
| `session`      | path | string                                                                             | true     | Anthropic session ID     |
| `body`         | body | [codersdk.SendAnthropicEventRequest](schemas.md#codersdksendanthropiceventrequest) | true     | Send event request       |

### Example responses

> 200 Response

```json
{
  "events": [
    {
      "id": "string",
      "processed_at": "2019-08-24T14:15:22Z",
      "type": "string"
    }
  ]
}
```

### Responses

| Status | Meaning                                                 | Description | Schema                                                                               |
|--------|---------------------------------------------------------|-------------|--------------------------------------------------------------------------------------|
| 200    | [OK](https://tools.ietf.org/html/rfc7231#section-6.3.1) | OK          | [codersdk.SendAnthropicEventResponse](schemas.md#codersdksendanthropiceventresponse) |

To perform this operation, you must be authenticated. [Learn more](authentication.md).
