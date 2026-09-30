# bulkvs-ddns

<!-- sf:project:start -->
<!-- sf:project:end -->
<!-- sf:badges:start -->
<!-- sf:badges:end -->
<!-- sf:image:start -->
<!-- sf:image:end -->

DDNS for a [BulkVS](https://www.bulkvs.com/) SIP trunk.

BulkVS authenticates SIP trunks by source IP — you register your public IP as an
**IP Host** (`/ipHost`, the portal's *Host → Add* screen). On a dynamic WAN that
entry goes stale every time the IP rotates and calls break. `bulkvs-ddns` watches
the site's current public IP and keeps the `/ipHost` allowlist in sync — the same
job [`cloudflare-ddns`](https://github.com/favonia/cloudflare-ddns) does for DNS,
pointed at BulkVS instead.

It's a single static Go binary shipped `FROM scratch` (non-root, read-only, no
shell) — the whole thing is stdlib, no dependencies.

## What it does each pass

1. Read the current public IPv4 from a Cloudflare-trace endpoint
   (`https://1.1.1.1/cdn-cgi/trace`) — the same source `cloudflare-ddns` uses.
2. `PUT /ipHost` the current IP (upsert), tagged with your label plus the
   auto-appended `(bulkvs-ddns)` marker.
3. If pruning is on, `DELETE` every *other* `/ipHost` entry carrying that same
   marked Description — old rotated IPs and manual leftovers clean themselves up.

**The marker is the ownership boundary.** The tool appends ` (bulkvs-ddns)` to your
label and writes the composite (e.g. `pbx-host (bulkvs-ddns)`). Only entries whose
`Description` equals that exact composite are ever read as "ours" or pruned —
anything else, including a hand-made entry that happens to share your label, is left
untouched. Because the marker is always appended, the label may even be blank.

> Run it where its egress is the **same WAN the trunk sits behind** (e.g. in the
> same cluster/site). It syncs the IP *it* sees. Single-WAN sites: fine. If the PBX
> and this process ever egress different uplinks, they'd disagree.

## Configuration (env)

Authenticate with **either** the API username + password/token pair (the common
case — we base64 it for you) **or** the portal's pre-computed Basic Auth header.

| Var                       | Default                              | Meaning |
|---------------------------|--------------------------------------|---------|
| `BULKVS_API_USER`         | — (pair)                             | API username / account email (HTTP Basic) |
| `BULKVS_API_KEY`          | — (pair)                             | API password/token (HTTP Basic) |
| `BULKVS_BASIC_AUTH`       | — (optional)                         | pre-computed `Basic <b64>` header; overrides the pair, prefix optional |
| `BULKVS_HOST_DESCRIPTION` | `pbx-host`                           | human label for this deployment; the tool appends ` (bulkvs-ddns)` before writing, so the stored Description reads `pbx-host (bulkvs-ddns)`. May be blank (marker alone) |
| `BULKVS_API_BASE`         | `https://portal.bulkvs.com/api/v1.0` | API root |
| `BULKVS_MAX_OUT`          | *(empty)*                            | optional `MaxOut` stamped on the entry |
| `IP_PROVIDER`             | `cloudflare.trace`                   | IP source (mirrors cloudflare-ddns) |
| `IP_PROVIDER_URL`         | *(derived)*                          | override the trace URL directly |
| `INTERVAL`                | `5m`                                 | reconcile cadence (also accepts `@every 5m`) |
| `HTTP_TIMEOUT`            | `15s`                                | per-request timeout |
| `PRUNE`                   | `true`                               | delete our stale entries |
| `RUN_ONCE`                | `false`                              | reconcile once and exit (CronJob mode) |
| `DRY_RUN`                 | `false`                              | log intended changes, call nothing |

## Run

```sh
docker run --rm \
  -e BULKVS_API_USER=... -e BULKVS_API_KEY=... \
  -e BULKVS_HOST_DESCRIPTION=pbx-host \
  prplanit/bulkvs-ddns
```

Start with `DRY_RUN=true` the first time to see exactly what it would add/prune.

## Caveats

- **Auth scheme + JSON shapes** follow BulkVS's documented `/ipHost` fields (`IP`,
  `Description`, `MaxOut`) and HTTP Basic auth. The API responses aren't fully
  documented publicly — verify list decoding against your live account and adjust
  the struct tags in `src/bulkvs` if BulkVS wraps the array.
- IPv4 only for now (IP-based SIP auth is v4 in practice).

## Layout

```
cmd/bulkvs-ddns   entrypoint + loop
src/config        env config
src/ipsource      public-IP detection (cloudflare.trace)
src/bulkvs        /ipHost API client
src/reconcile     upsert-current + prune-stale
src/version       build-time identity
```
