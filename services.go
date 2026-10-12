package telbiz

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

func validPhone(phone string) bool {
	if len(phone) != 10 || !strings.HasPrefix(phone, "20") {
		return false
	}
	for i := range phone {
		if phone[i] < '0' || phone[i] > '9' {
			return false
		}
	}
	return true
}

// SendSMS validates input, obtains a usable token, and submits once. Success is
// acceptance, not confirmed delivery. No response or failure triggers replay.
func (c *Client) SendSMS(ctx context.Context, req SMSRequest) (*SMSResponse, error) {
	if ctx == nil {
		return nil, validation("sms", "Context")
	}
	if err := c.checkOpen(); err != nil {
		return nil, err
	}
	switch req.Title {
	case TitleOTP, TitleInfo, TitlePromotion, TitleNews:
	default:
		return nil, validation("sms", "Title")
	}
	if !validPhone(req.Phone) {
		return nil, validation("sms", "Phone")
	}
	if strings.TrimSpace(req.Message) == "" {
		return nil, validation("sms", "Message")
	}
	result, key, err := c.submit(ctx, "sms", "/api/v1/SMSService/newtransaction", req)
	if err != nil {
		return nil, err
	}
	return &SMSResponse{Response: result, Key: key}, nil
}

// Topup validates input and submits whole LAK once. Success is acceptance, not
// proof of airtime credit. Amount must be at least 10,000.
func (c *Client) Topup(ctx context.Context, req TopupRequest) (*TopupResponse, error) {
	if ctx == nil {
		return nil, validation("topup", "Context")
	}
	if err := c.checkOpen(); err != nil {
		return nil, err
	}
	if !validPhone(req.Phone) {
		return nil, validation("topup", "Phone")
	}
	if req.Amount < 10000 {
		return nil, validation("topup", "Amount")
	}
	result, key, err := c.submit(ctx, "topup", "/api/v1/TopupService/newtransaction", req)
	if err != nil {
		return nil, err
	}
	return &TopupResponse{Response: result, Key: key}, nil
}

func (c *Client) checkOpen() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ErrClosed
	}
	return nil
}

func (c *Client) submit(ctx context.Context, op, path string, req any) (Result, TransactionKey, error) {
	for {
		tok, err := c.acquire(ctx)
		if err != nil {
			return Result{}, TransactionKey{}, err
		}
		data, status, err := c.post(ctx, op, path, tok.value, req, true, tok.timeInfo)
		if errors.Is(err, errTokenExpired) {
			continue
		} // No HTTP call occurred.
		if status == 401 {
			c.invalidate(tok.generation)
		}
		if err != nil {
			return Result{}, TransactionKey{}, err
		}
		return parseService(data, status, op)
	}
}

func parseService(data []byte, status int, op string) (Result, TransactionKey, error) {
	var envelope struct {
		wireResult
		Response *wireResult     `json:"response"`
		Key      *TransactionKey `json:"key"`
	}
	err := json.Unmarshal(data, &envelope)
	w := envelope.wireResult
	if envelope.Response != nil {
		w = *envelope.Response
	}
	unknown := err != nil || w.Success == nil || *w.Success
	if status < 200 || status >= 300 {
		return Result{}, TransactionKey{}, responseError(ErrProvider, op, status, w, unknown)
	}
	if err != nil || w.Success == nil {
		return Result{}, TransactionKey{}, responseError(ErrMalformedResponse, op, status, w, true)
	}
	if !*w.Success {
		return Result{}, TransactionKey{}, responseError(ErrProvider, op, status, w, false)
	}
	if envelope.Response == nil || envelope.Key == nil {
		return Result{}, TransactionKey{}, responseError(ErrMalformedResponse, op, status, w, true)
	}
	return w.result(), *envelope.Key, nil
}
