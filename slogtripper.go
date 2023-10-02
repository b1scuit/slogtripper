package slogtripper

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

var m sync.Once

func Init() {
	m.Do(func() {
		http.DefaultTransport = NewSlogTripper()

		http.DefaultTransport = &SlogTripper{
			proxyTransport: http.DefaultTransport,
		}
	})
}

type Option func(*SlogTripper)

func WithLogger(l *slog.Logger) Option {
	return func(st *SlogTripper) {
		if l == nil {
			l = slog.Default()
		}

		st.logger = l
	}
}

func WithLoggingLevel(level slog.Level) Option {
	return func(st *SlogTripper) {
		st.logAtLevel = level
	}
}

func WithRoundTripper(t http.RoundTripper) Option {
	return func(st *SlogTripper) {
		if t == nil {
			t = http.DefaultTransport
		}

		st.proxyTransport = t
	}
}

func CaptureRequestBody() Option {
	return func(st *SlogTripper) {
		st.captureRequestBody = true
	}
}

func CaptureResponseBody() Option {
	return func(st *SlogTripper) {
		st.captureResponseBody = true
	}
}

func CaptureRequestHeaders() Option {
	return func(st *SlogTripper) {
		st.captureRequestHeaders = true
	}
}

func CaptureResponseHeaders() Option {
	return func(st *SlogTripper) {
		st.captureResponseHeaders = true
	}
}

type SlogTripper struct {
	logger     *slog.Logger
	logAtLevel slog.Level

	proxyTransport http.RoundTripper

	captureRequestBody  bool
	captureResponseBody bool

	captureRequestHeaders  bool
	captureResponseHeaders bool
}

func NewSlogTripper(opts ...Option) *SlogTripper {
	st := &SlogTripper{
		logger:         slog.Default(),
		logAtLevel:     slog.LevelInfo,
		proxyTransport: http.DefaultTransport,
	}

	for _, f := range opts {
		f(st)
	}

	return st
}

func (st *SlogTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// A local instance of slog for this rountrip
	start := time.Now()

	requestGroup := []any{
		slog.Time("started_at", start),
	}

	if req != nil {
		requestGroup = append(requestGroup,
			slog.String("method", req.Method),
			slog.Int64("content_length", req.ContentLength),
			slog.String("proto", req.Proto),
		)

		if u := req.URL; u != nil {
			requestGroup = append(requestGroup, slog.String("url", u.String()))
		}

		if st.captureRequestBody && req.Body != nil {
			b := new(bytes.Buffer)
			_, err := b.ReadFrom(req.Body)

			if err != nil {
				return nil, err
			}
			req.Body.Close()

			requestGroup = append(requestGroup, slog.Any("body_content", b.String()))

			req.Body = io.NopCloser(b)
		}

		if st.captureRequestHeaders && req.Header != nil {
			headers := []any{}

			for name := range req.Header {
				// We don't use value here as value would be a []string and I can't be bothered to check len, pick the one .Get would use and use it
				headers = append(headers, slog.String(name, req.Header.Get(name)))
			}

			if len(headers) != 0 {
				requestGroup = append(requestGroup, slog.Group("headers", headers...))
			}
		}
	}

	res, err := st.proxyTransport.RoundTrip(req)

	responseGroup := []any{}
	if err != nil {
		responseGroup = append(responseGroup, slog.Any("error", err))
	}

	if res != nil {
		responseGroup = append(responseGroup,
			slog.String("status", http.StatusText(res.StatusCode)),
			slog.Int("status_code", res.StatusCode),
			slog.Int64("content_length", res.ContentLength),
			slog.Duration("time_taken", time.Since(start)),
			slog.String("content_type", res.Header.Get("Content-Type")),
		)

		if st.captureResponseBody && res.Body != nil {
			b := new(bytes.Buffer)
			_, err := b.ReadFrom(res.Body)
			if err != nil {
				return nil, err
			}

			res.Body.Close()

			responseGroup = append(responseGroup, slog.Any("body_content", b.String()))

			res.Body = io.NopCloser(b)
		}

		if st.captureResponseHeaders && res.Header != nil {
			headers := []any{}

			for name := range res.Header {
				// We don't use value here as value would be a []string and I can't be bothered to check len, pick the one .Get would use and use it
				headers = append(headers, slog.String(name, req.Header.Get(name)))
			}

			if len(headers) != 0 {
				responseGroup = append(responseGroup, slog.Group("headers", headers...))
			}
		}
	}

	st.log(req.Context(), "HTTP Request", slog.Group("request", requestGroup...), slog.Group("response", responseGroup...))

	return res, err
}

func (st *SlogTripper) log(ctx context.Context, msg string, args ...any) {
	switch st.logAtLevel {
	case slog.LevelDebug:
		st.logger.DebugContext(ctx, msg, args...)
	case slog.LevelInfo:
		st.logger.InfoContext(ctx, msg, args...)
	}
}
