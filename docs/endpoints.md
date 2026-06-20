# Mobilithek Subscription Endpoints

The package can build the common subscription URLs from a base URL and subscription ID.

Default base URL:

```text
https://mobilithek.info:8443
```

Supported endpoint kinds:

| Kind | Path |
| --- | --- |
| `generic` | `/mobilithek/api/V1.0/subscription?subscriptionID={id}` |
| `datex2-v2` | `/mobilithek/api/v1.0/subscription/{id}/clientPullService?subscriptionID={id}` |
| `datex2-v3` | `/mobilithek/api/v1.0/subscription/datexv3?subscriptionID={id}` |
| `container` | `/mobilithek/api/v1.0/container/subscription?subscriptionID={id}` |
| `auto` | tries `generic`, `datex2-v3`, `datex2-v2`, then `container` |

Use `EndpointURL` when you only need URL construction:

```go
u, err := mobilithek.EndpointURL(
	mobilithek.DefaultBaseURL,
	"123456789012345678",
	mobilithek.EndpointDATEX2V2,
)
```

Use `FetchSubscription` when you want the package to perform the HTTP request:

```go
response, err := client.FetchSubscription(ctx, "123456789012345678", mobilithek.EndpointAuto)
```

Subscription IDs are not offer IDs. Subscribe to an offer in Mobilithek first, then use the concrete subscription ID assigned to your account.
