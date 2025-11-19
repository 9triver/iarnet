package logger

import (
	"encoding/json"
	"fmt"

	"github.com/9triver/iarnet/internal/infra/repository/application"
	"github.com/9triver/iarnet/internal/util"
)

// domainEntryToDAO 将 domain Entry 转换为 DAO
func domainEntryToDAO(applicationID string, entry *Entry) *application.LogEntryDAO {
	dao := &application.LogEntryDAO{
		ID:            util.GenIDWith("log."),
		ApplicationID: applicationID,
		Timestamp:     entry.Timestamp,
		Level:         string(entry.Level),
		Message:       entry.Message,
	}

	// 转换 Fields 为 JSON
	if len(entry.Fields) > 0 {
		fieldsJSON, err := json.Marshal(entry.Fields)
		if err != nil {
			// 如果序列化失败，记录错误但不中断流程
			fieldsJSON = []byte("[]")
		}
		dao.Fields = string(fieldsJSON)
	}

	// 转换 Caller
	if entry.Caller != nil {
		dao.CallerFile = entry.Caller.File
		dao.CallerLine = entry.Caller.Line
		dao.CallerFunc = entry.Caller.Function
	}

	return dao
}

// domainEntriesToDAOs 批量将 domain Entry 转换为 DAO
func domainEntriesToDAOs(applicationID string, entries []*Entry) []*application.LogEntryDAO {
	daos := make([]*application.LogEntryDAO, len(entries))
	for i, entry := range entries {
		daos[i] = domainEntryToDAO(applicationID, entry)
	}
	return daos
}

// daoToDomainEntry 将 DAO 转换为 domain Entry
func daoToDomainEntry(dao *application.LogEntryDAO) (*Entry, error) {
	entry := &Entry{
		Timestamp: dao.Timestamp,
		Level:     LogLevel(dao.Level),
		Message:   dao.Message,
	}

	// 解析 Fields JSON
	if dao.Fields != "" {
		var fields []LogField
		if err := json.Unmarshal([]byte(dao.Fields), &fields); err != nil {
			return nil, fmt.Errorf("failed to unmarshal fields: %w", err)
		}
		entry.Fields = fields
	}

	// 转换 Caller
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
func daosToDomainEntries(daos []*application.LogEntryDAO) ([]*Entry, error) {
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
