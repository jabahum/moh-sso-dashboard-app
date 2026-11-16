package metrics

import "time"

type TotalUsersResponse struct {
	TotalUsers int64 `json:"total_users"`
}

type DisabledUsersResponse struct {
	DisabledUsers int64 `json:"disabled_users"`
}

type ActiveUsersTodayResponse struct {
	ActiveUsersToday int64 `json:"active_users_today"`
}

type ActiveUsersThisWeekResponse struct {
	ActiveUsersThisWeek int64 `json:"active_users_this_week"`
}

type FailedLoginsResponse struct {
	FailedLogins int64 `json:"failed_logins"`
}

type TotalClientsResponse struct {
	TotalClients int64 `json:"total_clients"`
}

type LoginCountResponse struct {
	LoginCount int64 `json:"login_count"`
}

type LoginTrendPoint struct {
	Day   time.Time `json:"day"`
	Total int64     `json:"total"`
}

type LoginTrendByDayPoint struct {
	Day          time.Time `json:"day"`
	SuccessCount int64     `json:"success_count"`
	FailureCount int64     `json:"failure_count"`
}

type SuspiciousLogin struct {
	ID        string     `json:"id"`
	UserID    *string    `json:"user_id,omitempty"`
	Ip        *string    `json:"ip,omitempty"`
	Country   *string    `json:"country,omitempty"`
	City      *string    `json:"city,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
}

type MostAccessedClient struct {
	ClientID   *string `json:"client_id,omitempty"`
	LoginCount int64   `json:"login_count"`
}

type RecentClient struct {
	ID           string    `json:"id"`
	ClientID     string    `json:"client_id"`
	Name         string    `json:"name"`
	Description  *string   `json:"description,omitempty"`
	BaseUrl      *string   `json:"base_url,omitempty"`
	Icon         *string   `json:"icon,omitempty"`
	PublicClient bool      `json:"public_client"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ActiveUsersPerClient struct {
	ClientID    *string `json:"client_id,omitempty"`
	ActiveUsers int64   `json:"active_users"`
}

type UserClientUsagePoint struct {
	ClientID   *string `json:"client_id,omitempty"`
	LoginCount int64   `json:"login_count"`
}

type LastLoginResponse struct {
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

type OverviewSystemStats struct {
	TotalUsers          int64 `json:"total_users"`
	DisabledUsers       int64 `json:"disabled_users"`
	ActiveUsersToday    int64 `json:"active_users_today"`
	ActiveUsersThisWeek int64 `json:"active_users_this_week"`
}

type OverviewClientStats struct {
	TotalClients   int64                  `json:"total_clients"`
	EnabledClients int64                  `json:"enabled_clients"`
	ActiveToday    []ActiveUsersPerClient `json:"active_today"`
	RecentClients  []RecentClient         `json:"recent_clients"`
}

type OverviewSecurityStats struct {
	FailedLogins   int64 `json:"failed_logins"`
	ActiveSessions int64 `json:"active_sessions"`
}

type OverviewTrends struct {
	LoginTrend30Days    []LoginTrendByDayPoint `json:"login_trend_30_days"`
	NewUsersTrend30Days []NewUsersTrendPoint   `json:"new_users_trend_30_days"`
}

type OverviewUsersStats struct {
	RecentUsers   []UserSummary `json:"recent_users"`
	NeverLoggedIn []UserSummary `json:"never_logged_in"`
}

type UserSummary struct {
	ID          string     `json:"id"`
	Username    string     `json:"username"`
	FirstName   *string    `json:"first_name,omitempty"`
	LastName    *string    `json:"last_name,omitempty"`
	Email       string     `json:"email"`
	Enabled     *bool      `json:"enabled,omitempty"`
	Roles       []string   `json:"roles,omitempty"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

type NewUsersTrendPoint struct {
	Day      time.Time `json:"day"`
	NewUsers int64     `json:"new_users"`
}

type OverviewResponse struct {
	System   OverviewSystemStats   `json:"system"`
	Clients  OverviewClientStats   `json:"clients"`
	Security OverviewSecurityStats `json:"security"`
	Trends   OverviewTrends        `json:"trends"`
	Users    OverviewUsersStats    `json:"users"`
	Meta     map[string]time.Time  `json:"_meta"`
}
