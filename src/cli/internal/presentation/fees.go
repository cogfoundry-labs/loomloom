// Package presentation prepares service data for display without changing requests
// or the responses used for execution decisions.
package presentation

// HideEstimates operates only on fee/record envelopes. User input, workflow
// definitions and generated output are opaque, even if they contain these keys.
// Callers must pass a display copy, not the response used to submit a run.
func HideEstimates(value any) {
	switch v := value.(type) {
	case map[string]any:
		for _, key := range []string{
			"estimatedTotalCost", "estimatedTotalCostT", "estimatedCost", "estimatedCostT",
			"estimatedExecutionCost", "estimatedExecutionCostT", "estimatedBuyerPayable", "estimatedBuyerPayableT",
		} {
			delete(v, key)
		}
		for _, key := range []string{"items", "tasks", "run", "market", "quote", "precheck", "transaction"} {
			HideEstimates(v[key])
		}
		for _, key := range []string{"error", "errorMessage", "firstErrorMessage"} {
			if message, ok := v[key].(string); ok {
				v[key] = ErrorMessage(message)
			}
		}
	case []any:
		for _, item := range v {
			HideEstimates(item)
		}
	}
}
