package platform

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
)

func validNotification() NotificationInput {
	var input NotificationInput
	input.Title = "Pilot update"
	input.Message = "Test message"
	input.Recipient.Email = "recipient@example.org"
	return input
}
func TestNotificationValidationAndCanonicalHash(t *testing.T) {
	for _, mutate := range []func(*NotificationInput){func(n *NotificationInput) { n.Title = "\r\nInjected" }, func(n *NotificationInput) { n.Message = "" }, func(n *NotificationInput) { n.Recipient.Email = "Name <recipient@example.org>" }, func(n *NotificationInput) { n.Message = strings.Repeat("x", 10001) }} {
		input := validNotification()
		mutate(&input)
		if !errors.Is(normalizeNotification(&input), ErrInvalid) {
			t.Fatal("accepted invalid notification")
		}
	}
	original := validNotification()
	padded := original
	padded.Title = " " + padded.Title + " "
	if err := normalizeNotification(&padded); err != nil {
		t.Fatal(err)
	}
	if notificationHash(original) != notificationHash(padded) {
		t.Fatal("normalization does not preserve replay hash")
	}
}

type notificationTestRepository struct {
	testRepository
	submitted int
	key       string
	input     NotificationInput
	replay    bool
	submitErr error
}

func (r *notificationTestRepository) SubmitNotification(_ context.Context, p Principal, key string, input NotificationInput) (NotificationStatus, error) {
	r.submitted++
	r.key = key
	r.input = input
	return NotificationStatus{ID: "00000000-0000-4000-8000-000000000001", ApplicationID: p.ApplicationID, Status: "PENDING", Replayed: r.replay}, r.submitErr
}
func TestSubmissionRequiresScopeAndIdempotencyKey(t *testing.T) {
	repo := &notificationTestRepository{testRepository: testRepository{principal: Principal{ApplicationID: "pilot", Scopes: []string{NotificationsWrite}}}}
	h := &Handler{Repository: repo, Verifier: testVerifier{token: validToken()}}
	router := gin.New()
	RegisterRoutes(router.Group("/api/v1"), router.Group("/api/v1"), h, nil)
	request := func(key, body string) int {
		req := httptest.NewRequest("POST", "/api/v1/platform/notifications", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer token")
		req.Header.Set("Content-Type", "application/json")
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w.Code
	}
	body := `{"title":"Pilot","message":"Hello","recipient":{"email":"recipient@example.org"}}`
	if code := request("", body); code != 400 {
		t.Fatal(code)
	}
	if code := request("pilot-1", body); code != 202 {
		t.Fatal(code)
	}
	repo.replay = true
	if code := request("pilot-1", body); code != 200 {
		t.Fatal(code)
	}
	repo.submitErr = ErrIdempotencyConflict
	if code := request("pilot-1", body); code != 409 {
		t.Fatal(code)
	}
	repo.submitErr = ErrQuota
	if code := request("pilot-2", body); code != 429 {
		t.Fatal(code)
	}
	repo.principal.Scopes = nil
	before := repo.submitted
	if code := request("pilot-3", body); code != 403 {
		t.Fatal(code)
	}
	if repo.submitted != before {
		t.Fatal("unscoped request reached queue")
	}
}
