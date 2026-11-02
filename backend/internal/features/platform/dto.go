package platform

type IdentityInput struct {
	Subject string   `json:"subject" binding:"required"`
	Scopes  []string `json:"scopes"`
	Enabled bool     `json:"enabled"`
	Reason  string   `json:"reason" binding:"required"`
}
type Identity struct {
	ApplicationID string   `json:"applicationId"`
	ClientID      string   `json:"clientId"`
	Subject       string   `json:"subject"`
	Scopes        []string `json:"scopes"`
	Enabled       bool     `json:"enabled"`
}
type UserAccess struct {
	ApplicationID string   `json:"applicationId"`
	Subject       string   `json:"subject"`
	Roles         []string `json:"roles"`
	Permissions   []string `json:"permissions"`
}
