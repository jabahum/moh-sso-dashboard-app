package users

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/moh-sso-dashboard/internal/cache"
	"github.com/moh-sso-dashboard/internal/http/apierror"
	"github.com/moh-sso-dashboard/internal/http/response"
	models "github.com/moh-sso-dashboard/internal/model"
	sharedservice "github.com/moh-sso-dashboard/internal/service"
	"github.com/moh-sso-dashboard/internal/utils"
)

type Handler struct {
	service      *Service
	auditService *sharedservice.AuditService
	cache        *cache.RedisCache
}

func NewHandler(
	s *Service,
	audit *sharedservice.AuditService,
	cache *cache.RedisCache,
) *Handler {
	return &Handler{
		service:      s,
		auditService: audit,
		cache:        cache,
	}
}

/* =========================================================
 * Create User
 * ========================================================= */

func (h *Handler) CreateUser(c *gin.Context) {
	var req CreateUserRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.audit(c, "user.create_failed", map[string]interface{}{
			"reason": "invalid_body",
		})

		response.Fail(c, http.StatusBadRequest, "VALIDATION_FAILED", "Invalid request payload")
		return
	}

	actorID, _ := uuid.Parse(c.GetString("user_id"))

	user, err := h.service.CreateUser(
		c.Request.Context(),
		req,
		actorID,
	)
	if err != nil {
		h.audit(c, "user.create_failed", map[string]interface{}{
			"username": req.Username,
			"email":    req.Email,
			"reason":   "request failed",
		})

		var apiErr *apierror.APIError
		if errors.As(err, &apiErr) {
			response.Fail(c, apiErr.HTTPStatus, apiErr.Code, apiErr.Message)
			return
		}

		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create user")
		return
	}

	h.audit(c, "user.create_success", map[string]interface{}{
		"user_id":  user.ID,
		"username": user.Username,
		"email":    user.Email,
	})

	h.invalidateUsersCache(c.Request.Context())

	response.OK(c, http.StatusCreated, toUserResponse(user))
}

/* =========================================================
 * Get User
 * ========================================================= */

func (h *Handler) GetUser(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		h.audit(c, "user.get_failed", map[string]interface{}{
			"reason": "missing_id",
		})

		response.Fail(c, http.StatusBadRequest, "VALIDATION_FAILED", "User ID is required")
		return
	}

	userID, err := uuid.Parse(id)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_UUID", "Invalid user ID format")
		return
	}

	user, err := h.service.GetUser(userID)
	if err != nil {
		h.audit(c, "user.get_failed", map[string]interface{}{
			"user_id": id,
		})

		response.Fail(c, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
		return
	}

	h.audit(c, "user.get_success", map[string]interface{}{
		"user_id": id,
	})

	response.OK(c, http.StatusOK, toUserResponse(user))
}

/* =========================================================
 * List Users
 * ========================================================= */

func (h *Handler) ListUsers(c *gin.Context) {
	h.audit(c, "user.list", nil)

	var (
		users     []models.User
		fromCache bool
	)

	cacheKey := listUsersCacheKey()

	if h.cache != nil {
		if ok, _ := h.cache.Get(c.Request.Context(), cacheKey, &users); ok {
			fromCache = true
		}
	}

	if !fromCache {
		var err error
		users, err = h.service.ListUsers()
		if err != nil {
			response.Fail(
				c,
				http.StatusInternalServerError,
				"INTERNAL_ERROR",
				"Failed to list users",
			)
			return
		}

		if h.cache != nil {
			_ = h.cache.Set(
				c.Request.Context(),
				cacheKey,
				users,
				45*time.Second,
			)
		}
	}
	out := make([]UserResponse, len(users))
	for i := range users {
		out[i] = toUserResponse(&users[i])
	}

	response.OK(c, http.StatusOK, out)
}

/* =========================================================
 * Delete User
 * ========================================================= */

func (h *Handler) DeleteUser(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		h.audit(c, "user.delete_failed", map[string]interface{}{
			"reason": "missing_id",
		})

		response.Fail(c, http.StatusBadRequest, "VALIDATION_FAILED", "User ID is required")
		return
	}

	userID, err := uuid.Parse(id)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_UUID", "Invalid user ID format")
		return
	}

	actorID, _ := uuid.Parse(c.GetString("user_id"))

	if err := h.service.DeleteUser(
		c.Request.Context(),
		userID,
		actorID,
	); err != nil {

		h.audit(c, "user.delete_failed", map[string]interface{}{
			"user_id": id,
			"reason":  "request failed",
		})

		var apiErr *apierror.APIError
		if errors.As(err, &apiErr) {
			response.Fail(c, apiErr.HTTPStatus, apiErr.Code, apiErr.Message)
			return
		}

		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete user")
		return
	}

	h.audit(c, "user.delete_success", map[string]interface{}{
		"user_id": id,
	})

	h.invalidateUsersCache(c.Request.Context())

	c.Status(http.StatusNoContent)
}

/* =========================================================
 * Update User
 * ========================================================= */

func (h *Handler) UpdateUser(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_UUID", "Invalid user ID format")
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.audit(c, "user.update_failed", map[string]interface{}{
			"user_id": userID.String(),
			"reason":  "invalid_body",
		})

		response.Fail(c, http.StatusBadRequest, "VALIDATION_FAILED", "Invalid request payload")
		return
	}

	req.ID = userID.String()
	actorID, _ := uuid.Parse(c.GetString("user_id"))

	user, err := h.service.UpdateUser(c.Request.Context(), req, actorID)
	if err != nil {
		h.audit(c, "user.update_failed", map[string]interface{}{
			"user_id": userID.String(),
			"reason":  "request failed",
		})

		var apiErr *apierror.APIError
		if errors.As(err, &apiErr) {
			response.Fail(c, apiErr.HTTPStatus, apiErr.Code, apiErr.Message)
			return
		}

		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update user")
		return
	}

	h.audit(c, "user.update_success", map[string]interface{}{
		"user_id": userID.String(),
	})

	h.invalidateUsersCache(c.Request.Context())

	response.OK(c, http.StatusOK, toUserResponse(user))
}

/* =========================================================
 * Get User Client Roles (ADMIN)
 * ========================================================= */

func (h *Handler) GetUserClientRoles(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_UUID", "Invalid user ID")
		return
	}

	roles, err := h.service.GetUserClientRoles(
		c.Request.Context(),
		userID,
	)
	if err != nil {
		h.audit(c, "user.client_roles_failed", map[string]interface{}{
			"user_id": userID.String(),
			"reason":  "request failed",
		})

		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch user client roles")
		return
	}

	h.audit(c, "user.client_roles_listed", map[string]interface{}{
		"user_id": userID.String(),
	})

	response.OK(c, http.StatusOK, toUserClientRoleAssignmentResponses(roles))
}

/* =========================================================
 * Get user client roles for a specific client (ADMIN)
 * ========================================================= */
func (h *Handler) GetUserClientRolesForClient(c *gin.Context) {

	// -----------------------------
	// User ID (path param)
	// -----------------------------
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(
			c,
			http.StatusBadRequest,
			"INVALID_UUID",
			"Invalid user ID",
		)
		return
	}

	clientID := strings.TrimSpace(c.Param("clientID"))
	clientUUID := strings.TrimSpace(c.Query("clientUuid"))

	if clientID == "" {
		response.Fail(
			c,
			http.StatusBadRequest,
			"VALIDATION_FAILED",
			"clientID is required",
		)
		return
	}

	if clientUUID != "" {
		if _, err := uuid.Parse(clientUUID); err != nil {
			response.Fail(
				c,
				http.StatusBadRequest,
				"INVALID_UUID",
				"Invalid client UUID",
			)
			return
		}
	}

	body := userClientRolesForClientRequest{
		ClientID:   clientID,
		ClientUUID: clientUUID,
	}

	if strings.TrimSpace(body.ClientID) == "" {
		response.Fail(
			c,
			http.StatusBadRequest,
			"VALIDATION_FAILED",
			"clientID is required",
		)
		return
	}

	// -----------------------------
	// Service call (INTERNAL UUID ONLY)
	// -----------------------------
	roles, err := h.service.GetUserClientRolesForClient(
		c.Request.Context(),
		userID,
		body.ClientID,
		body.ClientUUID,
	)
	if err != nil {
		h.audit(
			c,
			"user.client_roles_get_failed",
			map[string]interface{}{
				"user_id":     userID.String(),
				"client_id":   body.ClientID,
				"client_uuid": body.ClientUUID,
				"reason":      "request failed",
			},
		)

		response.Fail(
			c,
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"Failed to fetch user client roles",
		)
		return
	}

	// -----------------------------
	// Audit success
	// -----------------------------
	h.audit(
		c,
		"user.client_roles_get_success",
		map[string]interface{}{
			"user_id":     userID.String(),
			"client_id":   body.ClientID,
			"client_uuid": body.ClientUUID,
			"count":       len(roles),
		},
	)

	response.OK(c, http.StatusOK, toClientRoleResponses(roles))
}

/* =========================================================
 * Update User Client Roles (ADMIN, PUT)
 * ========================================================= */

func (h *Handler) UpdateUserClientRoles(c *gin.Context) {

	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(
			c,
			http.StatusBadRequest,
			"INVALID_UUID",
			"Invalid user ID",
		)
		return
	}

	var body userClientRolesRequest

	if err := c.ShouldBindJSON(&body); err != nil || body.ClientUUID == "" || body.ClientID == "" {
		response.Fail(
			c,
			http.StatusBadRequest,
			"VALIDATION_FAILED",
			"clientId , clientUuid and roles are required",
		)
		return
	}

	body.ClientID = strings.TrimSpace(body.ClientID)

	clientUUID, err := uuid.Parse(body.ClientUUID)
	if err != nil {
		response.Fail(
			c,
			http.StatusBadRequest,
			"INVALID_UUID",
			"Invalid client UUID",
		)
		return
	}

	adminID, _ := uuid.Parse(c.GetString("user_id"))

	if err := h.service.UpdateUserClientRoles(
		c.Request.Context(),
		userID,
		body.ClientID,
		clientUUID.String(),
		body.Roles,
		adminID,
	); err != nil {

		h.audit(c, "user.client_roles_update_failed", map[string]interface{}{
			"user_id":     userID.String(),
			"client_id":   body.ClientID,
			"client_uuid": clientUUID.String(),
			"roles":       body.Roles,
			"reason":      "request failed",
		})

		if errors.Is(err, ErrServiceOnlyRole) {
			response.Fail(c, http.StatusBadRequest, "SERVICE_ONLY_ROLE", ErrServiceOnlyRole.Error())
			return
		}

		response.Fail(
			c,
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"Failed to update user client roles",
		)
		return
	}

	h.audit(c, "user.client_roles_updated", map[string]interface{}{
		"user_id":     userID.String(),
		"client_id":   body.ClientID,
		"client_uuid": clientUUID.String(),
		"roles":       body.Roles,
	})

	h.invalidateUsersCache(c.Request.Context())

	c.Status(http.StatusNoContent)
}

/* =========================================================
 * Add User Client Roles (ADMIN)
 * ========================================================= */

func (h *Handler) AddUserClientRoles(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_UUID", "Invalid user ID")
		return
	}

	var body userClientRolesRequest

	if err := c.ShouldBindJSON(&body); err != nil || body.ClientID == "" || body.ClientUUID == "" {
		response.Fail(c, http.StatusBadRequest, "VALIDATION_FAILED", "clientId, clientUuid and roles are required")
		return
	}

	body.ClientID = strings.TrimSpace(body.ClientID)

	clientUUID, err := uuid.Parse(body.ClientUUID)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_UUID", "Invalid client UUID")
		return
	}

	adminID, _ := uuid.Parse(c.GetString("user_id"))

	if err := h.service.AddUserClientRoles(
		c.Request.Context(),
		userID,
		body.ClientID,
		clientUUID.String(),
		body.Roles,
		adminID,
	); err != nil {
		h.audit(c, "user.client_roles_add_failed", map[string]interface{}{
			"user_id":     userID.String(),
			"client_id":   body.ClientID,
			"client_uuid": clientUUID.String(),
			"roles":       body.Roles,
			"reason":      "request failed",
		})

		if errors.Is(err, ErrServiceOnlyRole) {
			response.Fail(c, http.StatusBadRequest, "SERVICE_ONLY_ROLE", ErrServiceOnlyRole.Error())
			return
		}
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to add user client roles")
		return
	}

	h.invalidateUsersCache(c.Request.Context())

	c.Status(http.StatusNoContent)
}

/* =========================================================
 * Remove User Client Roles (ADMIN)
 * ========================================================= */

func (h *Handler) RemoveUserClientRoles(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_UUID", "Invalid user ID")
		return
	}

	var body userClientRolesRequest

	if err := c.ShouldBindJSON(&body); err != nil || body.ClientID == "" || body.ClientUUID == "" {
		response.Fail(c, http.StatusBadRequest, "VALIDATION_FAILED", "clientId, clientUuid and roles are required")
		return
	}

	body.ClientID = strings.TrimSpace(body.ClientID)

	clientUUID, err := uuid.Parse(body.ClientUUID)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_UUID", "Invalid client UUID")
		return
	}

	adminID, _ := uuid.Parse(c.GetString("user_id"))

	if err := h.service.RemoveUserClientRoles(
		c.Request.Context(),
		userID,
		body.ClientID,
		clientUUID.String(),
		body.Roles,
		adminID,
	); err != nil {
		h.audit(c, "user.client_roles_remove_failed", map[string]interface{}{
			"user_id":     userID.String(),
			"client_id":   body.ClientID,
			"client_uuid": clientUUID.String(),
			"roles":       body.Roles,
			"reason":      "request failed",
		})

		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to remove user client roles")
		return
	}

	h.invalidateUsersCache(c.Request.Context())

	c.Status(http.StatusNoContent)
}

/* =========================================================
 * Reset User Password (ADMIN)
 * ========================================================= */

func (h *Handler) ResetUserPassword(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_UUID", "Invalid user ID")
		return
	}

	adminID, _ := uuid.Parse(c.GetString("user_id"))

	if err := h.service.ResetUserPassword(
		c.Request.Context(),
		userID,
		adminID,
	); err != nil {

		h.audit(c, "user.password_reset_failed", map[string]interface{}{
			"user_id": userID.String(),
			"reason":  "request failed",
		})

		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to reset user password")
		return
	}

	h.audit(c, "user.password_reset_success", map[string]interface{}{
		"user_id": userID.String(),
	})

	c.Status(http.StatusNoContent)
}

func (h *Handler) SendUserOnboardingEmail(c *gin.Context) {
	h.sendUserEmailAction(c, "user.onboarding_email", h.service.SendUserOnboardingEmail)
}

func (h *Handler) SendUserVerificationEmail(c *gin.Context) {
	h.sendUserEmailAction(c, "user.verification_email", h.service.SendUserVerificationEmail)
}

func (h *Handler) SendUserPasswordResetEmail(c *gin.Context) {
	h.sendUserEmailAction(c, "user.password_reset_email", h.service.SendUserPasswordResetEmail)
}

func (h *Handler) sendUserEmailAction(
	c *gin.Context,
	auditPrefix string,
	action func(context.Context, uuid.UUID, uuid.UUID) error,
) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_UUID", "Invalid user ID")
		return
	}

	adminID, _ := uuid.Parse(c.GetString("user_id"))

	if err := action(c.Request.Context(), userID, adminID); err != nil {
		h.audit(c, auditPrefix+"_failed", map[string]interface{}{
			"user_id": userID.String(),
			"reason":  "request failed",
		})

		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to send user email action")
		return
	}

	h.audit(c, auditPrefix+"_success", map[string]interface{}{
		"user_id": userID.String(),
	})

	c.Status(http.StatusNoContent)
}

/* =========================================================
 * Enable / Disable user (ADMIN)
 * ========================================================= */
func (h *Handler) SetUserEnabled(c *gin.Context) {

	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(
			c,
			http.StatusBadRequest,
			"INVALID_UUID",
			"Invalid user ID",
		)
		return
	}

	var body setUserEnabledRequest

	if err := c.ShouldBindJSON(&body); err != nil || body.Enabled == nil {
		response.Fail(
			c,
			http.StatusBadRequest,
			"VALIDATION_FAILED",
			"enabled is required",
		)
		return
	}

	adminID, _ := uuid.Parse(c.GetString("user_id"))

	if err := h.service.SetUserEnabled(
		c.Request.Context(),
		userID,
		*body.Enabled,
		adminID,
	); err != nil {

		h.audit(c, "user.toggle_failed", map[string]interface{}{
			"user_id": userID.String(),
			"enabled": body.Enabled,
			"reason":  "request failed",
		})
		var apiErr *apierror.APIError
		if errors.As(err, &apiErr) {
			response.Fail(c, apiErr.HTTPStatus, apiErr.Code, apiErr.Message)
			return
		}

		response.Fail(
			c,
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"Failed to update user status",
		)
		return
	}

	h.audit(c, "user.toggle_success", map[string]interface{}{
		"user_id": userID.String(),
		"enabled": body.Enabled,
	})

	h.invalidateUsersCache(c.Request.Context())

	c.Status(http.StatusNoContent)
}

/* =========================================================
 * Audit helper
 * ========================================================= */

func (h *Handler) audit(
	c *gin.Context,
	action string,
	meta map[string]interface{},
) {
	if h == nil || h.auditService == nil {
		return
	}

	if meta == nil {
		meta = map[string]interface{}{}
	}

	meta["ip"] = c.ClientIP()
	meta["user_agent"] = c.Request.UserAgent()

	_ = h.auditService.Log(
		c.Request.Context(),
		utils.ToNullUUID(c.GetString("user_id")),
		action,
		meta,
	)
}

func listUsersCacheKey() string {
	return "users:all"
}

func (h *Handler) invalidateUsersCache(ctx context.Context) {
	if h == nil || h.cache == nil {
		return
	}

	_ = h.cache.Del(ctx, listUsersCacheKey())
}
