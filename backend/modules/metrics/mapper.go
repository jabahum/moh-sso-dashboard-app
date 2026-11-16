package metrics

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/metrics/model"
)

func stringPtr(value interface{}) *string {
	if value == nil {
		return nil
	}
	s := toString(value)
	if s == "" {
		return nil
	}
	return &s
}

func timePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	v := value.Time
	return &v
}

func boolPtr(value sql.NullBool) *bool {
	if !value.Valid {
		return nil
	}
	v := value.Bool
	return &v
}

func toString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case uuid.UUID:
		return v.String()
	case uuid.NullUUID:
		if v.Valid {
			return v.UUID.String()
		}
		return ""
	default:
		return ""
	}
}

func toLoginTrendPoints(rows []db.LoginTrendRow) []LoginTrendPoint {
	out := make([]LoginTrendPoint, 0, len(rows))
	for _, row := range rows {
		out = append(out, LoginTrendPoint{Day: row.Day, Total: row.Total})
	}
	return out
}

func toLoginTrendByDayPoints(rows []db.LoginTrendByDayRow) []LoginTrendByDayPoint {
	out := make([]LoginTrendByDayPoint, 0, len(rows))
	for _, row := range rows {
		out = append(out, LoginTrendByDayPoint{
			Day:          row.Day,
			SuccessCount: row.SuccessCount,
			FailureCount: row.FailureCount,
		})
	}
	return out
}

func toNewUsersTrendPoints(rows []db.NewUsersTrendRow) []NewUsersTrendPoint {
	out := make([]NewUsersTrendPoint, 0, len(rows))
	for _, row := range rows {
		out = append(out, NewUsersTrendPoint{
			Day:      row.Day,
			NewUsers: row.NewUsers,
		})
	}
	return out
}

func toSuspiciousLogins(rows []db.SuspiciousLoginsInRangeRow) []SuspiciousLogin {
	out := make([]SuspiciousLogin, 0, len(rows))
	for _, row := range rows {
		item := SuspiciousLogin{
			ID: row.ID.String(),
		}
		if row.UserID.Valid {
			v := row.UserID.UUID.String()
			item.UserID = &v
		}
		item.Ip = stringPtr(row.Ip)
		item.Country = stringPtr(row.Country)
		item.City = stringPtr(row.City)
		if row.CreatedAt.Valid {
			v := row.CreatedAt.Time
			item.CreatedAt = &v
		}
		out = append(out, item)
	}
	return out
}

func toMostAccessedClients(rows []db.MostAccessedClientsRow) []MostAccessedClient {
	out := make([]MostAccessedClient, 0, len(rows))
	for _, row := range rows {
		out = append(out, MostAccessedClient{
			ClientID:   stringPtr(row.ClientID),
			LoginCount: row.LoginCount,
		})
	}
	return out
}

func toRecentClients(rows []db.RecentlyCreatedClientsRow) []RecentClient {
	out := make([]RecentClient, 0, len(rows))
	for _, row := range rows {
		item := RecentClient{
			ID:           row.ID.String(),
			ClientID:     row.ClientID,
			Name:         row.Name,
			PublicClient: row.PublicClient,
			Enabled:      row.Enabled,
			CreatedAt:    row.CreatedAt,
			UpdatedAt:    row.UpdatedAt,
		}
		item.Description = stringPtr(row.Description)
		item.BaseUrl = stringPtr(row.BaseUrl)
		item.Icon = stringPtr(row.Icon)
		out = append(out, item)
	}
	return out
}

func toActiveUsersPerClient(rows []db.ActiveUsersPerClientTodayRow) []ActiveUsersPerClient {
	out := make([]ActiveUsersPerClient, 0, len(rows))
	for _, row := range rows {
		out = append(out, ActiveUsersPerClient{
			ClientID:    stringPtr(row.ClientID),
			ActiveUsers: row.ActiveUsers,
		})
	}
	return out
}

func toUserClientUsage(rows []db.ClientUsageForUserInRangeRow) []UserClientUsagePoint {
	out := make([]UserClientUsagePoint, 0, len(rows))
	for _, row := range rows {
		out = append(out, UserClientUsagePoint{
			ClientID:   stringPtr(row.ClientID),
			LoginCount: row.LoginCount,
		})
	}
	return out
}

func toUserSummary(row db.User) UserSummary {
	item := UserSummary{
		ID:       row.ID.String(),
		Username: row.Username,
		Email:    row.Email,
		Roles:    row.Roles,
	}
	if row.FirstName.Valid {
		v := row.FirstName.String
		item.FirstName = &v
	}
	if row.LastName.Valid {
		v := row.LastName.String
		item.LastName = &v
	}
	if row.Enabled.Valid {
		v := row.Enabled.Bool
		item.Enabled = &v
	}
	if row.CreatedAt.Valid {
		v := row.CreatedAt.Time
		item.CreatedAt = &v
	}
	if row.UpdatedAt.Valid {
		v := row.UpdatedAt.Time
		item.UpdatedAt = &v
	}
	if row.LastLoginAt.Valid {
		v := row.LastLoginAt.Time
		item.LastLoginAt = &v
	}
	return item
}

func toUserSummaries(rows []db.User) []UserSummary {
	out := make([]UserSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, toUserSummary(row))
	}
	return out
}

func lastLoginResponse(value sql.NullTime) LastLoginResponse {
	return LastLoginResponse{LastLoginAt: timePtr(value)}
}
