package metrics

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	db "github.com/moh-sso-dashboard/internal/db/sqlc"
)

type storeStub struct {
	db.Store
	err         error
	rangeParams db.LoginTrendByDayParams
}

func (s *storeStub) LoginTrendByDay(_ context.Context, params db.LoginTrendByDayParams) ([]db.LoginTrendByDayRow, error) {
	s.rangeParams = params
	return []db.LoginTrendByDayRow{{Day: params.StartTime.Time, SuccessCount: 3, FailureCount: 1}}, s.err
}

func (s *storeStub) NewUsersInRange(context.Context, db.NewUsersInRangeParams) ([]db.User, error) {
	return []db.User{{ID: uuid.Nil, Enabled: sql.NullBool{Bool: false, Valid: true}, Roles: []string{"user"}}}, s.err
}

func TestSQLAdapterPreservesRangeAndNullableRecords(t *testing.T) {
	store := &storeStub{}
	r := &metricsRepository{db: store}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	rows, err := r.LoginTrendByDay(context.Background(), start, end)
	if err != nil || len(rows) != 1 || rows[0].SuccessCount != 3 || !store.rangeParams.StartTime.Valid || !store.rangeParams.EndTime.Valid || !store.rangeParams.StartTime.Time.Equal(start) || !store.rangeParams.EndTime.Time.Equal(end) {
		t.Fatalf("range conversion failed: %+v, %v", rows, err)
	}
	users, err := r.NewUsersInRange(context.Background(), start, end)
	if err != nil || len(users) != 1 || !users[0].Enabled.Valid || users[0].Enabled.Bool || users[0].FirstName.Valid || !reflect.DeepEqual(users[0].Roles, []string{"user"}) {
		t.Fatalf("record conversion failed: %+v, %v", users, err)
	}
	store.err = errors.New("query failed")
	if rows, err := r.LoginTrendByDay(context.Background(), start, end); rows != nil || !errors.Is(err, store.err) {
		t.Fatalf("query error lost: %+v, %v", rows, err)
	}
}
