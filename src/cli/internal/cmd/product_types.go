package cmd

type publishMarketListingRequest struct {
	Category                        *string                          `json:"category,omitempty"`
	Value                           *marketPublicValue               `json:"value,omitempty"`
	UpdateSummary                   string                           `json:"updateSummary,omitempty"`
	ListingID                       string                           `json:"listingId,omitempty"`
	DisplayName                     string                           `json:"displayName"`
	Description                     string                           `json:"description,omitempty"`
	TaskFixedFeeT                   int64                            `json:"taskFixedFeeT"`
	TemplateID                      string                           `json:"templateId"`
	TemplateVersionID               string                           `json:"templateVersionId"`
	SkillPackage                    *listingSkillPackageSelection    `json:"skillPackage,omitempty"`
	PayPerUse                       *bool                            `json:"payPerUse,omitempty"`
	Subscription                    *bool                            `json:"subscription,omitempty"`
	SubscriptionTiers               []initialSubscriptionTierRequest `json:"subscriptionTiers,omitempty"`
	ExpectedPaymentSettingsRevision *int64                           `json:"expectedPaymentSettingsRevision,omitempty"`
}

type updateMarketListingPublicProfileRequest struct {
	DisplayName   string             `json:"displayName"`
	Description   string             `json:"description"`
	Category      *string            `json:"category,omitempty"`
	Value         *marketPublicValue `json:"value,omitempty"`
	UpdateSummary string             `json:"updateSummary,omitempty"`
}

type initialSubscriptionTierRequest struct {
	TierType string         `json:"tierType"`
	Price    *moneyResponse `json:"price"`
	Enabled  *bool          `json:"enabled"`
}

type listingPaymentSettingsRequest struct {
	PayPerUse         bool                             `json:"payPerUse"`
	Subscription      bool                             `json:"subscription"`
	ExpectedRevision  int64                            `json:"expectedRevision"`
	SubscriptionTiers []initialSubscriptionTierRequest `json:"subscriptionTiers,omitempty"`
}

type listingSkillPackageSelection struct {
	Mode                 string `json:"mode"`
	ExpectedArchiveHash  string `json:"expectedArchiveHash,omitempty"`
	ExpectedValidationID string `json:"expectedValidationId,omitempty"`
}
