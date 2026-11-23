package surveillance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	db "github.com/mohuganda/moh-sso-dashboard/backend/modules/surveillance/internal/postgres"
	"log/slog"
)

var (
	ErrWeeklyStatusIDRequired = errors.New("weekly status id is required")
	ErrEpiWeekIDRequired      = errors.New("epi week id is required")
	ErrRegionIDRequired       = errors.New("region id is required")
	ErrDistrictIDRequired     = errors.New("district id is required")
	ErrSubCountyIDRequired    = errors.New("sub county id is required")
)

type WeeklyStatusService struct {
	log  *slog.Logger
	repo WeeklyStatusRepository
}

func NewWeeklyStatusService(
	log *slog.Logger,
	repo WeeklyStatusRepository,
) *WeeklyStatusService {
	return &WeeklyStatusService{
		log:  log,
		repo: repo,
	}
}

func (s *WeeklyStatusService) Create(
	ctx context.Context,
	arg db.CreateWeeklyStatusParams,
) (db.WeeklyStatus, error) {
	s.log.Info("creating weekly status")

	status, err := s.repo.Create(ctx, arg)
	if err != nil {
		s.log.Error("failed to create weekly status", "error", err)
		return db.WeeklyStatus{}, fmt.Errorf("create weekly status: %w", err)
	}

	s.log.Info("weekly status created", "weeklyStatusID", status.ID.String())
	return status, nil
}

func (s *WeeklyStatusService) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (db.WeeklyStatus, error) {
	if id == uuid.Nil {
		s.log.Warn("get weekly status failed: missing weekly status id")
		return db.WeeklyStatus{}, ErrWeeklyStatusIDRequired
	}

	s.log.Info("fetching weekly status by id", "weeklyStatusID", id.String())

	status, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.log.Error("failed to fetch weekly status by id", "weeklyStatusID", id.String(), "error", err)
		return db.WeeklyStatus{}, fmt.Errorf("get weekly status by id: %w", err)
	}

	s.log.Info("weekly status fetched", "weeklyStatusID", id.String())
	return status, nil
}

func (s *WeeklyStatusService) Delete(
	ctx context.Context,
	id uuid.UUID,
) error {
	if id == uuid.Nil {
		s.log.Warn("delete weekly status failed: missing weekly status id")
		return ErrWeeklyStatusIDRequired
	}

	s.log.Info("deleting weekly status", "weeklyStatusID", id.String())

	if err := s.repo.Delete(ctx, id); err != nil {
		s.log.Error("failed to delete weekly status", "weeklyStatusID", id.String(), "error", err)
		return fmt.Errorf("delete weekly status: %w", err)
	}

	s.log.Info("weekly status deleted", "weeklyStatusID", id.String())
	return nil
}

func (s *WeeklyStatusService) List(
	ctx context.Context,
	arg db.ListWeeklyStatusesParams,
) ([]db.WeeklyStatus, error) {
	s.log.Info("listing weekly statuses")

	rows, err := s.repo.List(ctx, arg)
	if err != nil {
		s.log.Error("failed to list weekly statuses", "error", err)
		return nil, fmt.Errorf("list weekly statuses: %w", err)
	}

	s.log.Info("weekly statuses listed", "count", len(rows))
	return rows, nil
}

func (s *WeeklyStatusService) ListFromInput(
	ctx context.Context,
	input WeeklyStatusListInput,
) ([]db.WeeklyStatus, error) {
	return s.List(ctx, db.ListWeeklyStatusesParams{
		EpiWeekID:   uuidNull(input.EpiWeekID),
		RegionID:    uuidNull(input.RegionID),
		DistrictID:  uuidNull(input.DistrictID),
		SubCountyID: uuidNull(input.SubCountyID),
		DiseaseID:   uuidNull(input.DiseaseID),
		IndicatorID: uuidNull(input.IndicatorID),
		Status:      riskLevelNull(input.Status),
	})
}

func (s *WeeklyStatusService) ListFromInputInHealthContext(
	ctx context.Context,
	input WeeklyStatusListInput,
	scope HealthContextScope,
) ([]db.WeeklyStatus, error) {
	return s.repo.ListInHealthContext(ctx, db.ListWeeklyStatusesParams{
		EpiWeekID: uuidNull(input.EpiWeekID), RegionID: uuidNull(input.RegionID),
		DistrictID: uuidNull(input.DistrictID), SubCountyID: uuidNull(input.SubCountyID),
		DiseaseID: uuidNull(input.DiseaseID), IndicatorID: uuidNull(input.IndicatorID),
		Status: riskLevelNull(input.Status),
	}, scope)
}

func (s *WeeklyStatusService) ListDetailed(
	ctx context.Context,
	arg db.ListWeeklyStatusesDetailedParams,
) ([]db.ListWeeklyStatusesDetailedRow, error) {
	s.log.Info("listing detailed weekly statuses")

	rows, err := s.repo.ListDetailed(ctx, arg)
	if err != nil {
		s.log.Error("failed to list detailed weekly statuses", "error", err)
		return nil, fmt.Errorf("list detailed weekly statuses: %w", err)
	}

	s.log.Info("detailed weekly statuses listed", "count", len(rows))
	return rows, nil
}

func (s *WeeklyStatusService) ListDetailedFromInput(
	ctx context.Context,
	input WeeklyStatusListInput,
) ([]db.ListWeeklyStatusesDetailedRow, error) {
	return s.ListDetailed(ctx, db.ListWeeklyStatusesDetailedParams{
		EpiWeekID:   uuidNull(input.EpiWeekID),
		RegionID:    uuidNull(input.RegionID),
		DistrictID:  uuidNull(input.DistrictID),
		SubCountyID: uuidNull(input.SubCountyID),
		DiseaseID:   uuidNull(input.DiseaseID),
		IndicatorID: uuidNull(input.IndicatorID),
		Status:      riskLevelNull(input.Status),
	})
}

func (s *WeeklyStatusService) ListDetailedFromInputInHealthContext(
	ctx context.Context,
	input WeeklyStatusListInput,
	scope HealthContextScope,
) ([]db.ListWeeklyStatusesDetailedRow, error) {
	return s.repo.ListDetailedInHealthContext(ctx, db.ListWeeklyStatusesDetailedParams{
		EpiWeekID: uuidNull(input.EpiWeekID), RegionID: uuidNull(input.RegionID),
		DistrictID: uuidNull(input.DistrictID), SubCountyID: uuidNull(input.SubCountyID),
		DiseaseID: uuidNull(input.DiseaseID), IndicatorID: uuidNull(input.IndicatorID),
		Status: riskLevelNull(input.Status),
	}, scope)
}

func (s *WeeklyStatusService) ListLevelByWeekInHealthContext(
	ctx context.Context,
	epiWeekID uuid.UUID,
	level string,
	scope HealthContextScope,
) ([]db.WeeklyStatus, error) {
	rows, err := s.ListFromInputInHealthContext(ctx, WeeklyStatusListInput{EpiWeekID: epiWeekID}, scope)
	if err != nil {
		return nil, err
	}
	filtered := make([]db.WeeklyStatus, 0, len(rows))
	for _, row := range rows {
		matches := false
		switch level {
		case "national":
			matches = !row.RegionID.Valid && !row.DistrictID.Valid && !row.SubCountyID.Valid
		case "region":
			matches = row.RegionID.Valid && !row.DistrictID.Valid && !row.SubCountyID.Valid
		case "district":
			matches = row.RegionID.Valid && row.DistrictID.Valid && !row.SubCountyID.Valid
		}
		if matches {
			filtered = append(filtered, row)
		}
	}
	return filtered, nil
}

func (s *WeeklyStatusService) ListByWeek(
	ctx context.Context,
	epiWeekID uuid.UUID,
) ([]db.WeeklyStatus, error) {
	if epiWeekID == uuid.Nil {
		s.log.Warn("list weekly statuses by week failed: missing epi week id")
		return nil, ErrEpiWeekIDRequired
	}

	s.log.Info("listing weekly statuses by week", "epiWeekID", epiWeekID.String())

	rows, err := s.repo.ListByWeek(ctx, epiWeekID)
	if err != nil {
		s.log.Error("failed to list weekly statuses by week", "epiWeekID", epiWeekID.String(), "error", err)
		return nil, fmt.Errorf("list weekly statuses by week: %w", err)
	}

	s.log.Info("weekly statuses by week listed", "epiWeekID", epiWeekID.String(), "count", len(rows))
	return rows, nil
}

func (s *WeeklyStatusService) ListByRegion(
	ctx context.Context,
	regionID uuid.UUID,
) ([]db.WeeklyStatus, error) {
	if regionID == uuid.Nil {
		s.log.Warn("list weekly statuses by region failed: missing region id")
		return nil, ErrRegionIDRequired
	}

	s.log.Info("listing weekly statuses by region", "regionID", regionID.String())

	rows, err := s.repo.ListByRegion(ctx, regionID)
	if err != nil {
		s.log.Error("failed to list weekly statuses by region", "regionID", regionID.String(), "error", err)
		return nil, fmt.Errorf("list weekly statuses by region: %w", err)
	}

	s.log.Info("weekly statuses by region listed", "regionID", regionID.String(), "count", len(rows))
	return rows, nil
}

func (s *WeeklyStatusService) ListByDistrict(
	ctx context.Context,
	districtID uuid.UUID,
) ([]db.WeeklyStatus, error) {
	if districtID == uuid.Nil {
		s.log.Warn("list weekly statuses by district failed: missing district id")
		return nil, ErrDistrictIDRequired
	}

	s.log.Info("listing weekly statuses by district", "districtID", districtID.String())

	rows, err := s.repo.ListByDistrict(ctx, districtID)
	if err != nil {
		s.log.Error("failed to list weekly statuses by district", "districtID", districtID.String(), "error", err)
		return nil, fmt.Errorf("list weekly statuses by district: %w", err)
	}

	s.log.Info("weekly statuses by district listed", "districtID", districtID.String(), "count", len(rows))
	return rows, nil
}

func (s *WeeklyStatusService) ListBySubCounty(
	ctx context.Context,
	subCountyID uuid.UUID,
) ([]db.WeeklyStatus, error) {
	if subCountyID == uuid.Nil {
		s.log.Warn("list weekly statuses by sub county failed: missing sub county id")
		return nil, ErrSubCountyIDRequired
	}

	s.log.Info("listing weekly statuses by sub county", "subCountyID", subCountyID.String())

	rows, err := s.repo.ListBySubCounty(ctx, subCountyID)
	if err != nil {
		s.log.Error("failed to list weekly statuses by sub county", "subCountyID", subCountyID.String(), "error", err)
		return nil, fmt.Errorf("list weekly statuses by sub county: %w", err)
	}

	s.log.Info("weekly statuses by sub county listed", "subCountyID", subCountyID.String(), "count", len(rows))
	return rows, nil
}

func (s *WeeklyStatusService) ListNationalByWeek(
	ctx context.Context,
	epiWeekID uuid.UUID,
) ([]db.WeeklyStatus, error) {
	if epiWeekID == uuid.Nil {
		s.log.Warn("list national weekly statuses by week failed: missing epi week id")
		return nil, ErrEpiWeekIDRequired
	}

	s.log.Info("listing national weekly statuses by week", "epiWeekID", epiWeekID.String())

	rows, err := s.repo.ListNationalByWeek(ctx, epiWeekID)
	if err != nil {
		s.log.Error("failed to list national weekly statuses by week", "epiWeekID", epiWeekID.String(), "error", err)
		return nil, fmt.Errorf("list national weekly statuses by week: %w", err)
	}

	s.log.Info("national weekly statuses by week listed", "epiWeekID", epiWeekID.String(), "count", len(rows))
	return rows, nil
}

func (s *WeeklyStatusService) ListRegionByWeek(
	ctx context.Context,
	epiWeekID uuid.UUID,
) ([]db.WeeklyStatus, error) {
	if epiWeekID == uuid.Nil {
		s.log.Warn("list region weekly statuses by week failed: missing epi week id")
		return nil, ErrEpiWeekIDRequired
	}

	s.log.Info("listing region weekly statuses by week", "epiWeekID", epiWeekID.String())

	rows, err := s.repo.ListRegionByWeek(ctx, epiWeekID)
	if err != nil {
		s.log.Error("failed to list region weekly statuses by week", "epiWeekID", epiWeekID.String(), "error", err)
		return nil, fmt.Errorf("list region weekly statuses by week: %w", err)
	}

	s.log.Info("region weekly statuses by week listed", "epiWeekID", epiWeekID.String(), "count", len(rows))
	return rows, nil
}

func (s *WeeklyStatusService) ListDistrictByWeek(
	ctx context.Context,
	epiWeekID uuid.UUID,
) ([]db.WeeklyStatus, error) {
	if epiWeekID == uuid.Nil {
		s.log.Warn("list district weekly statuses by week failed: missing epi week id")
		return nil, ErrEpiWeekIDRequired
	}

	s.log.Info("listing district weekly statuses by week", "epiWeekID", epiWeekID.String())

	rows, err := s.repo.ListDistrictByWeek(ctx, epiWeekID)
	if err != nil {
		s.log.Error("failed to list district weekly statuses by week", "epiWeekID", epiWeekID.String(), "error", err)
		return nil, fmt.Errorf("list district weekly statuses by week: %w", err)
	}

	s.log.Info("district weekly statuses by week listed", "epiWeekID", epiWeekID.String(), "count", len(rows))
	return rows, nil
}

func (s *WeeklyStatusService) ListSubCountyByWeek(
	ctx context.Context,
	epiWeekID uuid.UUID,
) ([]db.WeeklyStatus, error) {
	if epiWeekID == uuid.Nil {
		s.log.Warn("list subcounty weekly statuses by week failed: missing epi week id")
		return nil, ErrEpiWeekIDRequired
	}

	s.log.Info("listing subcounty weekly statuses by week", "epiWeekID", epiWeekID.String())

	rows, err := s.repo.ListSubCountyByWeek(ctx, epiWeekID)
	if err != nil {
		s.log.Error("failed to list subcounty weekly statuses by week", "epiWeekID", epiWeekID.String(), "error", err)
		return nil, fmt.Errorf("list subcounty weekly statuses by week: %w", err)
	}

	s.log.Info("subcounty weekly statuses by week listed", "epiWeekID", epiWeekID.String(), "count", len(rows))
	return rows, nil
}

func (s *WeeklyStatusService) UpsertNationalDisease(
	ctx context.Context,
	arg db.UpsertNationalDiseaseWeeklyStatusParams,
) (db.WeeklyStatus, error) {
	s.log.Info("upserting national disease weekly status", "epiWeekID", arg.EpiWeekID.String())

	status, err := s.repo.UpsertNationalDisease(ctx, arg)
	if err != nil {
		s.log.Error("failed to upsert national disease weekly status", "epiWeekID", arg.EpiWeekID.String(), "error", err)
		return db.WeeklyStatus{}, fmt.Errorf("upsert national disease weekly status: %w", err)
	}

	s.log.Info("national disease weekly status upserted", "weeklyStatusID", status.ID.String())
	return status, nil
}

func (s *WeeklyStatusService) UpsertNationalIndicator(
	ctx context.Context,
	arg db.UpsertNationalIndicatorWeeklyStatusParams,
) (db.WeeklyStatus, error) {
	s.log.Info("upserting national indicator weekly status", "epiWeekID", arg.EpiWeekID.String())

	status, err := s.repo.UpsertNationalIndicator(ctx, arg)
	if err != nil {
		s.log.Error("failed to upsert national indicator weekly status", "epiWeekID", arg.EpiWeekID.String(), "error", err)
		return db.WeeklyStatus{}, fmt.Errorf("upsert national indicator weekly status: %w", err)
	}

	s.log.Info("national indicator weekly status upserted", "weeklyStatusID", status.ID.String())
	return status, nil
}

func (s *WeeklyStatusService) UpsertRegionDisease(
	ctx context.Context,
	arg db.UpsertRegionDiseaseWeeklyStatusParams,
) (db.WeeklyStatus, error) {
	s.log.Info("upserting region disease weekly status", "regionID", arg.RegionID.UUID.String(), "epiWeekID", arg.EpiWeekID.String())

	status, err := s.repo.UpsertRegionDisease(ctx, arg)
	if err != nil {
		s.log.Error("failed to upsert region disease weekly status", "regionID", arg.RegionID.UUID.String(), "epiWeekID", arg.EpiWeekID.String(), "error", err)
		return db.WeeklyStatus{}, fmt.Errorf("upsert region disease weekly status: %w", err)
	}

	s.log.Info("region disease weekly status upserted", "weeklyStatusID", status.ID.String())
	return status, nil
}

func (s *WeeklyStatusService) UpsertRegionIndicator(
	ctx context.Context,
	arg db.UpsertRegionIndicatorWeeklyStatusParams,
) (db.WeeklyStatus, error) {
	s.log.Info("upserting region indicator weekly status", "regionID", arg.RegionID.UUID.String(), "epiWeekID", arg.EpiWeekID.String())

	status, err := s.repo.UpsertRegionIndicator(ctx, arg)
	if err != nil {
		s.log.Error("failed to upsert region indicator weekly status", "regionID", arg.RegionID.UUID.String(), "epiWeekID", arg.EpiWeekID.String(), "error", err)
		return db.WeeklyStatus{}, fmt.Errorf("upsert region indicator weekly status: %w", err)
	}

	s.log.Info("region indicator weekly status upserted", "weeklyStatusID", status.ID.String())
	return status, nil
}

func (s *WeeklyStatusService) UpsertDistrictDisease(
	ctx context.Context,
	arg db.UpsertDistrictDiseaseWeeklyStatusParams,
) (db.WeeklyStatus, error) {
	s.log.Info("upserting district disease weekly status", "districtID", arg.DistrictID.UUID.String(), "epiWeekID", arg.EpiWeekID.String())

	status, err := s.repo.UpsertDistrictDisease(ctx, arg)
	if err != nil {
		s.log.Error("failed to upsert district disease weekly status", "districtID", arg.DistrictID.UUID.String(), "epiWeekID", arg.EpiWeekID.String(), "error", err)
		return db.WeeklyStatus{}, fmt.Errorf("upsert district disease weekly status: %w", err)
	}

	s.log.Info("district disease weekly status upserted", "weeklyStatusID", status.ID.String())
	return status, nil
}

func (s *WeeklyStatusService) UpsertDistrictIndicator(
	ctx context.Context,
	arg db.UpsertDistrictIndicatorWeeklyStatusParams,
) (db.WeeklyStatus, error) {
	s.log.Info("upserting district indicator weekly status", "districtID", arg.DistrictID.UUID.String(), "epiWeekID", arg.EpiWeekID.String())

	status, err := s.repo.UpsertDistrictIndicator(ctx, arg)
	if err != nil {
		s.log.Error("failed to upsert district indicator weekly status", "districtID", arg.DistrictID.UUID.String(), "epiWeekID", arg.EpiWeekID.String(), "error", err)
		return db.WeeklyStatus{}, fmt.Errorf("upsert district indicator weekly status: %w", err)
	}

	s.log.Info("district indicator weekly status upserted", "weeklyStatusID", status.ID.String())
	return status, nil
}

func (s *WeeklyStatusService) UpsertSubCountyDisease(
	ctx context.Context,
	arg db.UpsertSubCountyDiseaseWeeklyStatusParams,
) (db.WeeklyStatus, error) {
	s.log.Info("upserting subcounty disease weekly status", "subCountyID", arg.SubCountyID.UUID.String(), "epiWeekID", arg.EpiWeekID.String())

	status, err := s.repo.UpsertSubCountyDisease(ctx, arg)
	if err != nil {
		s.log.Error("failed to upsert subcounty disease weekly status", "subCountyID", arg.SubCountyID.UUID.String(), "epiWeekID", arg.EpiWeekID.String(), "error", err)
		return db.WeeklyStatus{}, fmt.Errorf("upsert subcounty disease weekly status: %w", err)
	}

	s.log.Info("subcounty disease weekly status upserted", "weeklyStatusID", status.ID.String())
	return status, nil
}

func (s *WeeklyStatusService) UpsertSubCountyIndicator(
	ctx context.Context,
	arg db.UpsertSubCountyIndicatorWeeklyStatusParams,
) (db.WeeklyStatus, error) {
	s.log.Info("upserting subcounty indicator weekly status", "subCountyID", arg.SubCountyID.UUID.String(), "epiWeekID", arg.EpiWeekID.String())

	status, err := s.repo.UpsertSubCountyIndicator(ctx, arg)
	if err != nil {
		s.log.Error("failed to upsert subcounty indicator weekly status", "subCountyID", arg.SubCountyID.UUID.String(), "epiWeekID", arg.EpiWeekID.String(), "error", err)
		return db.WeeklyStatus{}, fmt.Errorf("upsert subcounty indicator weekly status: %w", err)
	}

	s.log.Info("subcounty indicator weekly status upserted", "weeklyStatusID", status.ID.String())
	return status, nil
}

func (s *WeeklyStatusService) WithTx(
	ctx context.Context,
	fn func(q db.Querier) error,
) error {
	s.log.Info("starting weekly status transaction")

	if err := s.repo.WithTx(ctx, fn); err != nil {
		s.log.Error("weekly status transaction failed", "error", err)
		return fmt.Errorf("weekly status transaction: %w", err)
	}

	s.log.Info("weekly status transaction completed")
	return nil
}

func parseInt32Field(raw json.RawMessage, fieldName string) (int32, error) {
	raw = bytesTrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return 0, fmt.Errorf("%s is required", fieldName)
	}

	var asInt int32
	if err := json.Unmarshal(raw, &asInt); err == nil {
		return asInt, nil
	}

	var asFloat float64
	if err := json.Unmarshal(raw, &asFloat); err == nil {
		return int32(asFloat), nil
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		asString = strings.TrimSpace(asString)
		if asString == "" {
			return 0, fmt.Errorf("%s is required", fieldName)
		}

		n, err := strconv.ParseFloat(asString, 64)
		if err != nil {
			return 0, fmt.Errorf("%s must be a valid number", fieldName)
		}

		return int32(n), nil
	}

	return 0, fmt.Errorf("%s must be a valid number", fieldName)
}

func parseStringField(raw json.RawMessage, field string) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", fmt.Errorf("%s is required", field)
	}

	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be a string", field)
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s is required", field)
	}

	return value, nil
}

func parseStringSliceField(raw json.RawMessage, field string) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}

	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return normalizeDiseaseNames(arr), nil
	}

	var str string
	if err := json.Unmarshal(raw, &str); err != nil {
		return nil, fmt.Errorf("%s has invalid format", field)
	}

	str = strings.TrimSpace(str)
	if str == "" || str == "[]" {
		return nil, nil
	}

	normalized := normalizeArrayString(str)
	if normalized == "[]" {
		return nil, nil
	}

	if err := json.Unmarshal([]byte(normalized), &arr); err != nil {
		return nil, fmt.Errorf("%s has invalid list format: %w", field, err)
	}

	return normalizeDiseaseNames(arr), nil
}

func normalizeArrayString(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, `'`, `"`)
	return value
}

func normalizeDiseaseNames(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))

	for _, value := range values {
		name := strings.TrimSpace(value)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}

	return result
}

func requireUUID(name string, value uuid.UUID) error {
	if value == uuid.Nil {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

func parseOptionalInt32Field(raw json.RawMessage) (int32, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, nil
	}

	var num int32
	if err := json.Unmarshal(raw, &num); err == nil {
		return num, nil
	}

	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		str = strings.TrimSpace(str)
		if str == "" {
			return 0, nil
		}

		value, convErr := strconv.ParseInt(str, 10, 32)
		if convErr != nil {
			return 0, convErr
		}
		return int32(value), nil
	}

	return 0, fmt.Errorf("unsupported value")
}

func resolvePayloadOrBatchYear(payloadYear, batchYear int32, allowCurrentYearFallback bool) (int32, error) {
	if payloadYear > 0 {
		return payloadYear, nil
	}

	if batchYear > 0 {
		return batchYear, nil
	}

	if allowCurrentYearFallback {
		return int32(time.Now().Year()), nil
	}

	return 0, errors.New("year is required")
}

func resolveBatchYear(batch db.SurveillanceImportBatch) (int32, error) {

	candidates := []string{
		strings.TrimSpace(batch.FileName.String),
		strings.TrimSpace(batch.SourceName),
	}

	if batch.Notes.Valid {
		candidates = append(candidates, strings.TrimSpace(batch.Notes.String))
	}

	for _, candidate := range candidates {
		if year := extractYear(candidate); year > 0 {
			return year, nil
		}
	}

	return 0, nil
}

func extractYear(value string) int32 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}

	for year := 2035; year >= 2000; year-- {
		token := strconv.Itoa(year)
		if strings.Contains(value, token) {
			return int32(year)
		}
	}

	return 0
}
