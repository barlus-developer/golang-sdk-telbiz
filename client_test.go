package telbiz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func mockResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func authBody(token string, expire int) string {
	return fmt.Sprintf(`{"accessToken":%q,"expire":%d,"code":"SUCCESS_OPERATION","message":"ok","success":true}`, token, expire)
}

const acceptedBody = `{"response":{"code":"SENDING_SMS_INPROGRESS","message":"accepted","success":true,"detail":"processing"},"key":{"partitionKey":"","rangeKey":""}}`
const rejectedBody = `{"code":"REQUEST_FAIL","message":"sensitive-provider-echo","success":false,"detail":"sensitive-provider-detail"}`

func mockConfig(rt roundTripFunc) Config {
	return Config{ClientID: "fake-client", ClientSecret: "fake-secret", HTTPClient: &http.Client{Transport: rt}}
}

func mustClient(t *testing.T, cfg Config) *Client {
	t.Helper()
	c, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func send(c *Client, ctx context.Context, topup bool) error {
	if topup {
		_, err := c.Topup(ctx, TopupRequest{Phone: "2012345678", Amount: 10000})
		return err
	}
	_, err := c.SendSMS(ctx, SMSRequest{Title: TitleOTP, Phone: "2012345678", Message: "fabricated test content"})
	return err
}

func TestWireContract(t *testing.T) {
	var authCalls, smsCalls, topupCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("incorrect method or content type")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/Connect/token":
			authCalls.Add(1)
			if !reflect.DeepEqual(body, map[string]any{"clientID": "fake-client", "secret": "fake-secret", "grantType": "client_credentials"}) || r.Header.Get("Authorization") != "" {
				t.Error("incorrect auth contract")
			}
			io.WriteString(w, authBody("fake-access", 3600))
		case "/api/v1/SMSService/newtransaction":
			smsCalls.Add(1)
			if r.Header.Get("Authorization") != "bearer fake-access" {
				t.Error("incorrect bearer header")
			}
			if !reflect.DeepEqual(body, map[string]any{"title": "OTP", "phone": "2012345678", "message": "  fabricated content  "}) {
				t.Error("SMS body changed")
			}
			io.WriteString(w, acceptedBody)
		case "/api/v1/TopupService/newtransaction":
			topupCalls.Add(1)
			if r.Header.Get("Authorization") != "bearer fake-access" {
				t.Error("incorrect topup authorization")
			}
			if !reflect.DeepEqual(body, map[string]any{"phone": "2012345678", "amount": float64(10001)}) {
				t.Error("amount not numeric whole kip")
			}
			io.WriteString(w, `{"response":{"code":"TOPUP_BALANCE_INPROGRESS","success":true,"message":"ok","detail":"processing"},"key":{"partitionKey":"p","rangeKey":"r"}}`)
		default:
			t.Error("unexpected route")
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	c := mustClient(t, Config{ClientID: "fake-client", ClientSecret: "fake-secret", BaseURL: server.URL})
	defer c.Close()
	if authCalls.Load() != 1 {
		t.Fatal("constructor did not authenticate eagerly")
	}
	sms, err := c.SendSMS(context.Background(), SMSRequest{Title: TitleOTP, Phone: "2012345678", Message: "  fabricated content  "})
	if err != nil || sms.Response.Code != CodeSendingSMSInProgress || !sms.Response.Success || sms.Key != (TransactionKey{}) {
		t.Fatalf("SMS result: %#v, %v", sms, err)
	}
	topup, err := c.Topup(context.Background(), TopupRequest{Phone: "2012345678", Amount: 10001})
	if err != nil || topup.Response.Code != CodeTopupBalanceInProgress || topup.Key.RangeKey != "r" {
		t.Fatalf("topup result: %#v, %v", topup, err)
	}
	if smsCalls.Load() != 1 || topupCalls.Load() != 1 {
		t.Fatal("incorrect submission count")
	}
}

func TestConfigurationValidation(t *testing.T) {
	base := mockConfig(func(*http.Request) (*http.Response, error) {
		t.Error("invalid configuration reached transport")
		return mockResponse(200, authBody("fake", 3600)), nil
	})
	for _, tc := range []struct {
		name   string
		change func(*Config)
	}{
		{"empty-id", func(c *Config) { c.ClientID = " " }},
		{"empty-secret", func(c *Config) { c.ClientSecret = "" }},
		{"negative-lead", func(c *Config) { c.RefreshBefore = -1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base
			tc.change(&cfg)
			c, err := New(context.Background(), cfg)
			if c != nil || !errors.Is(err, ErrValidation) {
				t.Fatal("wanted validation error")
			}
		})
	}
	for _, raw := range []string{"relative", "://bad", "ftp://example.com", "https://user:secret@example.com", "https://example.com?q=1", "https://example.com?", "https://example.com#frag", "https://example.com#", "https://example.com/prefix", "https://example.com:abc", "https://example.com:99999", "https://example.com:", "http://example.com", "https://"} {
		t.Run(raw, func(t *testing.T) {
			cfg := base
			cfg.BaseURL = raw
			_, err := New(context.Background(), cfg)
			if !errors.Is(err, ErrValidation) {
				t.Fatal("wanted validation error")
			}
		})
	}
	for _, raw := range []string{"", "https://api.telbiz.la/", "http://localhost:8080", "http://127.0.0.1:8080", "http://[::1]:8080"} {
		if _, err := baseURL(raw); err != nil {
			t.Errorf("valid base URL rejected: %s", raw)
		}
	}
	if _, err := New(nil, base); !errors.Is(err, ErrValidation) {
		t.Fatal("nil context accepted")
	}
}

func TestConstructorResponses(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		kind       error
	}{
		{"flat-failure", rejectedBody, 200, ErrProvider},
		{"unauthorized", rejectedBody, 401, ErrProvider},
		{"missing-success", `{"accessToken":"fake","expire":3600}`, 200, ErrMalformedResponse},
		{"missing-token", `{"expire":3600,"success":true}`, 200, ErrMalformedResponse},
		{"empty-token", `{"accessToken":" ","expire":3600,"success":true}`, 200, ErrMalformedResponse},
		{"unsafe-token", `{"accessToken":"fake\r\nvalue","expire":3600,"success":true}`, 200, ErrMalformedResponse},
		{"missing-expire", `{"accessToken":"fake","success":true}`, 200, ErrMalformedResponse},
		{"zero-expire", `{"accessToken":"fake","expire":0,"success":true}`, 200, ErrMalformedResponse},
		{"negative-expire", `{"accessToken":"fake","expire":-1,"success":true}`, 200, ErrMalformedResponse},
		{"fractional-expire", `{"accessToken":"fake","expire":1.5,"success":true}`, 200, ErrMalformedResponse},
		{"string-expire", `{"accessToken":"fake","expire":"3600","success":true}`, 200, ErrMalformedResponse},
		{"duration-overflow", `{"accessToken":"fake","expire":9223372037,"success":true}`, 200, ErrMalformedResponse},
		{"int-overflow", `{"accessToken":"fake","expire":9223372036854775808,"success":true}`, 200, ErrMalformedResponse},
		{"empty", "", 200, ErrMalformedResponse},
		{"truncated", `{"success":`, 200, ErrMalformedResponse},
		{"trailing-json", authBody("fake", 3600) + `{}`, 200, ErrMalformedResponse},
		{"html", "<html>error</html>", 502, ErrProvider},
		{"oversized", strings.Repeat("x", maxResponseBytes+1), 200, ErrMalformedResponse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			cfg := mockConfig(func(*http.Request) (*http.Response, error) {
				calls.Add(1)
				return mockResponse(tc.status, tc.body), nil
			})
			c, err := New(context.Background(), cfg)
			if c != nil || !errors.Is(err, ErrAuthentication) || !errors.Is(err, tc.kind) || calls.Load() != 1 {
				t.Fatalf("bad constructor error: %v, calls %d", err, calls.Load())
			}
			var e *Error
			if !errors.As(err, &e) || e.HTTPStatus != tc.status {
				t.Fatal("HTTP status missing")
			}
		})
	}
}

func TestInputValidation(t *testing.T) {
	var services atomic.Int32
	c := mustClient(t, mockConfig(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/api/v1/Connect/token" {
			return mockResponse(200, authBody("fake", 3600)), nil
		}
		services.Add(1)
		return mockResponse(200, acceptedBody), nil
	}))
	defer c.Close()
	valid := SMSRequest{Title: TitleOTP, Phone: "2012345678", Message: "fabricated"}
	for _, req := range []SMSRequest{{Title: "bad", Phone: valid.Phone, Message: valid.Message}, {Phone: valid.Phone, Message: valid.Message}, {Title: TitleOTP, Phone: "+8562012345678", Message: valid.Message}, {Title: TitleOTP, Phone: "201234567", Message: valid.Message}, {Title: TitleOTP, Phone: "2112345678", Message: valid.Message}, {Title: TitleOTP, Phone: "201234567x", Message: valid.Message}, {Title: TitleOTP, Phone: "２０123456", Message: valid.Message}, {Title: TitleOTP, Phone: valid.Phone, Message: "\n\t "}} {
		_, err := c.SendSMS(context.Background(), req)
		if !errors.Is(err, ErrValidation) {
			t.Fatal("invalid SMS accepted")
		}
	}
	for _, req := range []TopupRequest{{Phone: valid.Phone, Amount: 9999}, {Phone: valid.Phone, Amount: -1}, {Phone: "+8562012345678", Amount: 10000}} {
		_, err := c.Topup(context.Background(), req)
		if !errors.Is(err, ErrValidation) {
			t.Fatal("invalid topup accepted")
		}
	}
	if services.Load() != 0 {
		t.Fatal("validation submitted a service request")
	}
	for _, title := range []SMSTitle{TitleOTP, TitleInfo, TitlePromotion, TitleNews} {
		req := valid
		req.Title = title
		req.Message = strings.Repeat("fabricated ", 1000)
		if _, err := c.SendSMS(context.Background(), req); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := c.Topup(context.Background(), TopupRequest{Phone: valid.Phone, Amount: 100000001}); err != nil {
		t.Fatal("invented maximum or denomination", err)
	}
	if _, err := c.SendSMS(nil, valid); !errors.Is(err, ErrValidation) {
		t.Fatal("nil context accepted")
	}
	if _, err := c.Topup(nil, TopupRequest{Phone: valid.Phone, Amount: 10000}); !errors.Is(err, ErrValidation) {
		t.Fatal("nil context accepted")
	}
	if err := c.RefreshToken(nil); !errors.Is(err, ErrValidation) {
		t.Fatal("nil context accepted")
	}
}

func TestServiceEnvelopes(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		kind       error
		unknown    bool
	}{
		{"accepted", acceptedBody, 200, nil, false},
		{"unknown-code", `{"response":{"success":true,"code":"FUTURE_CODE"},"key":{"partitionKey":"p","rangeKey":"r"}}`, 200, nil, false},
		{"flat-rejection", rejectedBody, 400, ErrProvider, false},
		{"200-rejection", rejectedBody, 200, ErrProvider, false},
		{"nested-rejection", `{"response":{"success":false,"code":"SEND_SMS_FAIL"},"key":{}}`, 200, ErrProvider, false},
		{"missing-success", `{"response":{"code":"SENDING_SMS_INPROGRESS"},"key":{}}`, 200, ErrMalformedResponse, true},
		{"missing-response", `{"success":true,"code":"SENDING_SMS_INPROGRESS","key":{}}`, 200, ErrMalformedResponse, true},
		{"missing-key", `{"response":{"success":true}}`, 200, ErrMalformedResponse, true},
		{"null-key", `{"response":{"success":true},"key":null}`, 200, ErrMalformedResponse, true},
		{"invalid-success", `{"response":{"success":"true"},"key":{}}`, 200, ErrMalformedResponse, true},
		{"null", "null", 200, ErrMalformedResponse, true},
		{"empty", "", 200, ErrMalformedResponse, true},
		{"html-200", "<html>bad</html>", 200, ErrMalformedResponse, true},
		{"html-503", "<html>bad</html>", 503, ErrProvider, true},
		{"truncated", `{"response":`, 200, ErrMalformedResponse, true},
		{"trailing-json", acceptedBody + `{}`, 200, ErrMalformedResponse, true},
		{"http-error-success", acceptedBody, 500, ErrProvider, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, key, err := parseService([]byte(tc.body), tc.status, "sms")
			if tc.kind == nil {
				if err != nil || !result.Success {
					t.Fatal(err)
				}
				if tc.name == "unknown-code" && (result.Code != "FUTURE_CODE" || key.RangeKey != "r") {
					t.Fatal("unknown fields lost")
				}
				return
			}
			var e *Error
			if !errors.Is(err, tc.kind) || !errors.As(err, &e) || e.HTTPStatus != tc.status || e.OutcomeUnknown != tc.unknown {
				t.Fatalf("bad response error: %#v", err)
			}
			if strings.Contains(err.Error(), "sensitive") {
				t.Fatal("provider text leaked")
			}
			if tc.body == rejectedBody && (e.Code != CodeRequestFail || e.Message != "sensitive-provider-echo" || e.Detail != "sensitive-provider-detail") {
				t.Fatal("provider fields lost")
			}
		})
	}
}

func TestNoServiceReplay(t *testing.T) {
	for _, topup := range []bool{false, true} {
		for _, status := range []int{401, 403, 429, 500, 503, 307, 308} {
			t.Run(fmt.Sprintf("topup=%t/status=%d", topup, status), func(t *testing.T) {
				var services, target atomic.Int32
				destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { target.Add(1); io.WriteString(w, acceptedBody) }))
				defer destination.Close()
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/api/v1/Connect/token" {
						io.WriteString(w, authBody("fake", 3600))
						return
					}
					services.Add(1)
					w.Header().Set("Location", destination.URL)
					w.WriteHeader(status)
					io.WriteString(w, rejectedBody)
				}))
				defer server.Close()
				var originalRedirect atomic.Int32
				httpClient := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { originalRedirect.Add(1); return nil }}
				cfg := Config{ClientID: "fake", ClientSecret: "fake", BaseURL: server.URL, HTTPClient: httpClient}
				c := mustClient(t, cfg)
				err := send(c, context.Background(), topup)
				c.Close()
				var e *Error
				if !errors.As(err, &e) || e.HTTPStatus != status {
					t.Fatalf("incorrect error: %v", err)
				}
				if services.Load() != 1 || target.Load() != 0 || originalRedirect.Load() != 0 {
					t.Fatal("POST replay or redirect occurred")
				}
				if httpClient.Timeout != 10*time.Second {
					t.Fatal("caller HTTP client changed")
				}
				if err := httpClient.CheckRedirect(nil, nil); err != nil || originalRedirect.Load() != 1 {
					t.Fatal("caller redirect handler changed")
				}
			})
		}
	}
}

func TestAuthenticationRedirect(t *testing.T) {
	var target atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { target.Add(1) }))
	defer destination.Close()
	for _, status := range []int{301, 302, 303, 307, 308} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", destination.URL)
				w.WriteHeader(status)
			}))
			defer server.Close()
			_, err := New(context.Background(), Config{ClientID: "fake", ClientSecret: "fake", BaseURL: server.URL})
			if !errors.Is(err, ErrAuthentication) {
				t.Fatal("auth redirect accepted")
			}
		})
	}
	if target.Load() != 0 {
		t.Fatal("credentials followed redirect")
	}
}

func TestConnectionLossNoReplay(t *testing.T) {
	for _, topup := range []bool{false, true} {
		t.Run(fmt.Sprintf("topup=%t", topup), func(t *testing.T) {
			var services atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/Connect/token" {
					io.WriteString(w, authBody("fake", 3600))
					return
				}
				services.Add(1)
				conn, _, err := w.(http.Hijacker).Hijack()
				if err != nil {
					t.Error(err)
					return
				}
				conn.Close()
			}))
			defer server.Close()
			c := mustClient(t, Config{ClientID: "fake", ClientSecret: "fake", BaseURL: server.URL})
			defer c.Close()
			err := send(c, context.Background(), topup)
			var e *Error
			if !errors.Is(err, ErrTransport) || !errors.As(err, &e) || !e.OutcomeUnknown || services.Load() != 1 {
				t.Fatalf("bad connection-loss handling: %v", err)
			}
			if errors.Unwrap(e) == nil {
				t.Fatal("transport cause lost")
			}
		})
	}
}

type trackedBody struct {
	io.Reader
	closed *atomic.Int32
}

func (b *trackedBody) Close() error { b.closed.Add(1); return nil }

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestResponseCleanupAndSafeErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		reader io.Reader
		kind   error
	}{
		{"success", 200, strings.NewReader(acceptedBody), nil},
		{"provider", 400, strings.NewReader(rejectedBody), ErrProvider},
		{"malformed", 200, strings.NewReader("bad"), ErrMalformedResponse},
		{"oversize", 200, strings.NewReader(strings.Repeat("x", maxResponseBytes+1)), ErrMalformedResponse},
		{"read-error", 200, brokenReader{}, ErrTransport},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var closed atomic.Int32
			c := mustClient(t, mockConfig(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/api/v1/Connect/token" {
					return mockResponse(200, authBody("fake", 3600)), nil
				}
				if r.GetBody != nil {
					t.Error("service body replayable")
				}
				return &http.Response{StatusCode: tc.status, Header: make(http.Header), Body: &trackedBody{Reader: tc.reader, closed: &closed}}, nil
			}))
			defer c.Close()
			err := send(c, context.Background(), false)
			if tc.kind == nil && err != nil || tc.kind != nil && !errors.Is(err, tc.kind) {
				t.Fatalf("bad error: %v", err)
			}
			if closed.Load() != 1 {
				t.Fatal("body not closed exactly once")
			}
			if err != nil && strings.Contains(err.Error(), "sensitive") {
				t.Fatal("unsafe error text")
			}
		})
	}
	secretError := errors.New("fake-secret fake-access 2012345678 fabricated-message")
	wrapped := authenticationError(&Error{Kind: ErrTransport, cause: secretError})
	if !errors.Is(wrapped, secretError) || strings.Contains(wrapped.Error(), "fake-secret") {
		t.Fatal("underlying cause lost or logged")
	}
}

func TestContextTimeouts(t *testing.T) {
	for _, topup := range []bool{false, true} {
		t.Run(fmt.Sprintf("topup=%t", topup), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var services int
				c := mustClient(t, mockConfig(func(r *http.Request) (*http.Response, error) {
					deadline, ok := r.Context().Deadline()
					if !ok || deadline.Sub(time.Now()) > RequestTimeout {
						t.Error("unbounded HTTP call")
					}
					if r.URL.Path == "/api/v1/Connect/token" {
						return mockResponse(200, authBody("fake", 3600)), nil
					}
					services++
					<-r.Context().Done()
					return nil, r.Context().Err()
				}))
				defer c.Close()
				started := time.Now()
				err := send(c, context.Background(), topup)
				var e *Error
				if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, ErrTransport) || !errors.As(err, &e) || !e.OutcomeUnknown || services != 1 || time.Since(started) != RequestTimeout {
					t.Fatalf("bad timeout: %v", err)
				}
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				err = send(c, ctx, topup)
				if !errors.Is(err, context.Canceled) || services != 1 {
					t.Fatal("pre-canceled call submitted")
				}
				if errors.As(err, &e) && e.OutcomeUnknown {
					t.Fatal("pre-submission failure marked unknown")
				}
				ctx, cancel = context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				started = time.Now()
				err = send(c, ctx, topup)
				if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) != time.Second || services != 2 {
					t.Fatal("caller timeout not honored")
				}
			})
		})
	}
	synctest.Test(t, func(t *testing.T) {
		cfg := mockConfig(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })
		start := time.Now()
		c, err := New(context.Background(), cfg)
		if c != nil || !errors.Is(err, ErrAuthentication) || !errors.Is(err, context.DeadlineExceeded) || time.Since(start) != RequestTimeout {
			t.Fatal("constructor timeout incorrect")
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		c, err = New(ctx, cfg)
		if c != nil || !errors.Is(err, context.Canceled) {
			t.Fatal("constructor cancellation incorrect")
		}
	})
}

func TestCallerTransportNotClosed(t *testing.T) {
	rt := &ownedCheckTransport{}
	c := mustClient(t, Config{ClientID: "fake", ClientSecret: "fake", HTTPClient: &http.Client{Transport: rt}})
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if rt.closed.Load() != 0 {
		t.Fatal("caller transport closed")
	}
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.Close(); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	for _, topup := range []bool{false, true} {
		if err := send(c, context.Background(), topup); !errors.Is(err, ErrClosed) {
			t.Fatal("service allowed after closure")
		}
	}
	if err := c.RefreshToken(context.Background()); !errors.Is(err, ErrClosed) {
		t.Fatal("refresh allowed after closure")
	}
}

type ownedCheckTransport struct{ closed atomic.Int32 }

func (*ownedCheckTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return mockResponse(200, authBody("fake", 3600)), nil
}
func (t *ownedCheckTransport) CloseIdleConnections() { t.closed.Add(1) }
