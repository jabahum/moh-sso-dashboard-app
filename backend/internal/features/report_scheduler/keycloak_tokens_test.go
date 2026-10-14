package report_scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestKeycloakTokenCacheAndRefresh(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requests.Add(1)
		json.NewEncoder(w).Encode(map[string]any{"access_token": fmt.Sprintf("token-%d", n), "token_type": "Bearer", "expires_in": 300})
	}))
	defer server.Close()
	provider := &keycloakTokenProvider{endpoint: server.URL, clientID: "worker", clientSecret: "secret", client: server.Client()}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := provider.get(context.Background()); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if requests.Load() != 1 {
		t.Fatalf("expected one cached token request, got %d", requests.Load())
	}
	provider.mu.Lock()
	provider.expiresAt = time.Now().Add(10 * time.Second)
	provider.mu.Unlock()
	if token, err := provider.get(context.Background()); err != nil || token != "token-2" {
		t.Fatalf("refresh: %s %v", token, err)
	}
	provider.invalidate("token-1")
	if token, err := provider.get(context.Background()); err != nil || token != "token-2" || requests.Load() != 2 {
		t.Fatalf("stale invalidation discarded refreshed token: %s %v", token, err)
	}
}

func TestHealthBIRefreshesUnauthorizedTokenOnce(t *testing.T) {
	for _, terminal := range []bool{false, true} {
		t.Run(fmt.Sprint(terminal), func(t *testing.T) {
			var tokens, requests int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/token" {
					tokens++
					json.NewEncoder(w).Encode(map[string]any{"access_token": fmt.Sprintf("token-%d", tokens), "token_type": "Bearer", "expires_in": 300})
					return
				}
				requests++
				if terminal || r.Header.Get("Authorization") == "Bearer token-1" {
					w.WriteHeader(401)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"data":[]}`))
			}))
			defer server.Close()
			client := NewKeycloakHealthBIClient(server.URL, server.URL+"/token", "worker", "secret", time.Second)
			_, err := client.ListReports(context.Background())
			if (err != nil) != terminal || requests != 2 || tokens != 2 {
				t.Fatalf("err=%v requests=%d tokens=%d", err, requests, tokens)
			}
		})
	}
}

func TestKeycloakTokenFailureDoesNotLeakCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte("private-client-secret"))
	}))
	defer server.Close()
	client := NewKeycloakHealthBIClient(server.URL, server.URL, "worker", "private-client-secret", time.Second)
	_, err := client.ListReports(context.Background())
	if err == nil || strings.Contains(err.Error(), "private-client-secret") {
		t.Fatalf("unsafe token error: %v", err)
	}
}
