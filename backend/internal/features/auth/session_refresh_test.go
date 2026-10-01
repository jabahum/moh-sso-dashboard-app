package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/moh-sso-dashboard/internal/features/authsession"
	"github.com/moh-sso-dashboard/internal/keycloak"
)

type fakeSessionStore struct {
	data     authsession.Data
	getErr   error
	writeErr error
	updated  bool
	deleted  bool
	ttl      time.Duration
}

func (s *fakeSessionStore) Create(_ context.Context, data authsession.Data, ttl time.Duration) (string, error) {
	if s.writeErr != nil {
		return "", s.writeErr
	}
	s.data, s.ttl = data, ttl
	return "new-session", nil
}

func (s *fakeSessionStore) Get(context.Context, string) (*authsession.Data, error) {
	return &s.data, s.getErr
}

func (s *fakeSessionStore) Update(_ context.Context, _ string, data authsession.Data, ttl time.Duration) error {
	if s.writeErr != nil {
		return s.writeErr
	}
	s.data, s.ttl, s.updated = data, ttl, true
	return nil
}

func (s *fakeSessionStore) Delete(context.Context, string) error {
	s.deleted = true
	return nil
}

func TestRefreshSessionPersistence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, scenario := range []string{"success", "update failure", "read failure", "legacy migration failure"} {
		t.Run(scenario, func(t *testing.T) {
			store := &fakeSessionStore{data: authsession.Data{RefreshToken: "old-refresh"}}
			if scenario == "update failure" || scenario == "legacy migration failure" {
				store.writeErr = errors.New("redis unavailable")
			}
			if scenario == "read failure" {
				store.getErr = errors.New("redis unavailable")
			}
			authSvc := &fakeAuthService{refreshTokens: &keycloak.TokenResponse{
				AccessToken: "new-access", RefreshToken: "new-refresh", ExpiresIn: 60, RefreshExpiresIn: 120,
			}}
			handler := newTestHandler(authSvc)
			handler.sessions = store
			router := gin.New()
			router.POST("/auth/refresh", handler.HandleAuthRefreshToken)
			req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
			if scenario == "legacy migration failure" {
				req.AddCookie(&http.Cookie{Name: cookieRefreshToken, Value: "old-refresh"})
			} else {
				req.AddCookie(&http.Cookie{Name: cookieSession, Value: "existing-session"})
			}
			req.AddCookie(&http.Cookie{Name: cookieOAuthState, Value: "in-flight-login"})
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			if scenario == "success" {
				if res.Code != http.StatusOK || !store.updated || store.data.RefreshToken != "new-refresh" || store.ttl != 120*time.Second {
					t.Fatalf("refresh was not persisted: status=%d store=%+v", res.Code, store)
				}
				assertSetCookiePresent(t, res, cookieSession)
			} else {
				assertErrorCode(t, res, http.StatusServiceUnavailable, "SESSION_UNAVAILABLE")
				for _, cookie := range res.Result().Cookies() {
					if cookie.Name == cookieSession && cookie.MaxAge > 0 {
						t.Fatal("failed persistence must not extend browser session")
					}
				}
			}
			if scenario == "read failure" && (authSvc.refreshToken != "" || store.deleted || len(res.Result().Cookies()) != 0) {
				t.Fatal("temporary read failure must preserve session and skip token exchange")
			}
			if scenario == "update failure" && !store.deleted {
				t.Fatal("failed rotation must invalidate stale server-side session")
			}
			for _, cookie := range res.Result().Cookies() {
				if cookie.Name == cookieOAuthState || cookie.Name == cookiePKCEVerifier || cookie.Name == cookieOAuthReturnTo {
					t.Fatal("refresh must preserve an in-flight OAuth login")
				}
			}
		})
	}
}
