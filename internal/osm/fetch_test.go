package osm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/davidimannuel/go-running-route-q-learning/internal/logger"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type trackedBody struct {
	io.Reader
	onClose func()
}

func (b *trackedBody) Close() error {
	b.onClose()
	return nil
}

type responseSequence struct {
	calls      int
	closed     int
	afterClose func()
	request    *http.Request
	form       url.Values
}

func newSequenceClient(t *testing.T, statuses []int, payload string) (*overpassClient, *responseSequence) {
	t.Helper()
	sequence := &responseSequence{}
	client := &overpassClient{
		endpoint: "https://overpass.invalid/api/interpreter",
		httpC: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if sequence.closed != sequence.calls {
				t.Errorf("previous response body was not closed before the next request")
			}
			if sequence.calls >= len(statuses) {
				t.Errorf("unexpected request %d", sequence.calls+1)
				return nil, errors.New("unexpected extra request")
			}
			status := statuses[sequence.calls]
			sequence.calls++
			sequence.request = req
			requestBody, err := io.ReadAll(req.Body)
			req.Body.Close()
			if err != nil {
				return nil, err
			}
			sequence.form, err = url.ParseQuery(string(requestBody))
			if err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: status,
				Header:     make(http.Header),
				Request:    req,
				Body: &trackedBody{Reader: strings.NewReader(payload), onClose: func() {
					sequence.closed++
					if sequence.afterClose != nil {
						sequence.afterClose()
					}
				}},
			}, nil
		})},
	}
	return client, sequence
}

func validFetchParam() FetchParam {
	return FetchParam{Lat: -7.282187, Long: 112.709282, RadM: 1000, TimeoutSec: 120}
}

func TestFetchSuccess(t *testing.T) {
	client, sequence := newSequenceClient(t, []int{200}, `{"version":0.6,"elements":[]}`)
	var logs bytes.Buffer
	ctx := logger.ContextWithLogger(context.Background(), logger.New(&logs, true))
	data, err := client.Fetch(ctx, validFetchParam())
	if err != nil {
		t.Fatal(err)
	}
	if data.Version != 0.6 || sequence.calls != 1 || sequence.closed != 1 {
		t.Fatalf("version=%v, requests=%d, closed bodies=%d", data.Version, sequence.calls, sequence.closed)
	}
	var queryLog struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(bytes.NewReader(logs.Bytes())).Decode(&queryLog); err != nil {
		t.Fatal(err)
	}
	if queryLog.Query != sequence.form.Get("data") {
		t.Fatalf("logged query=%q; sent query=%q", queryLog.Query, sequence.form.Get("data"))
	}
	for _, message := range []string{"Prepared Overpass query", "Sending Overpass request", "Overpass response received"} {
		if !strings.Contains(logs.String(), message) {
			t.Fatalf("missing debug message %q: %s", message, logs.String())
		}
	}
	if sequence.request.Method != http.MethodPost {
		t.Errorf("method=%q; want POST", sequence.request.Method)
	}
	if sequence.request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" ||
		sequence.request.Header.Get("Accept") != "application/json" ||
		!strings.HasPrefix(sequence.request.Header.Get("User-Agent"), "go-running-route-q-learning/") {
		t.Errorf("unexpected request headers: %v", sequence.request.Header)
	}
	if !strings.Contains(sequence.form.Get("data"), "(-7.29118020,112.70021567,-7.27319380,112.71834833)") {
		t.Errorf("unexpected query: %s", sequence.form.Get("data"))
	}
}

func TestFetchRetriesThenSucceeds(t *testing.T) {
	client, sequence := newSequenceClient(t, []int{503, 200}, `{"version":0.6,"elements":[]}`)
	var logs bytes.Buffer
	ctx := logger.ContextWithLogger(context.Background(), logger.New(&logs, false))
	_, err := client.Fetch(ctx, validFetchParam())
	if !strings.Contains(logs.String(), `"level":"WARN"`) || !strings.Contains(logs.String(), `"status":503`) || !strings.Contains(logs.String(), `"delay":1000000000`) || strings.Contains(logs.String(), `"level":"DEBUG"`) {
		t.Fatalf("unexpected retry logs: %s", logs.String())
	}
	if err != nil || sequence.calls != 2 || sequence.closed != 2 {
		t.Fatalf("error=%v, requests=%d, closed bodies=%d", err, sequence.calls, sequence.closed)
	}
}

func TestFetchRetryExhaustion(t *testing.T) {
	client, sequence := newSequenceClient(t, []int{503, 503, 503}, "service unavailable")
	_, err := client.Fetch(context.Background(), validFetchParam())
	if err == nil || !strings.Contains(err.Error(), "3 attempts") || !strings.Contains(err.Error(), "503") {
		t.Fatalf("expected an exhaustion error containing the HTTP status; got %v", err)
	}
	if sequence.calls != 3 || sequence.closed != 3 {
		t.Fatalf("requests=%d, closed bodies=%d", sequence.calls, sequence.closed)
	}
}

func TestFetchRejectsFailedResponses(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		payload string
	}{
		{"bad request", 400, "invalid query"},
		{"not acceptable", 406, "request rejected"},
		{"invalid JSON", 200, "not JSON"},
		{"server remark", 200, `{"remark":"runtime error: Query timed out","elements":[]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, sequence := newSequenceClient(t, []int{tc.status}, tc.payload)
			_, err := client.Fetch(context.Background(), validFetchParam())
			if err == nil || sequence.calls != 1 || sequence.closed != 1 {
				t.Fatalf("error=%v, requests=%d, closed bodies=%d", err, sequence.calls, sequence.closed)
			}
		})
	}
}

func TestFetchPreservesTransportError(t *testing.T) {
	client, sequence := newSequenceClient(t, nil, "")
	client.httpC.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, context.Canceled
	})
	_, err := client.Fetch(context.Background(), validFetchParam())
	if !errors.Is(err, context.Canceled) || sequence.calls != 0 {
		t.Fatalf("expected wrapped cancellation; got %v", err)
	}
}

func TestFetchCancellation(t *testing.T) {
	for _, status := range []int{503, 429} {
		ctx, cancel := context.WithCancel(context.Background())
		client, sequence := newSequenceClient(t, []int{status}, "try later")
		sequence.afterClose = cancel
		_, err := client.Fetch(ctx, validFetchParam())
		cancel()
		if !errors.Is(err, context.Canceled) || sequence.calls != 1 || sequence.closed != 1 {
			t.Fatalf("status=%d, error=%v, requests=%d, closed bodies=%d", status, err, sequence.calls, sequence.closed)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client, sequence := newSequenceClient(t, nil, "")
	_, err := client.Fetch(ctx, validFetchParam())
	if !errors.Is(err, context.Canceled) || sequence.calls != 0 {
		t.Fatalf("expected cancellation before HTTP; error=%v, requests=%d", err, sequence.calls)
	}
}

func TestFetchValidatesParametersBeforeHTTP(t *testing.T) {
	cases := []struct {
		name   string
		change func(*FetchParam)
	}{
		{"swapped latitude", func(p *FetchParam) { p.Lat = 112.709282 }},
		{"latitude NaN", func(p *FetchParam) { p.Lat = math.NaN() }},
		{"latitude infinity", func(p *FetchParam) { p.Lat = math.Inf(1) }},
		{"north pole", func(p *FetchParam) { p.Lat = 90 }},
		{"south pole", func(p *FetchParam) { p.Lat = -90 }},
		{"invalid longitude", func(p *FetchParam) { p.Long = 181 }},
		{"longitude NaN", func(p *FetchParam) { p.Long = math.NaN() }},
		{"longitude infinity", func(p *FetchParam) { p.Long = math.Inf(-1) }},
		{"zero radius", func(p *FetchParam) { p.RadM = 0 }},
		{"negative radius", func(p *FetchParam) { p.RadM = -1 }},
		{"radius NaN", func(p *FetchParam) { p.RadM = math.NaN() }},
		{"radius infinity", func(p *FetchParam) { p.RadM = math.Inf(1) }},
		{"oversized radius", func(p *FetchParam) { p.RadM = math.MaxFloat64 }},
		{"zero timeout", func(p *FetchParam) { p.TimeoutSec = 0 }},
		{"negative timeout", func(p *FetchParam) { p.TimeoutSec = -1 }},
		{"crosses antimeridian", func(p *FetchParam) { p.Long = 179.999 }},
		{"crosses pole", func(p *FetchParam) { p.Lat = 89.999 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := validFetchParam()
			tc.change(&p)
			client, sequence := newSequenceClient(t, nil, "")
			_, err := client.Fetch(context.Background(), p)
			if err == nil || sequence.calls != 0 {
				t.Fatalf("expected validation error before HTTP; error=%v, requests=%d", err, sequence.calls)
			}
		})
	}
}
