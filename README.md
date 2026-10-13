# Fizz-buzz REST server

A small HTTP service, written in Go, that generates customisable fizz-buzz
sequences and reports which request was asked for the most.

- `GET /fizzbuzz` returns the numbers from 1 to `limit`, where multiples of
  `int1` are replaced by `str1`, multiples of `int2` by `str2`, and multiples
  of both by `str1str2`.
- `GET /stats` returns the parameters of the most frequent request and its
  number of hits.

## Quick start

With Docker (the server and Redis):

```sh
docker compose up --build
```

Without Docker (Go 1.26+, statistics kept in memory):

```sh
go run ./cmd/server
```

Then:

```sh
curl 'localhost:8080/fizzbuzz?int1=3&int2=5&limit=15&str1=fizz&str2=buzz'
# ["1","2","fizz","4","buzz","fizz","7","8","fizz","buzz","11","fizz","13","14","fizzbuzz"]

curl localhost:8080/stats
# {"request":{"int1":3,"int2":5,"limit":15,"str1":"fizz","str2":"buzz"},"hits":1}
```

The interactive documentation (Swagger UI) is served at
<http://localhost:8080/docs>.

## API

The reference is the OpenAPI document, [`api/openapi.yaml`](api/openapi.yaml),
which the server also exposes at `/openapi.yaml`.

| Endpoint            | Description                                |
|---------------------|--------------------------------------------|
| `GET /fizzbuzz`     | Generate a sequence                        |
| `GET /stats`        | Most frequent request and its hit count    |
| `GET /healthz`      | Liveness probe                             |
| `GET /docs`         | Swagger UI                                 |
| `GET /openapi.yaml` | OpenAPI 3 specification                    |

### `GET /fizzbuzz`

All five query parameters are required.

| Parameter      | Type    | Constraints                                    |
|----------------|---------|------------------------------------------------|
| `int1`, `int2` | integer | at least 1                                     |
| `limit`        | integer | between 1 and `MAX_LIMIT` (10000 by default)   |
| `str1`, `str2` | string  | 1 to `MAX_STRING_LENGTH` characters (64 by default) |

It answers `200` with a JSON array of strings, or `400` when a parameter is
missing or invalid.

### `GET /stats`

| Status | Meaning                                              |
|--------|------------------------------------------------------|
| `200`  | `{"request": {...}, "hits": n}`                      |
| `404`  | No fizz-buzz request has been served yet             |
| `503`  | The statistics store is unreachable                  |

Only valid requests are counted. When several requests are tied, one of them
is returned.

### Errors

Every error has the same shape, with a stable machine-readable `code`:

```json
{"code": "invalid_parameter", "message": "int1 must be greater than or equal to 1"}
```

## Configuration

The server is configured with environment variables.

| Variable            | Default          | Description                                        |
|---------------------|------------------|----------------------------------------------------|
| `HTTP_ADDR`         | `:8080`          | Address the server listens on                      |
| `LOG_LEVEL`         | `info`           | `debug`, `info`, `warn` or `error`                 |
| `SHUTDOWN_TIMEOUT`  | `10s`            | Time given to in-flight requests on shutdown       |
| `MAX_LIMIT`         | `10000`          | Highest accepted `limit`                           |
| `MAX_STRING_LENGTH` | `64`             | Longest accepted `str1` or `str2`, in characters   |
| `STATS_STORE`       | `memory`         | `memory` or `redis` (the Docker image sets `redis`) |
| `STATS_TIMEOUT`     | `200ms`          | Timeout of every call to the statistics store      |
| `REDIS_ADDR`        | `localhost:6379` | Redis address                                      |
| `REDIS_PASSWORD`    | empty            | Redis password                                     |
| `REDIS_DB`          | `0`              | Redis database                                     |

## Project layout

```
cmd/server/         entry point: configuration, wiring, graceful shutdown
internal/fizzbuzz/  the sequence itself, with no dependency on HTTP
internal/stats/     hit counters: Redis and in-memory implementations
internal/httpapi/   routes, validation, JSON responses, middleware
internal/config/    configuration loaded from the environment
api/                OpenAPI specification, embedded in the binary
```

## Design decisions

**Standard library first.** Routing uses `net/http` (method-aware patterns
since Go 1.22) and logging uses `log/slog`. Two routes do not justify a web
framework. The only runtime dependency is the Redis client.

**GET with query parameters.** Generating a sequence is a safe, idempotent
read, so it can be cached and tried from a browser.

**Bounded requests.** `limit` and the length of the words are capped, so a
single call cannot make the server allocate an arbitrarily large response.
With the defaults the largest response is 10000 strings of at most 128
characters. The HTTP server also sets read, write and idle timeouts.

**Statistics in a Redis sorted set.** Each distinct request is a member of
the sorted set `fizzbuzz:hits`, scored by its hit count. `ZINCRBY` makes the
increment atomic, so counts stay exact with any number of instances and
survive restarts, and `ZREVRANGE 0 0` reads the leader in one call. The member
is the request encoded as JSON, which stays unambiguous whatever characters
the words contain.

**Statistics are best effort.** When Redis is down, `/fizzbuzz` keeps
answering: the failure is logged, the hit is not counted, and `/stats` answers
`503`. The Redis client is configured to fail fast (no retries, short
timeouts) so that an outage does not slow requests down. For the same reason
`/healthz` does not check Redis: failing the probe would take healthy
instances out of rotation for a feature the main endpoint does not need.

**A store interface with two implementations.** The HTTP layer depends on a
two-method interface. The in-memory store keeps local runs and tests free of
any infrastructure; it is not meant for production and the server says so at
startup.

**Hand-written OpenAPI document.** The specification is a plain YAML file
embedded in the binary, rather than generated from annotations, which keeps
code generation out of the build. Swagger UI is loaded from a CDN, pinned to
an exact version with subresource integrity hashes.

**Operations.** Structured JSON logs with a request ID (taken from
`X-Request-ID` or generated, and echoed in the response), panic recovery,
graceful shutdown on `SIGINT`/`SIGTERM`, and a distroless image running as a
non-root user.

## Known limitations and next steps

- The number of distinct requests is unbounded, so the sorted set grows with
  them. A production deployment with hostile traffic would need a cap, for
  example periodic trimming of the least frequent members.
- Hits are recorded synchronously. A Redis that accepts connections but
  answers slowly adds up to `STATS_TIMEOUT` to each request; a circuit breaker
  or asynchronous recording would remove that.
- There is no authentication or rate limiting. They are expected to be
  handled by the gateway in front of the service.
- There are no metrics or tracing yet. A Prometheus endpoint would be the
  first addition.
- The OpenAPI document is maintained by hand, so it has to be updated along
  with the handlers.
- The Redis store is tested against [miniredis](https://github.com/alicebob/miniredis),
  an in-process fake. An integration test against a real Redis would
  complement it.

## Development

```sh
make test    # tests with the race detector
make cover   # coverage per function
make lint    # golangci-lint v2
make up      # server + Redis with Docker Compose
```

CI (GitHub Actions) runs `go vet`, the tests with the race detector,
golangci-lint and a Docker build on every push and pull request.
