package telbiz

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func TestIdleRenewalSchedule(t *testing.T) {
	for _, tc := range []struct {
		name     string
		expire   int
		lead, at time.Duration
	}{
		{"default-hour", 3600, 0, 3540 * time.Second},
		{"configured-lead", 3600, 2 * time.Minute, 3480 * time.Second},
		{"variable-lifetime", 200, 0, 140 * time.Second},
		{"short-lifetime", 10, 0, 5 * time.Second},
		{"one-second", 1, 0, 500 * time.Millisecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var calls atomic.Int32
				cfg := mockConfig(func(r *http.Request) (*http.Response, error) {
					if r.URL.Path == "/api/v1/Connect/token" {
						calls.Add(1)
						return mockResponse(200, authBody(fmt.Sprintf("fake-%d", calls.Load()), tc.expire)), nil
					}
					if r.Header.Get("Authorization") != "bearer fake-2" {
						t.Error("renewed token not attached")
					}
					return mockResponse(200, acceptedBody), nil
				})
				cfg.RefreshBefore = tc.lead
				initCtx, cancel := context.WithCancel(context.Background())
				c, err := New(initCtx, cfg)
				if err != nil {
					t.Fatal(err)
				}
				defer c.Close()
				cancel()
				synctest.Wait()
				time.Sleep(tc.at - time.Nanosecond)
				synctest.Wait()
				if calls.Load() != 1 {
					t.Fatal("renewed too early")
				}
				time.Sleep(time.Nanosecond)
				synctest.Wait()
				if calls.Load() != 2 {
					t.Fatalf("idle renewal absent: %d", calls.Load())
				}
				if err := send(c, context.Background(), false); err != nil {
					t.Fatal(err)
				}
			})
		})
	}
}

func TestRenewalUsesNewLifetime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var calls atomic.Int32
		c := mustClient(t, mockConfig(func(*http.Request) (*http.Response, error) {
			calls.Add(1)
			expire := 200
			if calls.Load() > 1 {
				expire = 20
			}
			return mockResponse(200, authBody("fake", expire)), nil
		}))
		defer c.Close()
		time.Sleep(140 * time.Second)
		synctest.Wait()
		if calls.Load() != 2 {
			t.Fatal("first renewal missing")
		}
		time.Sleep(10*time.Second - time.Nanosecond)
		synctest.Wait()
		if calls.Load() != 2 {
			t.Fatal("old lifetime used incorrectly")
		}
		time.Sleep(time.Nanosecond)
		synctest.Wait()
		if calls.Load() != 3 {
			t.Fatal("new lifetime not used")
		}
	})
}

func TestSlowAuthenticationScheduling(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var calls atomic.Int32
		cfg := mockConfig(func(*http.Request) (*http.Response, error) {
			calls.Add(1)
			time.Sleep(9 * time.Second)
			return mockResponse(200, authBody("fake", 10)), nil
		})
		start := time.Now()
		c := mustClient(t, cfg)
		defer c.Close()
		synctest.Wait()
		c.mu.Lock()
		expiry := c.token.expiresAt
		c.mu.Unlock()
		if expiry != start.Add(10*time.Second) {
			t.Fatal("response latency extended lifetime")
		}
		time.Sleep(minScheduleDelay - time.Nanosecond)
		synctest.Wait()
		if calls.Load() != 1 {
			t.Fatal("immediate renewal loop")
		}
		time.Sleep(time.Nanosecond)
		synctest.Wait()
		if calls.Load() != 2 {
			t.Fatal("bounded prompt renewal absent")
		}
	})
	synctest.Test(t, func(t *testing.T) {
		c, err := New(context.Background(), mockConfig(func(*http.Request) (*http.Response, error) {
			time.Sleep(2 * time.Second)
			return mockResponse(200, authBody("fake", 1)), nil
		}))
		if c != nil || !errors.Is(err, ErrMalformedResponse) {
			t.Fatal("already exhausted token installed")
		}
	})
}

func TestSharedRenewalAndWaitCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var auths, services atomic.Int32
		release := make(chan struct{})
		c := mustClient(t, mockConfig(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == "/api/v1/Connect/token" {
				auths.Add(1)
				if auths.Load() == 2 {
					select {
					case <-release:
					case <-r.Context().Done():
						return nil, r.Context().Err()
					}
				}
				return mockResponse(200, authBody(fmt.Sprintf("fake-%d", auths.Load()), 10)), nil
			}
			services.Add(1)
			return mockResponse(200, acceptedBody), nil
		}))
		defer c.Close()
		time.Sleep(5 * time.Second)
		synctest.Wait()
		if auths.Load() != 2 {
			t.Fatal("background renewal did not start")
		}
		if err := send(c, context.Background(), false); err != nil || services.Load() != 1 {
			t.Fatal("valid old token unavailable during renewal")
		}
		results := make(chan error, 40)
		for range 10 {
			go func() { results <- c.RefreshToken(context.Background()) }()
		}
		time.Sleep(5*time.Second + time.Nanosecond)
		synctest.Wait()
		for range 10 {
			go func() { results <- send(c, context.Background(), false) }()
		}
		ctx, cancel := context.WithCancel(context.Background())
		canceled := make(chan error, 1)
		go func() { canceled <- send(c, ctx, false) }()
		synctest.Wait()
		cancel()
		synctest.Wait()
		if err := <-canceled; !errors.Is(err, context.Canceled) {
			t.Fatal("caller cancellation lost")
		}
		if auths.Load() != 2 || services.Load() != 1 {
			t.Fatal("duplicate auth or expired-token submission")
		}
		close(release)
		synctest.Wait()
		for range 20 {
			if err := <-results; err != nil {
				t.Fatal(err)
			}
		}
		if auths.Load() != 2 || services.Load() != 11 {
			t.Fatalf("coordination incorrect: auth=%d service=%d", auths.Load(), services.Load())
		}
	})
}

func TestManualCancellationDoesNotCancelSharedRenewal(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var calls atomic.Int32
		release := make(chan struct{})
		c := mustClient(t, mockConfig(func(r *http.Request) (*http.Response, error) {
			calls.Add(1)
			if calls.Load() > 1 {
				select {
				case <-release:
				case <-r.Context().Done():
					return nil, r.Context().Err()
				}
			}
			return mockResponse(200, authBody("fake", 3600)), nil
		}))
		defer c.Close()
		ctx, cancel := context.WithCancel(context.Background())
		first := make(chan error, 1)
		second := make(chan error, 1)
		go func() { first <- c.RefreshToken(ctx) }()
		synctest.Wait()
		go func() { second <- c.RefreshToken(context.Background()) }()
		synctest.Wait()
		cancel()
		synctest.Wait()
		if err := <-first; !errors.Is(err, context.Canceled) {
			t.Fatal("manual cancellation ignored")
		}
		if calls.Load() != 2 {
			t.Fatal("duplicate shared refresh")
		}
		close(release)
		synctest.Wait()
		if err := <-second; err != nil {
			t.Fatal("shared work canceled", err)
		}
	})
}

func TestTransientFailureBackoff(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var auths, services atomic.Int32
		c := mustClient(t, mockConfig(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == "/api/v1/Connect/token" {
				auths.Add(1)
				if auths.Load() >= 2 && auths.Load() <= 4 {
					return mockResponse(503, rejectedBody), nil
				}
				return mockResponse(200, authBody(fmt.Sprintf("fake-%d", auths.Load()), 4)), nil
			}
			services.Add(1)
			if r.Header.Get("Authorization") != "bearer fake-1" {
				t.Error("old token erased during failed renewal")
			}
			return mockResponse(200, acceptedBody), nil
		}))
		defer c.Close()
		time.Sleep(2 * time.Second)
		synctest.Wait()
		if auths.Load() != 2 {
			t.Fatal("renewal missing")
		}
		if err := send(c, context.Background(), false); err != nil {
			t.Fatal(err)
		}
		if err := c.RefreshToken(context.Background()); !errors.Is(err, ErrAuthentication) || auths.Load() != 2 {
			t.Fatal("manual refresh bypassed cooldown")
		}
		time.Sleep(time.Second)
		synctest.Wait()
		if auths.Load() != 3 {
			t.Fatal("first backoff not one second")
		}
		time.Sleep(time.Second)
		synctest.Wait()
		if auths.Load() != 4 {
			t.Fatal("backoff failed to account for expiry")
		}
		for range 100 {
			if err := send(c, context.Background(), false); !errors.Is(err, ErrAuthentication) {
				t.Fatal("expired token submitted")
			}
		}
		var wg sync.WaitGroup
		for range 30 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := send(c, context.Background(), true); !errors.Is(err, ErrAuthentication) {
					t.Error("expired token submitted")
				}
			}()
		}
		wg.Wait()
		synctest.Wait()
		if auths.Load() != 4 || services.Load() != 1 {
			t.Fatal("sequential/concurrent failure storm bypassed cooldown")
		}
		time.Sleep(4*time.Second - time.Nanosecond)
		synctest.Wait()
		if auths.Load() != 4 {
			t.Fatal("expiry caused tight retries")
		}
		time.Sleep(time.Nanosecond)
		synctest.Wait()
		if auths.Load() != 5 {
			t.Fatal("retry did not recover")
		}
	})
}

func TestBackoffCapAndAuthenticationPolicy(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var calls atomic.Int32
		c := mustClient(t, mockConfig(func(r *http.Request) (*http.Response, error) {
			calls.Add(1)
			if calls.Load() == 1 {
				return mockResponse(200, authBody("fake", 1)), nil
			}
			return mockResponse(429, rejectedBody), nil
		}))
		defer c.Close()
		time.Sleep(500 * time.Millisecond)
		synctest.Wait()
		if calls.Load() != 2 {
			t.Fatal("first renewal missing")
		}
		// Remaining lifetime clamps the first delay to 500 ms. After expiry,
		// exponential backoff continues and is capped at 30 seconds.
		for _, delay := range []time.Duration{500 * time.Millisecond, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second, 30 * time.Second} {
			old := calls.Load()
			time.Sleep(delay - time.Nanosecond)
			synctest.Wait()
			if calls.Load() != old {
				t.Fatal("retry too early")
			}
			time.Sleep(time.Nanosecond)
			synctest.Wait()
			if calls.Load() != old+1 {
				t.Fatalf("incorrect delay %v", delay)
			}
		}
	})
	for _, tc := range []struct {
		status    int
		kind      error
		transient bool
	}{{408, ErrProvider, true}, {429, ErrProvider, true}, {500, ErrProvider, true}, {503, ErrMalformedResponse, true}, {401, ErrProvider, false}, {403, ErrProvider, false}, {400, ErrProvider, false}, {200, ErrProvider, false}, {200, ErrMalformedResponse, false}, {0, ErrTransport, true}} {
		err := authenticationError(&Error{Kind: tc.kind, HTTPStatus: tc.status})
		if transientAuthentication(err) != tc.transient {
			t.Errorf("wrong transient policy for %d", tc.status)
		}
	}
	if transientAuthentication(authenticationError(&Error{Kind: ErrTransport, cause: context.Canceled})) {
		t.Fatal("cancellation retried")
	}
}

func TestTerminalFailureAndExplicitRecovery(t *testing.T) {
	for _, status := range []int{200, 400, 401, 403} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var auths, services atomic.Int32
				c := mustClient(t, mockConfig(func(r *http.Request) (*http.Response, error) {
					if r.URL.Path == "/api/v1/Connect/token" {
						auths.Add(1)
						if auths.Load() == 2 {
							return mockResponse(status, `{"code":"UNKNOWN_REJECTION","success":false}`), nil
						}
						return mockResponse(200, authBody("fake", 10)), nil
					}
					services.Add(1)
					return mockResponse(200, acceptedBody), nil
				}))
				defer c.Close()
				time.Sleep(5 * time.Second)
				synctest.Wait()
				if auths.Load() != 2 {
					t.Fatal("renewal missing")
				}
				if err := send(c, context.Background(), false); err != nil {
					t.Fatal("valid old token lost")
				}
				time.Sleep(time.Hour)
				synctest.Wait()
				if auths.Load() != 2 {
					t.Fatal("definitive rejection caused churn")
				}
				err := send(c, context.Background(), false)
				var e *Error
				if !errors.Is(err, ErrAuthentication) || !errors.As(err, &e) || e.Code != "UNKNOWN_REJECTION" || services.Load() != 1 {
					t.Fatal("terminal auth not visible")
				}
				if err := c.RefreshToken(context.Background()); err != nil {
					t.Fatal("manual recovery failed", err)
				}
				if auths.Load() != 3 {
					t.Fatal("manual recovery absent")
				}
				if err := send(c, context.Background(), false); err != nil || services.Load() != 2 {
					t.Fatal("service did not recover")
				}
			})
		})
	}
}

func TestLate401AndFutureRefresh(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var auths, services atomic.Int32
		release := make(chan struct{})
		c := mustClient(t, mockConfig(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == "/api/v1/Connect/token" {
				auths.Add(1)
				return mockResponse(200, authBody(fmt.Sprintf("fake-%d", auths.Load()), 3600)), nil
			}
			services.Add(1)
			if services.Load() == 1 {
				if r.Header.Get("Authorization") != "bearer fake-1" {
					t.Error("wrong initial token")
				}
				<-release
				return mockResponse(401, rejectedBody), nil
			}
			if r.Header.Get("Authorization") != "bearer fake-2" {
				t.Error("new token lost after old 401")
			}
			return mockResponse(200, acceptedBody), nil
		}))
		defer c.Close()
		first := make(chan error, 1)
		go func() { first <- send(c, context.Background(), false) }()
		synctest.Wait()
		if err := c.RefreshToken(context.Background()); err != nil {
			t.Fatal(err)
		}
		close(release)
		synctest.Wait()
		if err := <-first; !errors.Is(err, ErrProvider) {
			t.Fatal("401 not returned")
		}
		if err := send(c, context.Background(), false); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if auths.Load() != 2 || services.Load() != 2 {
			t.Fatal("late 401 caused extra auth or service replay")
		}
	})
	synctest.Test(t, func(t *testing.T) {
		var auths, services atomic.Int32
		c := mustClient(t, mockConfig(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == "/api/v1/Connect/token" {
				auths.Add(1)
				return mockResponse(200, authBody(fmt.Sprintf("fake-%d", auths.Load()), 3600)), nil
			}
			services.Add(1)
			if services.Load() == 1 {
				return mockResponse(401, rejectedBody), nil
			}
			if r.Header.Get("Authorization") != "bearer fake-2" {
				t.Error("invalidated token reused")
			}
			return mockResponse(200, acceptedBody), nil
		}))
		defer c.Close()
		if err := send(c, context.Background(), false); !errors.Is(err, ErrProvider) {
			t.Fatal("401 not returned")
		}
		synctest.Wait()
		if services.Load() != 1 || auths.Load() != 2 {
			t.Fatal("401 replayed instead of renewing for future calls")
		}
		if err := send(c, context.Background(), false); err != nil {
			t.Fatal(err)
		}
	})
}

func TestCloseInFlightAndConcurrentWaiters(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var calls atomic.Int32
		c := mustClient(t, mockConfig(func(r *http.Request) (*http.Response, error) {
			calls.Add(1)
			if calls.Load() == 1 {
				return mockResponse(200, authBody("fake", 3600)), nil
			}
			<-r.Context().Done()
			return nil, r.Context().Err()
		}))
		results := make(chan error, 21)
		for range 20 {
			go func() { results <- c.RefreshToken(context.Background()) }()
		}
		synctest.Wait()
		if calls.Load() != 2 {
			t.Fatal("duplicate in-flight auth")
		}
		var wg sync.WaitGroup
		for range 20 {
			wg.Add(1)
			go func() { defer wg.Done(); c.Close() }()
		}
		wg.Wait()
		synctest.Wait()
		for range 20 {
			if err := <-results; !errors.Is(err, ErrClosed) {
				t.Fatal("Close did not unblock waiter", err)
			}
		}
		time.Sleep(2 * time.Hour)
		synctest.Wait()
		if calls.Load() != 2 {
			t.Fatal("renewal continued after Close")
		}
	})
}

func TestCloseAllowsAlreadyStartedServiceToFinish(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var services atomic.Int32
		release := make(chan struct{})
		c := mustClient(t, mockConfig(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == "/api/v1/Connect/token" {
				return mockResponse(200, authBody("fake", 3600)), nil
			}
			services.Add(1)
			<-release
			if r.Context().Err() != nil {
				t.Error("Close canceled caller's service context")
			}
			return mockResponse(200, acceptedBody), nil
		}))
		result := make(chan error, 1)
		go func() { result <- send(c, context.Background(), false) }()
		synctest.Wait()
		c.Close()
		if err := send(c, context.Background(), true); !errors.Is(err, ErrClosed) {
			t.Fatal("new submission after Close")
		}
		close(release)
		synctest.Wait()
		if err := <-result; err != nil || services.Load() != 1 {
			t.Fatal("started service could not finish", err)
		}
	})
}

func TestAuthenticationAttemptBodiesClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var attempts, closed atomic.Int32
		c := mustClient(t, mockConfig(func(r *http.Request) (*http.Response, error) {
			n := attempts.Add(1)
			status, body := 200, authBody("fake", 3600)
			if n == 2 {
				status, body = 503, rejectedBody
			}
			return &http.Response{StatusCode: status, Header: make(http.Header), Body: &trackedBody{Reader: strings.NewReader(body), closed: &closed}}, nil
		}))
		defer c.Close()
		if err := c.RefreshToken(context.Background()); !errors.Is(err, ErrAuthentication) {
			t.Fatal("expected transient failure")
		}
		time.Sleep(time.Second)
		synctest.Wait()
		if attempts.Load() != 3 || closed.Load() != 3 {
			t.Fatal("authentication response body leak")
		}
	})
}

func TestServiceRejectionsDoNotInvalidateTokens(t *testing.T) {
	for _, status := range []int{200, 403, 429, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var auths, services atomic.Int32
				c := mustClient(t, mockConfig(func(r *http.Request) (*http.Response, error) {
					if r.URL.Path == "/api/v1/Connect/token" {
						auths.Add(1)
						return mockResponse(200, authBody("fake", 3600)), nil
					}
					services.Add(1)
					return mockResponse(status, rejectedBody), nil
				}))
				defer c.Close()
				if err := send(c, context.Background(), false); !errors.Is(err, ErrProvider) {
					t.Fatal("rejection not returned")
				}
				synctest.Wait()
				if auths.Load() != 1 || services.Load() != 1 {
					t.Fatal("generic rejection invalidated token or replayed service")
				}
			})
		})
	}
}

func TestServiceAdmissionChecksExpiryAndClosure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var services atomic.Int32
		c := mustClient(t, mockConfig(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == "/api/v1/Connect/token" {
				return mockResponse(200, authBody("fake", 3600)), nil
			}
			services.Add(1)
			return mockResponse(200, acceptedBody), nil
		}))
		expired := timeInfo{expiresAt: time.Now()}
		_, _, err := c.post(context.Background(), "sms", "/api/v1/SMSService/newtransaction", "fake", SMSRequest{}, true, expired)
		if !errors.Is(err, errTokenExpired) || services.Load() != 0 {
			t.Fatal("expired token passed admission")
		}
		c.Close()
		_, _, err = c.post(context.Background(), "sms", "/api/v1/SMSService/newtransaction", "fake", SMSRequest{}, true, timeInfo{expiresAt: time.Now().Add(time.Hour)})
		if !errors.Is(err, ErrClosed) || services.Load() != 0 {
			t.Fatal("closed client passed admission")
		}
	})
}

// The authentication response can be usable during parsing but expire before
// the state lock is acquired for installation. Script just those clock reads.
type installationClock struct {
	realClock
	start time.Time
	reads atomic.Int32
}

func (c *installationClock) Now() time.Time {
	if c.reads.Add(1) >= 4 {
		return c.start.Add(2 * time.Second)
	}
	return c.start
}

func TestExpiredTokenNotInstalled(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := mustClient(t, mockConfig(func(*http.Request) (*http.Response, error) {
			return mockResponse(200, authBody("fake", 1)), nil
		}))
		defer c.Close()
		c.mu.Lock()
		c.terminal = true
		c.mu.Unlock()
		c.signal()
		synctest.Wait()
		c.mu.Lock()
		oldGeneration := c.token.generation
		c.clock = &installationClock{start: time.Now()}
		c.mu.Unlock()
		if err := c.RefreshToken(context.Background()); !errors.Is(err, ErrMalformedResponse) {
			t.Fatal("expired token installation succeeded")
		}
		c.mu.Lock()
		generation := c.token.generation
		c.mu.Unlock()
		if generation != oldGeneration {
			t.Fatal("expired token replaced prior token")
		}
	})
}
