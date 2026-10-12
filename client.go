package telbiz

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var errTokenExpired = errors.New("telbiz: token expired before submission")

const minScheduleDelay = 100 * time.Millisecond

type timeInfo struct {
	expiresAt time.Time
	refreshAt time.Time
}

type tokenState struct {
	timeInfo
	value      string
	generation uint64
	authStatus int
	authCode   string
}

type authFlight struct {
	done chan struct{}
	err  error
}

// Client is safe for concurrent use. Construct it with New; the zero value is
// not usable. Reuse one client, then call Close to stop its background work.
type Client struct {
	mu          sync.Mutex
	cfg         Config
	base        string
	http        *http.Client
	owned       *http.Transport
	clock       clock
	ctx         context.Context
	cancel      context.CancelFunc
	wake        chan struct{}
	wg          sync.WaitGroup
	closeDone   chan struct{}
	closed      bool
	token       tokenState
	flight      *authFlight
	lastErr     error
	nextAttempt time.Time
	failures    uint
	terminal    bool
}

// New validates configuration and authenticates immediately. Its context governs
// initialization only; a successful client's lifecycle ends through Close.
func New(ctx context.Context, cfg Config) (*Client, error) {
	return newClient(ctx, cfg, realClock{})
}

func newClient(ctx context.Context, cfg Config, clk clock) (*Client, error) {
	if ctx == nil {
		return nil, validation("new", "Context")
	}
	if strings.TrimSpace(cfg.ClientID) == "" {
		return nil, validation("new", "ClientID")
	}
	if strings.TrimSpace(cfg.ClientSecret) == "" {
		return nil, validation("new", "ClientSecret")
	}
	if cfg.RefreshBefore < 0 {
		return nil, validation("new", "RefreshBefore")
	}
	if cfg.RefreshBefore == 0 {
		cfg.RefreshBefore = time.Minute
	}
	base, err := baseURL(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	c := &Client{cfg: cfg, base: base, clock: clk, wake: make(chan struct{}, 1), closeDone: make(chan struct{})}
	if cfg.HTTPClient != nil {
		copyClient := *cfg.HTTPClient
		c.http = &copyClient
	} else {
		c.owned = &http.Transport{
			Proxy:             http.ProxyFromEnvironment,
			DialContext:       (&net.Dialer{Timeout: RequestTimeout, KeepAlive: 30 * time.Second}).DialContext,
			ForceAttemptHTTP2: true, MaxIdleConns: 100, IdleConnTimeout: 90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second, ExpectContinueTimeout: time.Second,
		}
		c.http = &http.Client{Transport: c.owned}
	}
	c.http.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	tok, err := c.authenticate(ctx)
	if err == nil {
		err = c.checkTokenInstallation(tok)
	}
	if err == nil && ctx.Err() != nil {
		err = authenticationError(ctx.Err())
	}
	if err != nil {
		if c.owned != nil {
			c.owned.CloseIdleConnections()
		}
		return nil, err
	}
	tok.generation = 1
	c.token = tok
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.wg.Add(1)
	go c.lifecycle()
	return c, nil
}

func (c *Client) authenticate(ctx context.Context) (tokenState, error) {
	started := c.clock.Now()
	body := struct {
		ClientID  string `json:"clientID"`
		Secret    string `json:"secret"`
		GrantType string `json:"grantType"`
	}{c.cfg.ClientID, c.cfg.ClientSecret, "client_credentials"}
	data, status, err := c.post(ctx, "authenticate", "/api/v1/Connect/token", "", body, false, timeInfo{})
	if err != nil {
		return tokenState{}, authenticationError(err)
	}
	var wire struct {
		wireResult
		AccessToken string          `json:"accessToken"`
		Expire      json.RawMessage `json:"expire"`
	}
	decodeErr := json.Unmarshal(data, &wire)
	if status < 200 || status >= 300 {
		return tokenState{}, authenticationError(responseError(ErrProvider, "authenticate", status, wire.wireResult, false))
	}
	if decodeErr != nil || wire.Success == nil {
		return tokenState{}, authenticationError(responseError(ErrMalformedResponse, "authenticate", status, wire.wireResult, false))
	}
	if !*wire.Success {
		return tokenState{}, authenticationError(responseError(ErrProvider, "authenticate", status, wire.wireResult, false))
	}
	seconds, err := strconv.ParseInt(strings.TrimSpace(string(wire.Expire)), 10, 64)
	if err != nil || seconds <= 0 || seconds > math.MaxInt64/int64(time.Second) || strings.TrimSpace(wire.AccessToken) == "" || strings.ContainsAny(wire.AccessToken, "\r\n") {
		return tokenState{}, authenticationError(responseError(ErrMalformedResponse, "authenticate", status, wire.wireResult, false))
	}
	lifetime := time.Duration(seconds) * time.Second
	expires := started.Add(lifetime)
	if !c.clock.Now().Before(expires) {
		return tokenState{}, authenticationError(responseError(ErrMalformedResponse, "authenticate", status, wire.wireResult, false))
	}
	lead := c.cfg.RefreshBefore
	if lead > lifetime/2 {
		lead = lifetime / 2
	}
	refresh := expires.Add(-lead)
	if minimum := c.clock.Now().Add(minScheduleDelay); refresh.Before(minimum) {
		refresh = minimum
	}
	return tokenState{value: wire.AccessToken, authStatus: status, authCode: wire.Code, timeInfo: timeInfo{expiresAt: expires, refreshAt: refresh}}, nil
}

func (c *Client) checkTokenInstallation(tok tokenState) error {
	if !c.clock.Now().Before(tok.expiresAt) {
		return authenticationError(responseError(ErrMalformedResponse, "authenticate", tok.authStatus, wireResult{Code: tok.authCode}, false))
	}
	return nil
}

// RefreshToken forces renewal or joins an operation already in progress. Caller
// cancellation cancels only its wait. Transient failure cooldown still applies.
func (c *Client) RefreshToken(ctx context.Context) error {
	if ctx == nil {
		return validation("refresh", "Context")
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return ErrClosed
	}
	if err := ctx.Err(); err != nil {
		c.mu.Unlock()
		return authenticationError(err)
	}
	f, err := c.beginRefreshLocked(true)
	c.mu.Unlock()
	if err != nil {
		return err
	}
	return c.wait(ctx, f)
}

func (c *Client) beginRefreshLocked(force bool) (*authFlight, error) {
	if c.closed {
		return nil, ErrClosed
	}
	if c.flight != nil {
		return c.flight, nil
	}
	if c.lastErr != nil && ((!force && c.terminal) || (!c.terminal && c.clock.Now().Before(c.nextAttempt))) {
		return nil, c.lastErr
	}
	f := &authFlight{done: make(chan struct{})}
	c.flight = f
	c.wg.Add(1)
	go c.runRefresh(f)
	return f, nil
}

func (c *Client) runRefresh(f *authFlight) {
	defer c.wg.Done()
	tok, err := c.authenticate(c.ctx)
	c.mu.Lock()
	if err == nil {
		err = c.checkTokenInstallation(tok)
	}
	if c.closed {
		f.err = ErrClosed
	} else if err == nil {
		tok.generation = c.token.generation + 1
		c.token = tok
		c.lastErr, c.terminal, c.failures = nil, false, 0
		c.nextAttempt = time.Time{}
	} else {
		c.lastErr, f.err = err, err
		c.terminal = !transientAuthentication(err)
		if !c.terminal {
			delay := time.Second << min(c.failures, 5)
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}
			if c.failures < 5 {
				c.failures++
			}
			now := c.clock.Now()
			if remaining := c.token.expiresAt.Sub(now); remaining > 0 && remaining < delay {
				delay = max(remaining, minScheduleDelay)
			}
			c.nextAttempt = now.Add(delay)
		}
	}
	c.flight = nil
	close(f.done)
	c.mu.Unlock()
	c.signal()
}

func transientAuthentication(err error) bool {
	var e *Error
	if !errors.As(err, &e) {
		return false
	}
	// HTTP policy is SDK policy; the manual does not specify status mappings.
	if e.HTTPStatus == 408 || e.HTTPStatus == 429 || e.HTTPStatus >= 500 {
		return true
	}
	if e.HTTPStatus != 0 {
		return false
	}
	return errors.Is(err, ErrTransport) && !errors.Is(err, context.Canceled)
}

func (c *Client) wait(ctx context.Context, f *authFlight) error {
	select {
	case <-ctx.Done():
		return authenticationError(ctx.Err())
	case <-c.ctx.Done():
		return ErrClosed
	case <-f.done:
		return f.err
	}
}

func (c *Client) acquire(ctx context.Context) (tokenState, error) {
	for {
		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return tokenState{}, ErrClosed
		}
		if err := ctx.Err(); err != nil {
			c.mu.Unlock()
			return tokenState{}, authenticationError(err)
		}
		if c.clock.Now().Before(c.token.expiresAt) {
			tok := c.token
			c.mu.Unlock()
			return tok, nil
		}
		f, err := c.beginRefreshLocked(false)
		c.mu.Unlock()
		if err != nil {
			return tokenState{}, err
		}
		if err := c.wait(ctx, f); err != nil {
			return tokenState{}, err
		}
	}
}

func (c *Client) signal() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

func (c *Client) lifecycle() {
	defer c.wg.Done()
	for {
		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return
		}
		if c.flight != nil || c.terminal {
			c.mu.Unlock()
			select {
			case <-c.ctx.Done():
				return
			case <-c.wake:
			}
			continue
		}
		due := c.token.refreshAt
		if c.lastErr != nil {
			due = c.nextAttempt
		}
		delay := due.Sub(c.clock.Now())
		if delay <= 0 {
			_, _ = c.beginRefreshLocked(false)
			c.mu.Unlock()
			continue
		}
		t := c.clock.NewTimer(delay)
		c.mu.Unlock()
		select {
		case <-c.ctx.Done():
			t.Stop()
			return
		case <-c.wake:
			t.Stop()
		case <-t.C():
		}
	}
}

func (c *Client) invalidate(generation uint64) {
	c.mu.Lock()
	if !c.closed && c.token.generation == generation {
		c.token.expiresAt = time.Time{}
		c.token.refreshAt = c.clock.Now()
	}
	c.mu.Unlock()
	c.signal()
}

// Close stops and joins client-owned renewal work. Calls admitted before Close
// may finish under their own contexts. Caller-owned transports are never closed.
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		done := c.closeDone
		c.mu.Unlock()
		<-done
		return nil
	}
	c.closed = true
	c.cancel()
	c.mu.Unlock()
	c.wg.Wait()
	if c.owned != nil {
		c.owned.CloseIdleConnections()
	}
	close(c.closeDone)
	return nil
}
