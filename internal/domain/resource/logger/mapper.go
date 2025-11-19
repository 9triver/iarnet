package logger

import (
	"encoding/json"
	"fmt"

	resourceRepo "github.com/9triver/iarnet/internal/infra/repository/resource"
	"github.com/9triver/iarnet/internal/util"
)

// domainEntryToDAO 将 domain Entry 转换为资源日志 DAO
func domainEntryToDAO(componentID string, entry *Entry) *resourceRepo.LogEntryDAO {
	dao := &resourceRepo.LogEntryDAO{
		ID:          util.GenIDWith("resource-log."),
		ComponentID: componentID,
		Timestamp:   entry.Timestamp,
		Level:       string(entry.Level),
		Message:     entry.Message,
	}

	if len(entry.Fields) > 0 {
		fieldsJSON, err := json.Marshal(entry.Fields)
		if err != nil {
			fieldsJSON = []byte("[]")
		}
		dao.Fields = string(fieldsJSON)
	}

	if entry.Caller != nil {
		dao.CallerFile = entry.Caller.File
		dao.CallerLine = entry.Caller.Line
		dao.CallerFunc = entry.Caller.Function
	}

	return dao
}

// daoToDomainEntry 将 DAO 转换为 domain Entry
func daoToDomainEntry(dao *resourceRepo.LogEntryDAO) (*Entry, error) {
	entry := &Entry{
		Timestamp: dao.Timestamp,
		Level:     LogLevel(dao.Level),
		Message:   dao.Message,
	}

	if dao.Fields != "" {
		var fields []LogField
		if err := json.Unmarshal([]byte(dao.Fields), &fields); err != nil {
			return nil, fmt.Errorf("failed to unmarshal fields: %w", err)
		}
		entry.Fields = fields
	}

	if dao.CallerFile != "" || dao.CallerLine > 0 || dao.CallerFunc != "" {
		entry.Caller = &CallerInfo{
			File:     dao.CallerFile,
			Line:     dao.CallerLine,
			Function: dao.CallerFunc,
		}
	}

	return entry, nil
}

// daosToDomainEntries 批量将 DAO 转换为 domain Entry
func daosToDomainEntries(daos []*resourceRepo.LogEntryDAO) ([]*Entry, error) {
	entries := make([]*Entry, len(daos))
	for i, dao := range daos {
		entry, err := daoToDomainEntry(dao)
		if err != nil {
			return nil, fmt.Errorf("failed to convert dao to entry at index %d: %w", i, err)
		}
		entries[i] = entry
	}
	return entries, nil
}
