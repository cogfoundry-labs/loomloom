package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// readInitialSubscriptionTiersFile parses prices sent with a payment-mode change.
// Existing prices are updated through set-tier and its expected revision.
func readInitialSubscriptionTiersFile(path string) ([]initialSubscriptionTierRequest, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("--subscription-tiers-file requires a JSON file path")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read --subscription-tiers-file: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var tiers []initialSubscriptionTierRequest
	if err := decoder.Decode(&tiers); err != nil {
		return nil, fmt.Errorf("--subscription-tiers-file must contain a JSON array of tiers: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("--subscription-tiers-file must contain exactly one JSON array")
	}
	if len(tiers) == 0 {
		return nil, fmt.Errorf("--subscription-tiers-file must contain at least one monthly or yearly tier; null and empty arrays are not initial prices")
	}

	seen := make(map[string]bool)
	for i := range tiers {
		tier := &tiers[i]
		tier.TierType = strings.TrimSpace(tier.TierType)
		if tier.TierType != "monthly" && tier.TierType != "yearly" {
			return nil, fmt.Errorf("subscription tier %d: tierType must be monthly or yearly", i+1)
		}
		if seen[tier.TierType] {
			return nil, fmt.Errorf("subscription tier %d: duplicate %s tier", i+1, tier.TierType)
		}
		seen[tier.TierType] = true
		if tier.Enabled == nil {
			return nil, fmt.Errorf("subscription tier %d: enabled must be an explicit boolean", i+1)
		}
		if tier.Price == nil {
			return nil, fmt.Errorf("subscription tier %d: price is required; use amount \"0\" for a free tier", i+1)
		}
		tier.Price.Currency = strings.ToUpper(strings.TrimSpace(tier.Price.Currency))
		if tier.Price.Currency != "CNY" {
			return nil, fmt.Errorf("subscription tier %d: price.currency must be CNY", i+1)
		}
		tier.Price.Amount = strings.TrimSpace(tier.Price.Amount)
		// Initial prices allow four written decimal places; the shared money
		// parser accepts seven for other CLI amounts.
		whole, fraction, hasFraction := strings.Cut(tier.Price.Amount, ".")
		if !allDigits(whole) || (hasFraction && (!allDigits(fraction) || len(fraction) > 4)) {
			return nil, fmt.Errorf("subscription tier %d: price.amount must be a non-negative decimal string with at most four decimal places", i+1)
		}
		if _, err := parseMoneyAmountT(tier.Price.Amount); err != nil {
			return nil, fmt.Errorf("subscription tier %d: invalid price.amount: %w", i+1, err)
		}
	}
	// The Server remains authoritative for whether the resulting subscription
	// has an enabled tier, and returns subscription_tier_required when it does not.
	return tiers, nil
}
