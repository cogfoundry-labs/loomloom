package cmd

type publishMarketListingRequest struct {
	ListingID         string `json:"listingId,omitempty"`
	DisplayName       string `json:"displayName"`
	Description       string `json:"description,omitempty"`
	TaskFixedFeeT     int64  `json:"taskFixedFeeT"`
	TemplateID        string `json:"templateId"`
	TemplateVersionID string `json:"templateVersionId"`
}
