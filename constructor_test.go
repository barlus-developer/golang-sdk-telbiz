package telbiz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func TestNewEnvironmentAndExplicitConfig(t *testing.T) {
	t.Setenv("TELBIZ_CLIENT_ID", "fake-env-id")
	t.Setenv("TELBIZ_CLIENT_SECRET", "fake-env-secret")
	c := New()
	defer c.Close()
	// Capture the environment at construction, not at first use.
	t.Setenv("TELBIZ_CLIENT_ID", "changed-fake-id")
	t.Setenv("TELBIZ_CLIENT_SECRET", "changed-fake-secret")
	var auths atomic.Int32
	c.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		auths.Add(1)
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["clientID"] != "fake-env-id" || body["secret"] != "fake-env-secret" {
			t.Error("environment credentials were not captured")
		}
		return mockResponse(200, authBody("fake", 3600)), nil
	})}
	if auths.Load() != 0 || c.lifecycleStarted || c.cfg.Timeout != RequestTimeout {
		t.Fatal("New started authentication or did not apply defaults")
	}
	if err := c.RefreshToken(context.Background()); err != nil || auths.Load() != 1 {
		t.Fatal("first explicit refresh did not authenticate", err)
	}
	cfg := mockConfig(func(r *http.Request) (*http.Response, error) {
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["clientID"] != "fake-explicit-id" || body["secret"] != "fake-explicit-secret" {
			t.Error("explicit credentials changed")
		}
		return mockResponse(200, authBody("fake", 3600)), nil
	})
	cfg.ClientID, cfg.ClientSecret = "fake-explicit-id", "fake-explicit-secret"
	explicit := New(cfg)
	defer explicit.Close()
	cfg.ClientID, cfg.ClientSecret = "changed", "changed"
	if err := explicit.RefreshToken(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, cfg := range []Config{{}, {ClientID: "fake-id"}, {ClientSecret: "fake-secret"}} {
		lazy := New(cfg)
		if err := lazy.RefreshToken(context.Background()); !errors.Is(err, ErrValidation) {
			t.Fatal("explicit config fell back to environment", err)
		}
		lazy.Close()
		if eager, err := NewWithContext(context.Background(), cfg); eager != nil || !errors.Is(err, ErrValidation) {
			t.Fatal("explicit eager config fell back to environment", err)
		}
	}
	t.Setenv("TELBIZ_CLIENT_ID", "")
	t.Setenv("TELBIZ_CLIENT_SECRET", "")
	missing := New()
	defer missing.Close()
	if err := missing.RefreshToken(context.Background()); !errors.Is(err, ErrValidation) {
		t.Fatal("missing environment did not produce inspectable error", err)
	}
}

func TestNewDeferredConfigurationErrors(t *testing.T) {
	base := mockConfig(func(*http.Request) (*http.Response, error) {
		t.Error("invalid configuration reached network")
		return mockResponse(200, authBody("fake", 3600)), nil
	})
	for _, field := range []string{"ClientID", "ClientSecret", "BaseURL", "Timeout", "RefreshBefore", "Config"} {
		t.Run(field, func(t *testing.T) {
			cfg := base
			switch field {
			case "ClientID":
				cfg.ClientID = " "
			case "ClientSecret":
				cfg.ClientSecret = ""
			case "BaseURL":
				cfg.BaseURL = "https://example.com/prefix"
			case "Timeout":
				cfg.Timeout = -time.Second
			case "RefreshBefore":
				cfg.RefreshBefore = -time.Second
			}
			configs := []Config{cfg}
			if field == "Config" {
				configs = append(configs, cfg)
			}
			c := New(configs...)
			defer c.Close()
			for _, op := range []func() error{
				func() error { return send(c, context.Background(), false) },
				func() error { return send(c, context.Background(), true) },
				func() error { return c.RefreshToken(context.Background()) },
			} {
				err := op()
				var e *Error
				if !errors.Is(err, ErrValidation) || !errors.As(err, &e) || e.Field != field || e.OutcomeUnknown {
					t.Fatalf("configuration error not inspectable: %#v", err)
				}
			}
			if c.lifecycleStarted || c.flight != nil {
				t.Fatal("invalid configuration started workers")
			}
			if field != "Config" {
				if eager, err := NewWithContext(context.Background(), cfg); eager != nil || !errors.Is(err, ErrValidation) {
					t.Fatal("eager constructor did not return configuration error", err)
				}
			}
			c.Close()
			if err := send(c, context.Background(), false); !errors.Is(err, ErrClosed) {
				t.Fatal("closed invalid client did not report closure", err)
			}
		})
	}
}

func TestNewConcurrentFirstUseAndIdleRenewal(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var auths, services atomic.Int32
		release := make(chan struct{})
		cfg := mockConfig(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == "/api/v1/Connect/token" {
				n := auths.Add(1)
				if n == 1 {
					select {
					case <-release:
					case <-r.Context().Done():
						return nil, r.Context().Err()
					}
				}
				return mockResponse(200, authBody(fmt.Sprintf("fake-%d", n), 3600)), nil
			}
			services.Add(1)
			return mockResponse(200, acceptedBody), nil
		})
		c := New(cfg)
		defer c.Close()
		time.Sleep(2 * time.Hour)
		synctest.Wait()
		if auths.Load() != 0 {
			t.Fatal("unused lazy client authenticated")
		}
		firstCtx, cancel := context.WithCancel(context.Background())
		first := make(chan error, 1)
		go func() { first <- send(c, firstCtx, false) }()
		synctest.Wait()
		results := make(chan error, 30)
		for i := range 30 {
			go func() {
				if i%3 == 0 {
					results <- c.RefreshToken(context.Background())
				} else {
					results <- send(c, context.Background(), i%3 == 1)
				}
			}()
		}
		synctest.Wait()
		cancel()
		synctest.Wait()
		if err := <-first; !errors.Is(err, context.Canceled) {
			t.Fatal("first caller cancellation lost", err)
		}
		if auths.Load() != 1 || services.Load() != 0 {
			t.Fatal("duplicate first authentication or early submission")
		}
		close(release)
		synctest.Wait()
		for range 30 {
			if err := <-results; err != nil {
				t.Fatal("canceled first caller broke shared auth", err)
			}
		}
		if auths.Load() != 1 || services.Load() != 20 {
			t.Fatal("incorrect first-use request counts")
		}
		time.Sleep(3540*time.Second - time.Nanosecond)
		synctest.Wait()
		if auths.Load() != 1 {
			t.Fatal("lazy token renewed early")
		}
		time.Sleep(time.Nanosecond)
		synctest.Wait()
		if auths.Load() != 2 {
			t.Fatal("idle renewal tied to canceled first caller")
		}
	})
}

func TestNewAuthenticationFailureAndRecovery(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var auths, services atomic.Int32
		c := New(mockConfig(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == "/api/v1/Connect/token" {
				if auths.Add(1) == 1 {
					return mockResponse(401, rejectedBody), nil
				}
				return mockResponse(200, authBody("fake", 3600)), nil
			}
			services.Add(1)
			return mockResponse(200, acceptedBody), nil
		}))
		defer c.Close()
		for _, topup := range []bool{false, true} {
			err := send(c, context.Background(), topup)
			var e *Error
			if !errors.Is(err, ErrAuthentication) || !errors.Is(err, ErrProvider) || !errors.As(err, &e) || e.HTTPStatus != 401 || e.OutcomeUnknown {
				t.Fatal("lazy authentication failure not inspectable", err)
			}
		}
		time.Sleep(time.Hour)
		synctest.Wait()
		if auths.Load() != 1 || services.Load() != 0 || c.lifecycleStarted {
			t.Fatal("failed first auth submitted service or started lifecycle")
		}
		if err := c.RefreshToken(context.Background()); err != nil {
			t.Fatal("explicit recovery failed", err)
		}
		if err := send(c, context.Background(), false); err != nil || auths.Load() != 2 || services.Load() != 1 {
			t.Fatal("recovered client unusable", err)
		}
	})
}

func TestNewTransientFirstAuthenticationCooldown(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var calls atomic.Int32
		c := New(mockConfig(func(*http.Request) (*http.Response, error) {
			if calls.Add(1) == 1 {
				return mockResponse(503, rejectedBody), nil
			}
			return mockResponse(200, authBody("fake", 3600)), nil
		}))
		defer c.Close()
		if err := c.RefreshToken(context.Background()); !errors.Is(err, ErrAuthentication) {
			t.Fatal("first transient failure missing", err)
		}
		for range 10 {
			if err := c.RefreshToken(context.Background()); !errors.Is(err, ErrAuthentication) {
				t.Fatal("first-use cooldown bypassed", err)
			}
		}
		time.Sleep(time.Second)
		synctest.Wait()
		if calls.Load() != 1 || c.lifecycleStarted {
			t.Fatal("failed first-use auth started idle retries")
		}
		if err := c.RefreshToken(context.Background()); err != nil || calls.Load() != 2 {
			t.Fatal("first-use retry after cooldown failed", err)
		}
	})
}

func TestNewInvalidOrCanceledOperationDoesNotAuthenticate(t *testing.T) {
	c := New(mockConfig(func(*http.Request) (*http.Response, error) {
		t.Error("invalid or pre-canceled operation authenticated")
		return mockResponse(200, authBody("fake", 3600)), nil
	}))
	defer c.Close()
	if _, err := c.SendSMS(context.Background(), SMSRequest{}); !errors.Is(err, ErrValidation) {
		t.Fatal("invalid SMS accepted", err)
	}
	if _, err := c.Topup(context.Background(), TopupRequest{}); !errors.Is(err, ErrValidation) {
		t.Fatal("invalid top-up accepted", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, topup := range []bool{false, true} {
		if err := send(c, ctx, topup); !errors.Is(err, context.Canceled) {
			t.Fatal("pre-canceled service ignored cancellation", err)
		}
	}
	if err := c.RefreshToken(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("pre-canceled refresh ignored cancellation", err)
	}
	if c.lifecycleStarted || c.flight != nil {
		t.Fatal("invalid/canceled operation started workers")
	}
}

func TestConfigTimeoutAuthentication(t *testing.T) {
	for _, eager := range []bool{false, true} {
		t.Run(fmt.Sprintf("eager=%t", eager), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var calls atomic.Int32
				cfg := mockConfig(func(r *http.Request) (*http.Response, error) {
					calls.Add(1)
					deadline, ok := r.Context().Deadline()
					if !ok || deadline.Sub(time.Now()) != 15*time.Second {
						t.Error("custom auth timeout absent")
					}
					<-r.Context().Done()
					return nil, r.Context().Err()
				})
				cfg.Timeout = 15 * time.Second
				start := time.Now()
				var err error
				if eager {
					var c *Client
					c, err = NewWithContext(context.Background(), cfg)
					if c != nil {
						c.Close()
						t.Fatal("failed eager constructor returned client")
					}
				} else {
					c := New(cfg)
					defer c.Close()
					err = send(c, context.Background(), false)
				}
				if !errors.Is(err, ErrAuthentication) || !errors.Is(err, context.DeadlineExceeded) || calls.Load() != 1 || time.Since(start) != cfg.Timeout {
					t.Fatal("custom auth timeout not honored", err)
				}
				if cfg.HTTPClient.Timeout != 0 || cfg.HTTPClient.CheckRedirect != nil {
					t.Fatal("injected client mutated")
				}
			})
		})
	}
	synctest.Test(t, func(t *testing.T) {
		cfg := mockConfig(func(r *http.Request) (*http.Response, error) {
			<-r.Context().Done()
			return nil, r.Context().Err()
		})
		cfg.Timeout = 15 * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		start := time.Now()
		if c, err := NewWithContext(ctx, cfg); c != nil || !errors.Is(err, context.DeadlineExceeded) || time.Since(start) != time.Second {
			t.Fatal("eager initialization ignored earlier caller deadline", err)
		}
		c := New(cfg)
		defer c.Close()
		ctx, cancel = context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		start = time.Now()
		if err := send(c, ctx, true); !errors.Is(err, context.DeadlineExceeded) || time.Since(start) != time.Second {
			t.Fatal("lazy waiter ignored earlier caller deadline", err)
		}
		c.mu.Lock()
		inFlight := c.flight != nil
		c.mu.Unlock()
		if !inFlight {
			t.Fatal("earlier caller deadline canceled shared auth")
		}
	})
}

func TestConfigTimeoutServicesAndRenewal(t *testing.T) {
	for _, topup := range []bool{false, true} {
		t.Run(fmt.Sprintf("topup=%t", topup), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var auths, services atomic.Int32
				var bound time.Duration = 15 * time.Second
				cfg := mockConfig(func(r *http.Request) (*http.Response, error) {
					deadline, ok := r.Context().Deadline()
					want := bound
					if r.URL.Path == "/api/v1/Connect/token" {
						want = 15 * time.Second
						if !ok || deadline.Sub(time.Now()) != want {
							t.Error("auth/renewal timeout incorrect")
						}
						if auths.Add(1) == 1 {
							return mockResponse(200, authBody("fake", 3600)), nil
						}
					} else {
						services.Add(1)
						if !ok || deadline.Sub(time.Now()) != want {
							t.Error("service timeout incorrect")
						}
					}
					<-r.Context().Done()
					return nil, r.Context().Err()
				})
				cfg.Timeout = bound
				c := New(cfg)
				defer c.Close()
				start := time.Now()
				tokenStart := start
				err := send(c, context.Background(), topup)
				var e *Error
				if !errors.Is(err, context.DeadlineExceeded) || !errors.As(err, &e) || !e.OutcomeUnknown || time.Since(start) != bound || services.Load() != 1 {
					t.Fatal("custom service timeout/replay behavior incorrect", err)
				}
				bound = time.Second
				ctx, cancel := context.WithTimeout(context.Background(), bound)
				start = time.Now()
				err = send(c, ctx, topup)
				cancel()
				if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) != bound || services.Load() != 2 {
					t.Fatal("earlier service deadline ignored", err)
				}
				time.Sleep(3540*time.Second - time.Since(tokenStart))
				synctest.Wait()
				if auths.Load() != 2 {
					t.Fatal("idle renewal missing")
				}
				time.Sleep(15 * time.Second)
				synctest.Wait()
				c.mu.Lock()
				renewErr := c.lastErr
				c.mu.Unlock()
				if !errors.Is(renewErr, context.DeadlineExceeded) || auths.Load() != 2 {
					t.Fatal("renewal timeout not honored", renewErr)
				}
			})
		})
	}
}

func TestNewCloseBeforeAndDuringInitialization(t *testing.T) {
	for _, during := range []bool{false, true} {
		t.Run(fmt.Sprintf("during=%t", during), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var auths atomic.Int32
				c := New(mockConfig(func(r *http.Request) (*http.Response, error) {
					if r.URL.Path != "/api/v1/Connect/token" {
						t.Error("service submitted during shutdown")
					}
					auths.Add(1)
					<-r.Context().Done()
					return nil, r.Context().Err()
				}))
				results := make(chan error, 20)
				if during {
					for i := range 20 {
						go func() { results <- send(c, context.Background(), i%2 == 0) }()
					}
					synctest.Wait()
				}
				var wg sync.WaitGroup
				for range 20 {
					wg.Add(1)
					go func() { defer wg.Done(); c.Close() }()
				}
				wg.Wait()
				if during {
					for range 20 {
						if err := <-results; !errors.Is(err, ErrClosed) {
							t.Fatal("Close did not unblock lazy initialization", err)
						}
					}
				}
				if err := c.RefreshToken(context.Background()); !errors.Is(err, ErrClosed) {
					t.Fatal("closed client authenticated", err)
				}
				for _, topup := range []bool{false, true} {
					if err := send(c, context.Background(), topup); !errors.Is(err, ErrClosed) {
						t.Fatal("closed client submitted", err)
					}
				}
				time.Sleep(2 * time.Hour)
				synctest.Wait()
				want := int32(0)
				if during {
					want = 1
				}
				if auths.Load() != want || c.lifecycleStarted || c.token.generation != 0 {
					t.Fatal("shutdown installed token or left workers")
				}
			})
		})
	}
}

func TestNewFirstUseRacesClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		for i := range 100 {
			success := i%2 == 0
			var auths atomic.Int32
			c := New(mockConfig(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/api/v1/Connect/token" {
					return mockResponse(200, acceptedBody), nil
				}
				auths.Add(1)
				if success {
					return mockResponse(200, authBody("fake", 3600)), nil
				}
				<-r.Context().Done()
				return nil, r.Context().Err()
			}))
			var wg sync.WaitGroup
			for i := range 12 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if i%3 == 0 {
						c.Close()
					} else if err := send(c, context.Background(), i%3 == 1); err != nil && !errors.Is(err, ErrClosed) {
						t.Error("unexpected initialization race error", err)
					}
				}()
			}
			wg.Wait()
			if auths.Load() > 1 || (!success && (c.lifecycleStarted || c.token.generation != 0)) {
				t.Fatal("initialization/shutdown race left active work")
			}
			callsAtClose := auths.Load()
			time.Sleep(time.Hour)
			synctest.Wait()
			if auths.Load() != callsAtClose {
				t.Fatal("renewal continued after initialization/shutdown race")
			}
		}
	})
}
