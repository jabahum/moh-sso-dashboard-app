package platform

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/moh-sso-dashboard/internal/http/response"
)

var ErrIdempotencyConflict = errors.New("idempotency key already used with a different request")
var ErrQuota = errors.New("application notification quota exceeded")
var idempotencyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

type NotificationInput struct {
	Title     string `json:"title" binding:"required"`
	Message   string `json:"message" binding:"required"`
	Recipient struct {
		Email string `json:"email" binding:"required"`
		Name  string `json:"name"`
	} `json:"recipient" binding:"required"`
}
type NotificationStatus struct {
	ID               string `json:"id"`
	ApplicationID    string `json:"applicationId"`
	Channel          string `json:"channel"`
	Status           string `json:"status"`
	DispatchStatus   string `json:"dispatchStatus"`
	DispatchAttempts int    `json:"dispatchAttempts"`
	DeliveryAttempts int    `json:"deliveryAttempts"`
	CreatedAt        string `json:"createdAt"`
	Replayed         bool   `json:"replayed"`
}

func normalizeNotification(input *NotificationInput) error {
	if strings.ContainsAny(input.Title+input.Recipient.Name+input.Recipient.Email, "\r\n") {
		return ErrInvalid
	}
	input.Title, input.Message = strings.TrimSpace(input.Title), strings.TrimSpace(input.Message)
	input.Recipient.Email, input.Recipient.Name = strings.TrimSpace(input.Recipient.Email), strings.TrimSpace(input.Recipient.Name)
	if input.Title == "" || len(input.Title) > 200 || strings.ContainsAny(input.Title, "\r\n") || input.Message == "" || len(input.Message) > 10000 || len(input.Recipient.Email) > 254 || len(input.Recipient.Name) > 200 || strings.ContainsAny(input.Recipient.Name, "\r\n") {
		return ErrInvalid
	}
	address, err := mail.ParseAddress(input.Recipient.Email)
	if err != nil || address.Address != input.Recipient.Email || strings.ContainsAny(input.Recipient.Email, "\r\n") {
		return ErrInvalid
	}
	return nil
}
func notificationHash(input NotificationInput) string {
	data, _ := json.Marshal(input)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func (h *Handler) SubmitNotification(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16384)
	var input NotificationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		fail(c, ErrInvalid)
		return
	}
	key := c.GetHeader("Idempotency-Key")
	if !idempotencyPattern.MatchString(key) {
		fail(c, ErrInvalid)
		return
	}
	if err := normalizeNotification(&input); err != nil {
		fail(c, err)
		return
	}
	p := c.MustGet("platform_principal").(Principal)
	result, err := h.Repository.SubmitNotification(c.Request.Context(), p, key, input)
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("Location", "/api/v1/platform/notifications/"+result.ID)
	status := http.StatusAccepted
	if result.Replayed {
		status = http.StatusOK
	}
	response.OK(c, status, result)
}
func (h *Handler) GetNotification(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil || id == uuid.Nil {
		fail(c, ErrInvalid)
		return
	}
	p := c.MustGet("platform_principal").(Principal)
	result, err := h.Repository.GetNotification(c.Request.Context(), p.ApplicationID, id.String())
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, http.StatusOK, result)
}
