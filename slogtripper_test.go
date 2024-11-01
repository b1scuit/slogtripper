package slogtripper

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
)

type MockRoundTripper struct {
	MockRoundTrip func(*http.Request) (*http.Response, error)
}

func (mrt *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return mrt.MockRoundTrip(req)
}

type ErrorReadCloser struct{}

func (erc *ErrorReadCloser) Read(p []byte) (n int, err error) {
	return 0, errors.New("error")
}
func (erc *ErrorReadCloser) Close() error {
	return nil
}

type Scenario struct {
	Req  *http.Request
	Res  *http.Response
	Err  error
	Name string
}

var scenarios = []*Scenario{
	{
		Name: "Basic GET: /",
		Req:  Must(http.NewRequest(http.MethodGet, "http://localhost/", nil)),
		Res: &http.Response{
			Body:       io.NopCloser(strings.NewReader(`{"gday":"back"}`)),
			StatusCode: http.StatusOK,
		},
		Err: nil,
	},
	{
		Name: "Basic GET: / with headers",
		Req: &http.Request{
			URL: Must(url.Parse("http://localhst")),
			Header: http.Header{
				"example": []string{"value"},
			},
		},
		Res: &http.Response{
			Body:       io.NopCloser(strings.NewReader(`{"gday":"back"}`)),
			StatusCode: http.StatusOK,
		},
		Err: nil,
	},
	{
		Name: "Basic POST: /",
		Req:  Must(http.NewRequest(http.MethodPost, "http://localhost/", strings.NewReader(`{"hello": "world"}`))),
		Res: &http.Response{
			Body:       io.NopCloser(strings.NewReader(`{"gday":"back"}`)),
			StatusCode: http.StatusOK,
		},
		Err: nil,
	},
}

func Must[T any](in T, err error) T {
	if err != nil {
		panic(err)
	}

	return in
}

func TestNew(t *testing.T) {
	st := NewSlogTripper()

	if st == nil {
		t.Error("Slog Tripper created as nil")
	}
}

func TestFaultySetup(t *testing.T) {
	http.DefaultTransport = &MockRoundTripper{
		MockRoundTrip: func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
			}, nil
		},
	}

	// Should default to global settings
	st := NewSlogTripper(
		WithLogger(nil),
		WithRoundTripper(nil),
	)

	_, err := st.RoundTrip(Must(http.NewRequest(http.MethodGet, "http://localhost", nil)))
	if err != nil {
		t.Errorf("Error in roundtrip: %v", err)
	}
}

func TestExampleScenarios(t *testing.T) {
	// Loopthrouh all the scenarios we want to check
	for _, scenario := range scenarios {
		scenario := scenario
		t.Run(scenario.Name, func(t *testing.T) {
			t.Parallel()

			// Set up our mock transport to return our expected response
			mrt := &MockRoundTripper{
				MockRoundTrip: func(r *http.Request) (*http.Response, error) {
					return scenario.Res, scenario.Err
				},
			}

			// Create a SlogTripper
			st := NewSlogTripper(
				WithLogger(slog.Default()),
				WithLoggingLevel(slog.LevelDebug),
				WithRoundTripper(mrt),
				CaptureRequestBody(),
				CaptureResponseBody(),
				CaptureRequestHeaders(),
				CaptureResponseHeaders(),
			)

			// Run
			res, err := st.RoundTrip(scenario.Req)

			// Test Result
			if err != scenario.Err {
				t.Errorf("Unexpected Error: %v", err)
			}

			// Compare res
			_ = res
		})
	}
}

func TestDefaultLoggerChangeAfterInit(t *testing.T) {
	req := Must(http.NewRequest(http.MethodGet, "http://localhost/", nil))
	res := &http.Response{
		Body:       io.NopCloser(strings.NewReader(`{"gday":"back"}`)),
		StatusCode: http.StatusOK,
	}

	// Set up our mock transport to return our expected response
	mrt := &MockRoundTripper{
		MockRoundTrip: func(r *http.Request) (*http.Response, error) {
			return res, nil
		},
	}

	originalDefault := slog.Default()
	defer slog.SetDefault(originalDefault)

	var firstOutput bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&firstOutput, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// Create a SlogTripper without an explicit logger
	st := NewSlogTripper(
		WithLoggingLevel(slog.LevelDebug),
		WithRoundTripper(mrt),
		CaptureRequestBody(),
		CaptureResponseBody(),
		CaptureRequestHeaders(),
		CaptureResponseHeaders(),
	)

	// Run the request with the default logger set to INFO
	_, _ = st.RoundTrip(req)

	// expect no log printed
	expectedLogMessage := "\"msg\":\"HTTP Request\""
	if strings.Contains(firstOutput.String(), expectedLogMessage) {
		t.Errorf("Log contains message when should not: %s", firstOutput.String())
	}

	// Run the request again with the default logger updated to DEBUG without updating the SlogTripper instance
	var secondOutput bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&secondOutput, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	_, _ = st.RoundTrip(req)

	// expect log printed
	if !strings.Contains(secondOutput.String(), expectedLogMessage) {
		t.Errorf("Log does not contain expected message: %s", secondOutput.String())
	}
}

func TestUnmarshalResponse(t *testing.T) {
	mrt := &MockRoundTripper{
		MockRoundTrip: func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				Header: http.Header{
					"Example-Header": []string{"value"},
				},
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"ping": "pong"}`)),
			}, nil
		},
	}

	jsonLogger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))

	st := NewSlogTripper(
		WithLogger(jsonLogger),
		WithRoundTripper(mrt),
		CaptureRequestBody(),
		CaptureResponseBody(),
		CaptureRequestHeaders(),
		CaptureResponseHeaders(),
	)

	res, err := st.RoundTrip(&http.Request{
		URL: Must(url.Parse("http://localhost")),
		Header: http.Header{
			"Example-Header": []string{"value"},
		},
	})

	if err != nil {
		t.Errorf("Error returned: %v", err)
	}

	// A normal net/http unmarshall
	defer res.Body.Close()

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		t.Errorf("Error reading body content: %v", err)
	}

	something := map[string]any{}
	if err := json.Unmarshal(bodyBytes, &something); err != nil {
		t.Errorf("Error Unmarshalling body %v", err)
	}

	if _, ok := something["ping"].(string); !ok {
		t.Error("Body content missing")
	}
}

func TestInit(t *testing.T) {
	Init()

	if reflect.TypeOf(http.DefaultTransport) != reflect.TypeOf(&SlogTripper{}) {
		t.Error("Init fuction failed to replace default roundtripper")
	}
}

func TestResponseError(t *testing.T) {
	_, err := NewSlogTripper(
		WithRoundTripper(&MockRoundTripper{
			MockRoundTrip: func(r *http.Request) (*http.Response, error) {
				return nil, errors.New("mock error")
			},
		}),
	).RoundTrip(Must(http.NewRequest(http.MethodGet, "http://localhost", nil)))

	if err == nil {
		t.Error("Error should have been returned")
	}
}
func TestFaultyRequestBody(t *testing.T) {
	_, err := NewSlogTripper(
		CaptureRequestBody(),
		WithRoundTripper(&MockRoundTripper{
			MockRoundTrip: func(r *http.Request) (*http.Response, error) {
				return nil, nil
			},
		}),
	).RoundTrip(&http.Request{Body: &ErrorReadCloser{}})

	if err == nil {
		t.Error("Error should have been returned")
	}
}
func TestFaultyResponseBody(t *testing.T) {
	_, err := NewSlogTripper(
		CaptureResponseBody(),
		WithRoundTripper(&MockRoundTripper{
			MockRoundTrip: func(r *http.Request) (*http.Response, error) {
				return &http.Response{Body: &ErrorReadCloser{}}, nil
			},
		}),
	).RoundTrip(Must(http.NewRequest(http.MethodGet, "http://localhost", nil)))

	if err == nil {
		t.Error("Error should have been returned")
	}
}
