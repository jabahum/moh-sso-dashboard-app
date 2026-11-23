package visualiser

import "context"

type Service interface {
	ListDatasets(ctx context.Context) ([]DatasetResponse, error)
	ListDataElements(ctx context.Context, dataSetID *string) ([]DataElementResponse, error)
	ListDataValues(ctx context.Context, req DataValuesRequest) (DataValuesResponse, error)
}

type service struct {
	repository Repository
}

func NewService(repository Repository) Service {
	return &service{repository: repository}
}

func (s *service) ListDatasets(ctx context.Context) ([]DatasetResponse, error) {
	return s.repository.ListDatasets(ctx)
}

func (s *service) ListDataElements(ctx context.Context, dataSetID *string) ([]DataElementResponse, error) {
	return s.repository.ListDataElements(ctx, dataSetID)
}

func (s *service) ListDataValues(ctx context.Context, req DataValuesRequest) (DataValuesResponse, error) {
	return s.repository.ListDataValues(ctx, req)
}
