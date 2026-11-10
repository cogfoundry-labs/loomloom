package presentation

import (
	"encoding/json"
	"regexp"
	"strings"
)

const insufficientBalance = "insufficient balance; recharge and retry"

var balanceError = regexp.MustCompile(`(?i)(?:^|:\s*)(?:insufficient balance|余额不足)(?::|：|$)`)

// Match explicitly labelled monetary estimates, never arbitrary numbers such as
// row indexes, task counts, HTTP statuses or model identifiers.
var estimateAmount = regexp.MustCompile(`(?i)((?:estimated(?:[_ ]?(?:total|execution|model|api|buyer))*[_ ]?(?:cost|payable)(?:T)?|预估(?:费用|金额|成本))\s*[:=：]?\s*)(?:[A-Z]{3}\s*|[¥￥$€]\s*)?-?\d+(?:\.\d+)?(?:\s*(?:[A-Z]{3}\b|元))?`)

func ErrorMessage(message string) string {
	if balanceError.MatchString(message) {
		return insufficientBalance
	}
	return estimateAmount.ReplaceAllString(message, "${1}[hidden]")
}

// ErrorBody formats a display-only copy; RequestError.Body remains available to
// status/code based recovery and never becomes a successful response.
func ErrorBody(body string) string {
	var response map[string]any
	decoder := json.NewDecoder(strings.NewReader(body))
	decoder.UseNumber()
	if !json.Valid([]byte(body)) || decoder.Decode(&response) != nil || response == nil {
		if balanceError.MatchString(body) {
			return body
		}
		return ErrorMessage(body)
	}
	original, _ := json.Marshal(response)
	message, _ := response["error"].(string)
	if message == "" {
		message, _ = response["message"].(string)
	}
	code, _ := response["code"].(string)
	if strings.EqualFold(code, "insufficient_balance") || balanceError.MatchString(message) {
		return body
	}
	HideEstimates(response)
	if message, ok := response["message"].(string); ok {
		response["message"] = ErrorMessage(message)
	}
	if details, ok := response["details"]; ok {
		HideEstimates(details)
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		return ErrorMessage(body)
	}
	// Keep unrelated errors byte-for-byte, including their useful diagnostics.
	if string(original) == string(encoded) {
		return body
	}
	return string(encoded)
}
