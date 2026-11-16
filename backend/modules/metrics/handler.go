package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/mohuganda/moh-sso-dashboard/backend/modules/metrics/internal/http/response"
)

type Handler struct {
	service *Service
}

func NewHandler(s *Service) *Handler {
	return &Handler{service: s}
}

/* =========================================================
 * Helpers
 * ========================================================= */

func (h *Handler) parseRange(c *gin.Context) (time.Time, time.Time, bool) {
	startStr := c.Query("start")
	endStr := c.Query("end")

	if startStr == "" || endStr == "" {
		response.Fail(
			c,
			http.StatusBadRequest,
			"VALIDATION_FAILED",
			"Both start and end parameters are required (RFC3339)",
		)
		return time.Time{}, time.Time{}, false
	}

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		response.Fail(
			c,
			http.StatusBadRequest,
			"INVALID_DATE_FORMAT",
			"Invalid start format (must be RFC3339)",
		)
		return time.Time{}, time.Time{}, false
	}

	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		response.Fail(
			c,
			http.StatusBadRequest,
			"INVALID_DATE_FORMAT",
			"Invalid end format (must be RFC3339)",
		)
		return time.Time{}, time.Time{}, false
	}

	return start, end, true
}

/* =========================================================
 * System Metrics
 * ========================================================= */

func (h *Handler) CountUsers(c *gin.Context) {
	v, err := h.service.CountUsers(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to count users")
		return
	}

	response.OK(c, http.StatusOK, TotalUsersResponse{TotalUsers: v})
}

func (h *Handler) CountDisabledUsers(c *gin.Context) {
	v, err := h.service.CountDisabledUsers(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to count disabled users")
		return
	}

	response.OK(c, http.StatusOK, DisabledUsersResponse{DisabledUsers: v})
}

func (h *Handler) ActiveUsersToday(c *gin.Context) {
	v, err := h.service.ActiveUsersToday(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch active users today")
		return
	}

	response.OK(c, http.StatusOK, ActiveUsersTodayResponse{ActiveUsersToday: v})
}

func (h *Handler) ActiveUsersThisWeek(c *gin.Context) {
	v, err := h.service.ActiveUsersThisWeek(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch active users this week")
		return
	}

	response.OK(c, http.StatusOK, ActiveUsersThisWeekResponse{ActiveUsersThisWeek: v})
}

/* =========================================================
 * Login Trends
 * ========================================================= */

func (h *Handler) LoginTrend(c *gin.Context) {
	rows, err := h.service.LoginTrend(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch login trend")
		return
	}

	response.OK(c, http.StatusOK, toLoginTrendPoints(rows))
}

func (h *Handler) LoginTrendByDay(c *gin.Context) {
	start, end, ok := h.parseRange(c)
	if !ok {
		return
	}

	rows, err := h.service.LoginTrendByDay(c.Request.Context(), start, end)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch login trend")
		return
	}

	response.OK(c, http.StatusOK, toLoginTrendByDayPoints(rows))
}

/* =========================================================
 * Security Metrics
 * ========================================================= */

func (h *Handler) CountFailedLogins(c *gin.Context) {
	v, err := h.service.CountFailedLogins(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to count failed logins")
		return
	}

	response.OK(c, http.StatusOK, FailedLoginsResponse{FailedLogins: v})
}

func (h *Handler) CountFailedLoginsInRange(c *gin.Context) {
	start, end, ok := h.parseRange(c)
	if !ok {
		return
	}

	v, err := h.service.CountFailedLoginsInRange(c.Request.Context(), start, end)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to count failed logins")
		return
	}

	response.OK(c, http.StatusOK, FailedLoginsResponse{FailedLogins: v})
}

func (h *Handler) SuspiciousLogins(c *gin.Context) {
	start, end, ok := h.parseRange(c)
	if !ok {
		return
	}

	home := c.Query("home")

	rows, err := h.service.SuspiciousLogins(
		c.Request.Context(),
		start,
		end,
		home,
	)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch suspicious logins")
		return
	}

	response.OK(c, http.StatusOK, toSuspiciousLogins(rows))
}

/* =========================================================
 * Client Metrics
 * ========================================================= */

func (h *Handler) CountClients(c *gin.Context) {
	v, err := h.service.CountClients(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to count clients")
		return
	}

	response.OK(c, http.StatusOK, TotalClientsResponse{TotalClients: v})
}

func (h *Handler) MostAccessedClients(c *gin.Context) {
	start, end, ok := h.parseRange(c)
	if !ok {
		return
	}

	limit := int32(10)
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = int32(parsed)
		}
	}

	rows, err := h.service.MostAccessedClients(
		c.Request.Context(),
		start,
		end,
		limit,
	)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch client usage")
		return
	}

	response.OK(c, http.StatusOK, toMostAccessedClients(rows))
}

func (h *Handler) ActiveUsersPerClientToday(c *gin.Context) {
	rows, err := h.service.ActiveUsersPerClientToday(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch active users per client")
		return
	}

	response.OK(c, http.StatusOK, toActiveUsersPerClient(rows))
}

func (h *Handler) LoginCountForClient(c *gin.Context) {
	clientID := c.Query("client_id")
	if clientID == "" {
		response.Fail(c, http.StatusBadRequest, "CLIENT_ID NOT FOUND", "client_id required")
		return
	}

	start, end, ok := h.parseRange(c)
	if !ok {
		return
	}

	v, err := h.service.LoginCountForClient(c.Request.Context(), clientID, start, end)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to count client logins")
		return
	}

	response.OK(c, http.StatusOK, LoginCountResponse{LoginCount: v})
}

/* =========================================================
 * User Metrics
 * ========================================================= */

func (h *Handler) LastLoginForUser(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("userID"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_UUID", "Invalid user ID format")
		return
	}

	row, err := h.service.LastLoginForUser(c.Request.Context(), userID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch last login")
		return
	}

	response.OK(c, http.StatusOK, lastLoginResponse(row))
}

func (h *Handler) UserClientUsage(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("userID"))
	if err != nil {
		response.Fail(c, http.StatusBadGateway, "INVALID_UUID", "Invalid user ID format")
		return
	}

	start, end, ok := h.parseRange(c)
	if !ok {
		return
	}

	rows, err := h.service.ClientUsageForUser(
		c.Request.Context(),
		userID,
		start,
		end,
	)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch client usage")
		return
	}

	response.OK(c, http.StatusOK, toUserClientUsage(rows))
}

func (h *Handler) NewUsersInRange(c *gin.Context) {
	start, end, ok := h.parseRange(c)
	if !ok {
		return
	}

	rows, err := h.service.NewUsersInRange(c.Request.Context(), start, end)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch new users")
		return
	}

	response.OK(c, http.StatusOK, toUserSummaries(rows))
}

func (h *Handler) NewUsersTrend(c *gin.Context) {
	start, end, ok := h.parseRange(c)
	if !ok {
		return
	}

	rows, err := h.service.NewUsersTrend(c.Request.Context(), start, end)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch user trend")
		return
	}

	response.OK(c, http.StatusOK, toNewUsersTrendPoints(rows))
}

func (h *Handler) NeverLoggedInUsers(c *gin.Context) {
	rows, err := h.service.NeverLoggedInUsers(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch users")
		return
	}

	response.OK(c, http.StatusOK, toUserSummaries(rows))
}

/* =========================================================
 * Overview (Dashboard)
 * ========================================================= */

func (h *Handler) Overview(c *gin.Context) {
	ctx := c.Request.Context()
	now := time.Now()
	start := now.AddDate(0, 0, -30)

	loginTrend, _ := h.service.LoginTrendByDay(ctx, start, now)
	newUsersTrend, _ := h.service.NewUsersTrend(ctx, start, now)
	recentUsers, _ := h.service.NewUsersInRange(ctx, start, now)
	neverLoggedIn, _ := h.service.NeverLoggedInUsers(ctx)
	activeToday, _ := h.service.ActiveUsersPerClientToday(ctx)
	recentClients, _ := h.service.RecentlyCreatedClients(ctx, 10)

	response.OK(c, http.StatusOK, OverviewResponse{
		System: OverviewSystemStats{
			TotalUsers:          must(h.service.CountUsers(ctx)),
			DisabledUsers:       must(h.service.CountDisabledUsers(ctx)),
			ActiveUsersToday:    must(h.service.ActiveUsersToday(ctx)),
			ActiveUsersThisWeek: must(h.service.ActiveUsersThisWeek(ctx)),
		},
		Clients: OverviewClientStats{
			TotalClients:   must(h.service.CountClients(ctx)),
			EnabledClients: must(h.service.CountEnabledClients(ctx)),
			ActiveToday:    toActiveUsersPerClient(activeToday),
			RecentClients:  toRecentClients(recentClients),
		},
		Security: OverviewSecurityStats{
			FailedLogins:   must(h.service.CountFailedLogins(ctx)),
			ActiveSessions: must(h.service.ApproximateActiveSessions(ctx)),
		},
		Trends: OverviewTrends{
			LoginTrend30Days:    toLoginTrendByDayPoints(loginTrend),
			NewUsersTrend30Days: toNewUsersTrendPoints(newUsersTrend),
		},
		Users: OverviewUsersStats{
			RecentUsers:   toUserSummaries(recentUsers),
			NeverLoggedIn: toUserSummaries(neverLoggedIn),
		},
		Meta: map[string]time.Time{
			"range_start":  start,
			"range_end":    now,
			"generated_at": time.Now(),
		},
	})
}

/* =========================================================
 * Small helper to ignore secondary errors in Overview
 * ========================================================= */

func must[T any](v T, _ error) T {
	return v
}
