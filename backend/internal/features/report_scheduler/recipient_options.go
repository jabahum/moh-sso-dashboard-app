package report_scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/mail"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	"github.com/moh-sso-dashboard/internal/http/response"
)

func (h *Handler) RecipientOptions(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	kind := c.DefaultQuery("kind", "email")
	if kind != "email" && kind != "group" && kind != "portal" {
		response.Fail(c, 400, "INVALID_KIND", "kind must be email, group, or portal")
		return
	}
	limit := 100
	if raw := c.Query("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 100 {
			response.Fail(c, 400, "INVALID_LIMIT", "limit must be between 1 and 100")
			return
		}
		limit = value
	}
	search := strings.TrimSpace(c.Query("search"))
	if len(search) > 200 {
		response.Fail(c, 400, "INVALID_SEARCH", "search is too long")
		return
	}
	if h.service == nil || h.service.repo == nil {
		response.Fail(c, 503, "DIRECTORY_UNAVAILABLE", "Recipient directory is unavailable")
		return
	}
	var selected []string
	if raw := c.Query("selected"); raw != "" {
		if len(raw) > 32768 || json.Unmarshal([]byte(raw), &selected) != nil || len(selected) > 100 {
			response.Fail(c, 400, "INVALID_SELECTION", "selected must contain at most 100 recipient values")
			return
		}
	}
	options, err := h.service.repo.ListRecipientOptions(c.Request.Context(), kind, search, limit, selected)
	if err != nil {
		response.Fail(c, 503, "DIRECTORY_UNAVAILABLE", "Unable to load recipient options")
		return
	}
	response.OK(c, http.StatusOK, options)
}

// Return contact labels only; never expose roles, group membership or user-admin fields.
func (r *postgresRepository) ListRecipientOptions(ctx context.Context, kind, search string, limit int, selected []string) ([]RecipientOption, error) {
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("invalid recipient option limit")
	}
	if kind == "group" {
		rows, err := r.db.QueryContext(ctx, `SELECT id::text,COALESCE(NULLIF(display_name,''),name),path
   FROM ihp_rbac_groups WHERE enabled=TRUE
   AND ($1='' OR path ILIKE '%'||$1||'%' OR name ILIKE '%'||$1||'%' OR display_name ILIKE '%'||$1||'%' OR id::text=ANY($3::text[]) OR path=ANY($3::text[]))
   ORDER BY (id::text=ANY($3::text[]) OR path=ANY($3::text[])) DESC,path,id LIMIT $2`, search, limit, pq.Array(selected))
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		options := make([]RecipientOption, 0)
		for rows.Next() {
			var id, label, path string
			if err := rows.Scan(&id, &label, &path); err != nil {
				return nil, err
			}
			if path != "" {
				label += " · " + path
			}
			value := id
			for _, saved := range selected {
				if saved == path {
					value = path
					break
				}
			}
			options = append(options, RecipientOption{ID: "group:" + value, Type: "group", Value: value, Label: label})
		}
		return options, rows.Err()
	}
	if kind != "email" && kind != "portal" {
		return nil, fmt.Errorf("invalid recipient option kind")
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id::text,username,COALESCE(first_name,''),COALESCE(last_name,''),COALESCE(email,'')
  FROM users WHERE enabled=TRUE AND username NOT LIKE 'service-account-%'
  AND ($1='' OR username ILIKE '%'||$1||'%' OR concat_ws(' ',first_name,last_name) ILIKE '%'||$1||'%' OR email ILIKE '%'||$1||'%' OR id::text=ANY($4::text[]) OR email=ANY($4::text[]))
  AND ($3='portal' OR COALESCE(email,'')<>'')
  ORDER BY COALESCE((id::text=ANY($4::text[]) OR email=ANY($4::text[])),FALSE) DESC,lower(COALESCE(first_name,'')),lower(username),id LIMIT $2`, search, limit, kind, pq.Array(selected))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	options := make([]RecipientOption, 0)
	for rows.Next() {
		var id, username, first, last, email string
		if err := rows.Scan(&id, &username, &first, &last, &email); err != nil {
			return nil, err
		}
		if kind == "email" {
			if _, err := mail.ParseAddress(email); err != nil {
				continue
			}
		}
		label := strings.TrimSpace(first + " " + last)
		if label == "" {
			label = username
		} else if label != username {
			label += " · " + username
		}
		if email != "" {
			label += " (" + email + ")"
		}
		option := RecipientOption{ID: "user:" + id, Type: "user", Value: id, Label: label}
		if kind == "email" {
			for _, saved := range selected {
				if saved == email {
					option.ID = "email:" + email
					option.Type = "email"
					option.Value = email
					break
				}
			}
		}
		options = append(options, option)
	}
	return options, rows.Err()
}
