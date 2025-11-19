package logger

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/9triver/iarnet/internal/infra/repository/application"
)

type Service interface {
	SubmitLog(ctx context.Context, applicationID string, entry *Entry) (*SubmitLogResult, error)
	BatchSubmitLogs(ctx context.Context, applicationID string, entries []*Entry) (*BatchSubmitLogResult, error)
	GetLogs(ctx context.Context, applicationID string, options *QueryOptions) (*QueryResult, error)
	GetLogsByTimeRange(ctx context.Context, applicationID string, startTime, endTime time.Time, limit int) ([]*Entry, error)
}

type service struct {
	repo application.LoggerRepo
}

func NewService(repo application.LoggerRepo) Service {
	return &service{
		repo: repo,
	}
}

func (s *service) SubmitLog(ctx context.Context, applicationID string, entry *Entry) (*SubmitLogResult, error) {
	if applicationID == "" {
		return nil, errors.New("application ID is required")
	}
	if entry == nil {
		return nil, errors.New("log entry is required")
	}

	// 转换为 DAO
	dao := domainEntryToDAO(applicationID, entry)

	// 保存到数据库
	if err := s.repo.SaveLog(ctx, dao); err != nil {
		return &SubmitLogResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &SubmitLogResult{
		Success: true,
		LogID:   dao.ID,
	}, nil
}

func (s *service) BatchSubmitLogs(ctx context.Context, applicationID string, entries []*Entry) (*BatchSubmitLogResult, error) {
	if applicationID == "" {
		return nil, errors.New("application ID is required")
	}
	if len(entries) == 0 {
		return nil, errors.New("log entries are required")
	}

	// 转换为 DAO
	daos := domainEntriesToDAOs(applicationID, entries)

	// 批量保存到数据库
	if err := s.repo.BatchSaveLogs(ctx, daos); err != nil {
		return &BatchSubmitLogResult{
			Success:       false,
			Error:         err.Error(),
			AcceptedCount: 0,
			RejectedCount: len(entries),
		}, nil
	}

	// 提取日志 ID
	logIDs := make([]string, len(daos))
	for i, dao := range daos {
		logIDs[i] = dao.ID
	}

	return &BatchSubmitLogResult{
		Success:       true,
		AcceptedCount: len(entries),
		RejectedCount: 0,
		LogIDs:        logIDs,
	}, nil
}

func (s *service) GetLogs(ctx context.Context, applicationID string, options *QueryOptions) (*QueryResult, error) {
	if applicationID == "" {
		return nil, errors.New("application ID is required")
	}

	// 设置默认值
	if options == nil {
		options = &QueryOptions{}
	}
	if options.Limit <= 0 {
		options.Limit = 100 // 默认每页 100 条
	}
	if options.Offset < 0 {
		options.Offset = 0
	}

	// 如果有时间范围，使用时间范围查询
	if options.StartTime != nil && options.EndTime != nil {
		daos, err := s.repo.GetLogsByTimeRange(ctx, applicationID, *options.StartTime, *options.EndTime, options.Limit)
		if err != nil {
			return nil, fmt.Errorf("failed to get logs by time range: %w", err)
		}

		// 过滤日志级别（如果指定）
		if options.Level != "" {
			daos = filterLogsByLevel(daos, options.Level)
		}

		// 转换为 domain 类型
		entries, err := daosToDomainEntries(daos)
		if err != nil {
			return nil, fmt.Errorf("failed to convert daos to entries: %w", err)
		}

		return &QueryResult{
			Entries: entries,
			Total:   len(entries),
			HasMore: len(entries) >= options.Limit,
		}, nil
	}

	// 使用分页查询
	daos, err := s.repo.GetLogs(ctx, applicationID, options.Limit, options.Offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get logs: %w", err)
	}

	// 过滤日志级别（如果指定）
	if options.Level != "" {
		daos = filterLogsByLevel(daos, options.Level)
	}

	// 转换为 domain 类型
	entries, err := daosToDomainEntries(daos)
	if err != nil {
		return nil, fmt.Errorf("failed to convert daos to entries: %w", err)
	}

	// 判断是否还有更多数据
	hasMore := len(entries) >= options.Limit

	return &QueryResult{
		Entries: entries,
		Total:   len(entries),
		HasMore: hasMore,
	}, nil
}

func (s *service) GetLogsByTimeRange(ctx context.Context, applicationID string, startTime, endTime time.Time, limit int) ([]*Entry, error) {
	if applicationID == "" {
		return nil, errors.New("application ID is required")
	}
	if startTime.After(endTime) {
		return nil, errors.New("start time must be before end time")
	}
	if limit <= 0 {
		limit = 100 // 默认限制
	}

	// 查询 DAO
	daos, err := s.repo.GetLogsByTimeRange(ctx, applicationID, startTime, endTime, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get logs by time range: %w", err)
	}

	// 转换为 domain 类型
	entries, err := daosToDomainEntries(daos)
	if err != nil {
		return nil, fmt.Errorf("failed to convert daos to entries: %w", err)
	}

	return entries, nil
}

// filterLogsByLevel 根据日志级别过滤日志
func filterLogsByLevel(daos []*application.LogEntryDAO, level LogLevel) []*application.LogEntryDAO {
	if level == "" {
		return daos
	}

	filtered := make([]*application.LogEntryDAO, 0, len(daos))
	levelStr := string(level)
	for _, dao := range daos {
		if dao.Level == levelStr {
			filtered = append(filtered, dao)
		}
	}
	return filtered
}
