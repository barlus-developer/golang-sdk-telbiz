package telbiz

import (
	"net/http"
	"time"
)

// DefaultBaseURL is the documented Telbiz API origin.
const DefaultBaseURL = "https://api.telbiz.la"

// RequestTimeout bounds every SDK HTTP request, including injected clients.
const RequestTimeout = 30 * time.Second

// Config is copied by New and must not be modified concurrently with New.
type Config struct {
	ClientID     string
	ClientSecret string
	BaseURL      string
	HTTPClient   *http.Client
	// RefreshBefore defaults to 60 seconds and is clamped to half the lifetime.
	RefreshBefore time.Duration
}

// SMSTitle is a documented sender heading. Its zero value is invalid.
type SMSTitle string

const (
	TitleOTP       SMSTitle = "OTP"
	TitleInfo      SMSTitle = "Info"
	TitlePromotion SMSTitle = "Promotion"
	TitleNews      SMSTitle = "News"
)

// Documented provider codes. Unknown codes are also preserved.
const (
	CodeSuccessOperation       = "SUCCESS_OPERATION"
	CodeSendingSMSInProgress   = "SENDING_SMS_INPROGRESS"
	CodeTopupBalanceInProgress = "TOPUP_BALANCE_INPROGRESS"
	CodeUnsupportedHeader      = "UNSUPPORTED_HEADER"
	CodeSendSMSFail            = "SEND_SMS_FAIL"
	CodeRequestFail            = "REQUEST_FAIL"
)

// SMSRequest submits text without trimming or normalizing its contents.
type SMSRequest struct {
	Title   SMSTitle `json:"title"`
	Phone   string   `json:"phone"`
	Message string   `json:"message"`
}

// TopupRequest submits whole Lao kip. Amount must be at least 10,000.
type TopupRequest struct {
	Phone  string `json:"phone"`
	Amount int64  `json:"amount"`
}

// Result contains provider fields. Message and Detail may contain sensitive text.
// Success indicates acceptance, not final delivery or airtime credit.
type Result struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Success bool   `json:"success"`
	Detail  string `json:"detail"`
}

// TransactionKey preserves provider keys, including empty strings.
// Uniqueness and lookup semantics are unspecified by the provider.
type TransactionKey struct {
	PartitionKey string `json:"partitionKey"`
	RangeKey     string `json:"rangeKey"`
}

// SMSResponse represents an accepted SMS submission.
type SMSResponse struct {
	Response Result         `json:"response"`
	Key      TransactionKey `json:"key"`
}

// TopupResponse represents an accepted airtime submission.
type TopupResponse struct {
	Response Result         `json:"response"`
	Key      TransactionKey `json:"key"`
}
