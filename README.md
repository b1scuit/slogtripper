# slogtripper
[![Go Reference](https://pkg.go.dev/badge/github.com/b1scuit/slogtripper.svg)](https://pkg.go.dev/github.com/b1scuit/slogtripper)

A http.Roundtripper outputting what it can through slog for visibility
## Usage
Simple as can be, in the top of your application wherever you need call
```go
slogtripper.Init()
```

Or pass an instance to something with
```go
roundTripper := slogtripper.NewSlogTripper()
```

