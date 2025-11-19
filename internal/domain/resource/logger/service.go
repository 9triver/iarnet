package logger

import (
	"context"
	"errors"
	"fmt"
	"time"

	resourceRepo "github.com/9triver/iarnet/internal/infra/repository/resource"
)

type Service interface {
	SubmitLog(ctx context.Context, componentID string, entry *Entry) (LogID, error)
	GetLogs(ctx context.Context, componentID string, options *QueryOptions) (*QueryResult, error)
	GetLogsByTimeRange(ctx context.Context, componentID string, startTime, endTime time.Time, limit int) ([]*Entry, error)
}

type service struct {
	repo resourceRepo.LoggerRepo
}

func NewService(repo resourceRepo.LoggerRepo) Service {
	return &service{
		repo: repo,
	}
}

func (s *service) SubmitLog(ctx context.Context, componentID string, entry *Entry) (LogID, error) {
	if componentID == "" {
		return "", errors.New("component ID is required")
	}
	if entry == nil {
		return "", errors.New("log entry is required")
	}

	dao := domainEntryToDAO(componentID, entry)

	if err := s.repo.SaveLog(ctx, dao); err != nil {
		return "", err
	}

	return dao.ID, nil
}

func (s *service) GetLogs(ctx context.Context, componentID string, options *QueryOptions) (*QueryResult, error) {
	if options == nil {
		options = &QueryOptions{}
	}
	if options.Limit <= 0 {
		options.Limit = 100
	}
	if options.Offset < 0 {
		options.Offset = 0
	}

	var (
		daos []*resourceRepo.LogEntryDAO
		err  error
	)

	if options.StartTime != nil && options.EndTime != nil {
		daos, err = s.repo.GetLogsByTimeRange(ctx, componentID, *options.StartTime, *options.EndTime, options.Limit)
	} else {
		daos, err = s.repo.GetLogs(ctx, componentID, options.Limit, options.Offset)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get logs: %w", err)
	}

	if options.Level != "" {
		daos = filterLogsByLevel(daos, options.Level)
	}

	entries, err := daosToDomainEntries(daos)
	if err != nil {
		return nil, fmt.Errorf("failed to convert daos to entries: %w", err)
	}

	hasMore := len(entries) >= options.Limit

	return &QueryResult{
		Entries: entries,
		Total:   len(entries),
		HasMore: hasMore,
	}, nil
}

func (s *service) GetLogsByTimeRange(ctx context.Context, componentID string, startTime, endTime time.Time, limit int) ([]*Entry, error) {
	if componentID == "" {
		return nil, errors.New("component ID is required")
	}
	if startTime.After(endTime) {
		return nil, errors.New("start time must be before end time")
	}
	if limit <= 0 {
		limit = 100
	}

	daos, err := s.repo.GetLogsByTimeRange(ctx, componentID, startTime, endTime, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get logs by time range: %w", err)
	}

	entries, err := daosToDomainEntries(daos)
	if err != nil {
		return nil, fmt.Errorf("failed to convert daos to entries: %w", err)
	}

	return entries, nil
}

func filterLogsByLevel(daos []*resourceRepo.LogEntryDAO, level LogLevel) []*resourceRepo.LogEntryDAO {
	if level == "" {
		return daos
	}

	levelStr := string(level)
	filtered := make([]*resourceRepo.LogEntryDAO, 0, len(daos))
	for _, dao := range daos {
		if dao.Level == levelStr {
			filtered = append(filtered, dao)
		}
	}
	return filtered
}
