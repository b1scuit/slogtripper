# slogtripper
[![Go Reference](https://pkg.go.dev/badge/github.com/b1scuit/slogtripper.svg)](https://pkg.go.dev/github.com/b1scuit/slogtripper)

A http.Roundtripper outputting what it can through slog for visibility
Whenever your application makes a HTTP call, this hooks deep into the http roundtripper and allows you to see whats going in/out of your application
i.e.
```sh
2023/10/02 23:43:53 INFO HTTP Request request.started_at=2023-10-02T23:43:53.463+01:00 request.method=GET request.content_length=0 request.proto=HTTP/1.1 request.url=http://localhost response.status=OK response.status_code=200 response.content_length=18 response.time_taken=14.643µs response.content_type=application/json response.body_content="{\"ping\": \"pong\"}"
```
or as JSON (using slog.JsonHandler)
```json
{
    "time": "2023-10-02T23:46:33.768115+01:00",
    "level": "INFO",
    "msg": "HTTP Request",
    "request": {
        "started_at": "2023-10-02T23:46:33.768068+01:00",
        "method": "GET",
        "content_length": 0,
        "proto": "HTTP/1.1",
        "url": "http://localhost"
    },
    "response": {
        "status": "OK",
        "status_code": 200,
        "content_length": 18,
        "time_taken": 15019,
        "content_type": "application/json",
        "body_content": "{\"ping\": \"pong\"}"
    }
}
```

You can optionally choose to also include body content of the request, response or both

## Usage
Simple as can be, in the top of your application wherever you need call
```go
slogtripper.Init()
```

Or pass an instance to something with
```go
roundTripper := slogtripper.NewSlogTripper()
```

