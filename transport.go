package telbiz

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const maxResponseBytes = 1 << 20

type wireResult struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Success *bool  `json:"success"`
	Detail  string `json:"detail"`
}

func (w wireResult) result() Result {
	return Result{Code: w.Code, Message: w.Message, Success: w.Success != nil && *w.Success, Detail: w.Detail}
}

func baseURL(raw string) (string, error) {
	if raw == "" {
		return DefaultBaseURL, nil
	}
	u, err := url.Parse(raw)
	if err != nil || u == nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || strings.Contains(raw, "#") || u.Opaque != "" || (u.Path != "" && u.Path != "/") {
		return "", validation("new", "BaseURL")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", validation("new", "BaseURL")
	}
	if strings.ContainsAny(u.Host, " \\\t\r\n") || strings.HasSuffix(u.Host, ":") {
		return "", validation("new", "BaseURL")
	}
	if port := u.Port(); port != "" {
		p, err := strconv.Atoi(port)
		if err != nil || p < 1 || p > 65535 {
			return "", validation("new", "BaseURL")
		}
	}
	if u.Scheme == "http" {
		ip := net.ParseIP(u.Hostname())
		if u.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return "", validation("new", "BaseURL")
		}
	}
	return strings.TrimSuffix(u.String(), "/"), nil
}

// post performs exactly one Do. Redirects are disabled on a private client copy.
// A service is admitted before Do under the state lock; Close prevents later
// admissions. Already admitted calls retain their own bounded context.
func (c *Client) post(ctx context.Context, op, path, token string, body any, service bool, expiry timeInfo) ([]byte, int, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, 0, &Error{Kind: ErrValidation, Operation: op, cause: err}
	}
	ctx, cancel := context.WithTimeout(ctx, RequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(b))
	if err != nil {
		return nil, 0, &Error{Kind: ErrValidation, Operation: op, cause: err}
	}
	// Do not provide a replayable body to net/http or injected transports.
	req.GetBody = nil
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "bearer "+token)
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, &Error{Kind: ErrTransport, Operation: op, cause: err}
	}
	if service {
		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return nil, 0, ErrClosed
		}
		if !c.clock.Now().Before(expiry.expiresAt) {
			c.mu.Unlock()
			return nil, 0, errTokenExpired
		}
		c.mu.Unlock()
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, &Error{Kind: ErrTransport, Operation: op, OutcomeUnknown: service, cause: err}
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, resp.StatusCode, &Error{Kind: ErrTransport, Operation: op, HTTPStatus: resp.StatusCode, OutcomeUnknown: service, cause: err}
	}
	if len(data) > maxResponseBytes {
		return nil, resp.StatusCode, &Error{Kind: ErrMalformedResponse, Operation: op, HTTPStatus: resp.StatusCode, OutcomeUnknown: service}
	}
	return data, resp.StatusCode, nil
}

func responseError(kind error, op string, status int, w wireResult, unknown bool) error {
	return &Error{Kind: kind, Operation: op, HTTPStatus: status, Code: w.Code, Message: w.Message, Detail: w.Detail, OutcomeUnknown: unknown}
}
