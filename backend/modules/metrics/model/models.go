// Package model defines persistence records exchanged through the metrics repository port.
// These records belong to the metrics API and do not import a host SQL generator.
package model

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type ActiveUsersPerClientTodayRow struct {
	ClientID    interface{} `json:"client_id"`
	ActiveUsers int64       `json:"active_users"`
}

type ClientUsageForUserInRangeRow struct {
	ClientID   interface{} `json:"client_id"`
	LoginCount int64       `json:"login_count"`
}

type LastLoginForAllUsersRow struct {
	UserID      uuid.UUID   `json:"user_id"`
	LastLoginAt interface{} `json:"last_login_at"`
}

type LoginCountPerUserInRangeRow struct {
	UserID     uuid.NullUUID `json:"user_id"`
	LoginCount int64         `json:"login_count"`
}

type LoginSuccessFailureInRangeRow struct {
	SuccessCount int64 `json:"success_count"`
	FailureCount int64 `json:"failure_count"`
}

type LoginTrendByDayRow struct {
	Day          time.Time `json:"day"`
	SuccessCount int64     `json:"success_count"`
	FailureCount int64     `json:"failure_count"`
}

type LoginTrendRow struct {
	Day   time.Time `json:"day"`
	Total int64     `json:"total"`
}

type MostAccessedClientsRow struct {
	ClientID   interface{} `json:"client_id"`
	LoginCount int64       `json:"login_count"`
}

type MostActiveClientsRow struct {
	ClientID interface{} `json:"client_id"`
	Hits     int64       `json:"hits"`
}

type NewClientsInRangeRow struct {
	ID           uuid.UUID       `json:"id"`
	ClientID     string          `json:"client_id"`
	Name         string          `json:"name"`
	Description  sql.NullString  `json:"description"`
	BaseUrl      sql.NullString  `json:"base_url"`
	Icon         sql.NullString  `json:"icon"`
	PublicClient bool            `json:"public_client"`
	Enabled      bool            `json:"enabled"`
	Attributes   json.RawMessage `json:"attributes"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type NewUsersTrendRow struct {
	Day      time.Time `json:"day"`
	NewUsers int64     `json:"new_users"`
}

type RecentlyCreatedClientsRow struct {
	ID           uuid.UUID       `json:"id"`
	ClientID     string          `json:"client_id"`
	Name         string          `json:"name"`
	Description  sql.NullString  `json:"description"`
	BaseUrl      sql.NullString  `json:"base_url"`
	Icon         sql.NullString  `json:"icon"`
	PublicClient bool            `json:"public_client"`
	Enabled      bool            `json:"enabled"`
	Attributes   json.RawMessage `json:"attributes"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type SuspiciousLoginsInRangeRow struct {
	ID        uuid.UUID     `json:"id"`
	UserID    uuid.NullUUID `json:"user_id"`
	Ip        interface{}   `json:"ip"`
	Country   interface{}   `json:"country"`
	City      interface{}   `json:"city"`
	CreatedAt sql.NullTime  `json:"created_at"`
}

type User struct {
	ID          uuid.UUID      `json:"id"`
	Username    string         `json:"username"`
	FirstName   sql.NullString `json:"first_name"`
	LastName    sql.NullString `json:"last_name"`
	Email       string         `json:"email"`
	Enabled     sql.NullBool   `json:"enabled"`
	Roles       []string       `json:"roles"`
	CreatedAt   sql.NullTime   `json:"created_at"`
	UpdatedAt   sql.NullTime   `json:"updated_at"`
	LastLoginAt sql.NullTime   `json:"last_login_at"`
}
