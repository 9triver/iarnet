package logger

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lithammer/shortuuid/v4"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

// Storage 日志存储接口
type Storage interface {
	// Write 写入日志条目
	Write(ctx context.Context, entries []*LogEntry) error

	// Query 查询日志
	Query(ctx context.Context, req *QueryRequest) (*QueryResult, error)

	// Tail 获取最新日志
	Tail(ctx context.Context, req *TailRequest) ([]*LogEntry, error)

	// GetStats 获取统计信息
	GetStats(ctx context.Context) (*LogStats, error)

	// GetStream 获取日志流
	GetStream(ctx context.Context, streamID string) (*LogStream, error)

	// ListStreams 列出所有日志流
	ListStreams(ctx context.Context, labels map[string]string) ([]*LogStream, error)

	// Cleanup 清理过期日志
	Cleanup(ctx context.Context) error

	// Close 关闭存储
	Close() error
}

// storage 存储实现
type storage struct {
	db     *sql.DB
	config *StorageConfig

	// 当前活动块
	activeChunks map[string]*activeChunk // streamID -> activeChunk
	chunkMu      sync.Mutex
}

// activeChunk 当前正在写入的块
type activeChunk struct {
	chunk   *LogChunk
	buffer  *bytes.Buffer
	entries []*LogEntry
	mu      sync.Mutex
}

// NewStorage 创建新的存储实例
func NewStorage(config *StorageConfig) (Storage, error) {
	if config == nil {
		config = &DefaultConfig().Storage
	}

	// 创建数据目录
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// 创建数据库目录
	dbDir := filepath.Dir(config.DBPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	// 打开数据库
	db, err := sql.Open("sqlite3", config.DBPath+"?_journal_mode=WAL&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 设置连接池
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	s := &storage{
		db:           db,
		config:       config,
		activeChunks: make(map[string]*activeChunk),
	}

	// 初始化数据库表
	if err := s.initTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
	}

	logrus.Info("Log storage initialized")
	return s, nil
}

// initTables 初始化数据库表
func (s *storage) initTables() error {
	schema := `
	-- 日志流表
	CREATE TABLE IF NOT EXISTS log_streams (
		stream_id TEXT PRIMARY KEY,
		labels TEXT NOT NULL,
		first_seen INTEGER NOT NULL,
		last_seen INTEGER NOT NULL
	);

	-- 日志块表
	CREATE TABLE IF NOT EXISTS log_chunks (
		chunk_id TEXT PRIMARY KEY,
		stream_id TEXT NOT NULL,
		start_time INTEGER NOT NULL,
		end_time INTEGER NOT NULL,
		file_path TEXT NOT NULL,
		compressed_size INTEGER NOT NULL,
		uncompressed_size INTEGER NOT NULL,
		line_count INTEGER NOT NULL,
		created_at INTEGER NOT NULL,
		FOREIGN KEY (stream_id) REFERENCES log_streams(stream_id) ON DELETE CASCADE
	);

	-- 标签索引表
	CREATE TABLE IF NOT EXISTS log_labels (
		stream_id TEXT NOT NULL,
		label_key TEXT NOT NULL,
		label_value TEXT NOT NULL,
		FOREIGN KEY (stream_id) REFERENCES log_streams(stream_id) ON DELETE CASCADE
	);

	-- 创建索引
	CREATE INDEX IF NOT EXISTS idx_streams_last_seen ON log_streams(last_seen);
	CREATE INDEX IF NOT EXISTS idx_chunks_stream_time ON log_chunks(stream_id, start_time);
	CREATE INDEX IF NOT EXISTS idx_chunks_time_range ON log_chunks(start_time, end_time);
	CREATE INDEX IF NOT EXISTS idx_labels_key_value ON log_labels(label_key, label_value);
	CREATE INDEX IF NOT EXISTS idx_labels_stream ON log_labels(stream_id);
	`

	_, err := s.db.Exec(schema)
	return err
}

// Write 写入日志条目
func (s *storage) Write(ctx context.Context, entries []*LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	s.chunkMu.Lock()
	defer s.chunkMu.Unlock()

	// 按 streamID 分组
	streamGroups := s.groupByStream(entries)

	for streamID, streamEntries := range streamGroups {
		if err := s.writeToStream(ctx, streamID, streamEntries); err != nil {
			logrus.Errorf("Failed to write to stream %s: %v", streamID, err)
			// 继续处理其他流
		}
	}

	return nil
}

// groupByStream 按流分组日志
func (s *storage) groupByStream(entries []*LogEntry) map[string][]*LogEntry {
	groups := make(map[string][]*LogEntry)

	for _, entry := range entries {
		streamID := s.getOrCreateStreamID(entry.Labels)
		groups[streamID] = append(groups[streamID], entry)
	}

	return groups
}

// getOrCreateStreamID 获取或创建流ID
func (s *storage) getOrCreateStreamID(labels map[string]string) string {
	// 生成稳定的 streamID（基于标签的哈希）
	labelsJSON, _ := json.Marshal(labels)

	// 先尝试从数据库查找
	var streamID string
	err := s.db.QueryRow(
		`SELECT stream_id FROM log_streams WHERE labels = ? LIMIT 1`,
		string(labelsJSON),
	).Scan(&streamID)

	if err == nil {
		return streamID
	}

	// 不存在，创建新的
	streamID = shortuuid.New()
	now := time.Now().Unix()

	_, err = s.db.Exec(
		`INSERT INTO log_streams (stream_id, labels, first_seen, last_seen) VALUES (?, ?, ?, ?)`,
		streamID, string(labelsJSON), now, now,
	)
	if err != nil {
		logrus.Errorf("Failed to create stream: %v", err)
		return streamID
	}

	// 插入标签索引
	for key, value := range labels {
		s.db.Exec(
			`INSERT INTO log_labels (stream_id, label_key, label_value) VALUES (?, ?, ?)`,
			streamID, key, value,
		)
	}

	logrus.Debugf("Created new log stream: %s", streamID)
	return streamID
}

// writeToStream 写入日志到流
func (s *storage) writeToStream(ctx context.Context, streamID string, entries []*LogEntry) error {
	// 获取或创建活动块
	chunk := s.getOrCreateActiveChunk(streamID, entries[0].Labels)
	chunk.mu.Lock()
	defer chunk.mu.Unlock()

	// 写入条目到缓冲区
	for _, entry := range entries {
		// 序列化日志条目
		line := s.formatLogLine(entry)
		chunk.buffer.WriteString(line)
		chunk.buffer.WriteByte('\n')
		chunk.entries = append(chunk.entries, entry)
	}

	// 检查是否需要刷新
	shouldFlush := s.shouldFlushChunk(chunk)
	if shouldFlush {
		return s.flushChunk(ctx, streamID, chunk)
	}

	return nil
}

// getOrCreateActiveChunk 获取或创建活动块
func (s *storage) getOrCreateActiveChunk(streamID string, labels map[string]string) *activeChunk {
	if chunk, exists := s.activeChunks[streamID]; exists {
		return chunk
	}

	chunk := &activeChunk{
		chunk: &LogChunk{
			ChunkID:   shortuuid.New(),
			StreamID:  streamID,
			StartTime: time.Now(),
		},
		buffer:  new(bytes.Buffer),
		entries: make([]*LogEntry, 0, 1000),
	}

	s.activeChunks[streamID] = chunk
	return chunk
}

// shouldFlushChunk 判断是否应该刷新块
func (s *storage) shouldFlushChunk(chunk *activeChunk) bool {
	// 检查行数
	if len(chunk.entries) >= s.config.ChunkMaxLines {
		return true
	}

	// 检查大小
	if int64(chunk.buffer.Len()) >= s.config.ChunkMaxSize {
		return true
	}

	// 检查时间
	if time.Since(chunk.chunk.StartTime) >= s.config.ChunkDuration {
		return true
	}

	return false
}

// flushChunk 刷新块到磁盘
func (s *storage) flushChunk(ctx context.Context, streamID string, chunk *activeChunk) error {
	if len(chunk.entries) == 0 {
		return nil
	}

	chunk.chunk.EndTime = time.Now()
	chunk.chunk.LineCount = len(chunk.entries)
	chunk.chunk.UncompressedSize = int64(chunk.buffer.Len())

	// 生成文件路径
	dateStr := chunk.chunk.StartTime.Format("2006-01-02")
	chunkDir := filepath.Join(s.config.DataDir, "chunks", dateStr, streamID)
	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		return fmt.Errorf("failed to create chunk directory: %w", err)
	}

	filePath := filepath.Join(chunkDir, chunk.chunk.ChunkID+".gz")
	chunk.chunk.FilePath = filePath

	// 压缩并写入文件
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create chunk file: %w", err)
	}
	defer file.Close()

	gzipWriter, err := gzip.NewWriterLevel(file, s.config.CompressionLevel)
	if err != nil {
		return fmt.Errorf("failed to create gzip writer: %w", err)
	}
	defer gzipWriter.Close()

	written, err := gzipWriter.Write(chunk.buffer.Bytes())
	if err != nil {
		return fmt.Errorf("failed to write compressed data: %w", err)
	}

	if err := gzipWriter.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	chunk.chunk.CompressedSize = int64(written)

	// 保存块元数据到数据库
	if err := s.saveChunkMetadata(chunk.chunk); err != nil {
		return fmt.Errorf("failed to save chunk metadata: %w", err)
	}

	// 更新流的最后更新时间
	s.db.Exec(
		`UPDATE log_streams SET last_seen = ? WHERE stream_id = ?`,
		time.Now().Unix(), streamID,
	)

	logrus.Debugf("Flushed chunk %s: %d lines, %d bytes (compressed: %d bytes)",
		chunk.chunk.ChunkID, chunk.chunk.LineCount, chunk.chunk.UncompressedSize, chunk.chunk.CompressedSize)

	// 清理当前活动块
	delete(s.activeChunks, streamID)

	return nil
}

// saveChunkMetadata 保存块元数据
func (s *storage) saveChunkMetadata(chunk *LogChunk) error {
	_, err := s.db.Exec(`
		INSERT INTO log_chunks (
			chunk_id, stream_id, start_time, end_time, file_path,
			compressed_size, uncompressed_size, line_count, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		chunk.ChunkID, chunk.StreamID,
		chunk.StartTime.Unix(), chunk.EndTime.Unix(),
		chunk.FilePath, chunk.CompressedSize, chunk.UncompressedSize,
		chunk.LineCount, time.Now().Unix(),
	)
	return err
}

// formatLogLine 格式化日志行
func (s *storage) formatLogLine(entry *LogEntry) string {
	// 简单的 JSON 格式
	data := map[string]interface{}{
		"ts":    entry.Timestamp.Format(time.RFC3339Nano),
		"level": entry.Level,
		"msg":   entry.Message,
		"src":   entry.Source,
	}

	if entry.Raw != "" {
		data["raw"] = entry.Raw
	}

	jsonData, _ := json.Marshal(data)
	return string(jsonData)
}

// Query 查询日志
func (s *storage) Query(ctx context.Context, req *QueryRequest) (*QueryResult, error) {
	startTime := time.Now()
	result := &QueryResult{
		Entries: make([]*LogEntry, 0),
		Stats:   &QueryStats{},
	}

	// 查找匹配的流
	streamIDs, err := s.findMatchingStreams(req.Labels)
	if err != nil {
		return nil, fmt.Errorf("failed to find matching streams: %w", err)
	}

	if len(streamIDs) == 0 {
		return result, nil
	}

	// 查找时间范围内的块
	chunks, err := s.findChunksInTimeRange(streamIDs, req.StartTime, req.EndTime)
	if err != nil {
		return nil, fmt.Errorf("failed to find chunks: %w", err)
	}

	result.Stats.ChunksScanned = len(chunks)

	// 读取和过滤日志
	for _, chunk := range chunks {
		entries, err := s.readChunk(chunk)
		if err != nil {
			logrus.Errorf("Failed to read chunk %s: %v", chunk.ChunkID, err)
			continue
		}

		result.Stats.LinesScanned += len(entries)

		// 应用过滤器
		for _, entry := range entries {
			if s.matchesFilter(entry, req) {
				result.Entries = append(result.Entries, entry)

				// 检查限制
				if req.Limit > 0 && len(result.Entries) >= req.Limit {
					result.Limited = true
					goto DONE
				}
			}
		}
	}

DONE:
	result.Total = len(result.Entries)
	result.QueryTime = time.Since(startTime)

	// 根据方向排序
	if req.Direction == QueryDirectionBackward {
		s.reverseEntries(result.Entries)
	}

	return result, nil
}

// findMatchingStreams 查找匹配的流
func (s *storage) findMatchingStreams(labels map[string]string) ([]string, error) {
	if len(labels) == 0 {
		// 返回所有流
		rows, err := s.db.Query(`SELECT stream_id FROM log_streams`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var streamIDs []string
		for rows.Next() {
			var streamID string
			if err := rows.Scan(&streamID); err != nil {
				continue
			}
			streamIDs = append(streamIDs, streamID)
		}
		return streamIDs, nil
	}

	// 基于标签查询
	// 构建查询：找到包含所有指定标签的流
	query := `
		SELECT DISTINCT stream_id
		FROM log_labels
		WHERE (label_key = ? AND label_value = ?)
	`
	args := make([]interface{}, 0, len(labels)*2)

	first := true
	for key, value := range labels {
		if !first {
			query = `
				SELECT stream_id FROM (` + query + `)
				INTERSECT
				SELECT stream_id FROM log_labels WHERE label_key = ? AND label_value = ?
			`
		}
		args = append(args, key, value)
		first = false
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var streamIDs []string
	for rows.Next() {
		var streamID string
		if err := rows.Scan(&streamID); err != nil {
			continue
		}
		streamIDs = append(streamIDs, streamID)
	}

	return streamIDs, nil
}

// findChunksInTimeRange 查找时间范围内的块
func (s *storage) findChunksInTimeRange(streamIDs []string, startTime, endTime time.Time) ([]*LogChunk, error) {
	if len(streamIDs) == 0 {
		return nil, nil
	}

	// 构建 IN 子句
	placeholders := make([]string, len(streamIDs))
	args := make([]interface{}, 0, len(streamIDs)+2)
	for i := range streamIDs {
		placeholders[i] = "?"
		args = append(args, streamIDs[i])
	}
	args = append(args, startTime.Unix(), endTime.Unix())

	query := fmt.Sprintf(`
		SELECT chunk_id, stream_id, start_time, end_time, file_path,
		       compressed_size, uncompressed_size, line_count
		FROM log_chunks
		WHERE stream_id IN (%s)
		  AND end_time >= ?
		  AND start_time <= ?
		ORDER BY start_time ASC
	`, joinPlaceholders(placeholders))

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []*LogChunk
	for rows.Next() {
		chunk := &LogChunk{}
		var startTime, endTime int64

		err := rows.Scan(
			&chunk.ChunkID, &chunk.StreamID,
			&startTime, &endTime,
			&chunk.FilePath, &chunk.CompressedSize,
			&chunk.UncompressedSize, &chunk.LineCount,
		)
		if err != nil {
			logrus.Errorf("Failed to scan chunk: %v", err)
			continue
		}

		chunk.StartTime = time.Unix(startTime, 0)
		chunk.EndTime = time.Unix(endTime, 0)
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// readChunk 读取块内容
func (s *storage) readChunk(chunk *LogChunk) ([]*LogEntry, error) {
	file, err := os.Open(chunk.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open chunk file: %w", err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	data, err := io.ReadAll(gzipReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read compressed data: %w", err)
	}

	// 按行解析
	lines := bytes.Split(data, []byte("\n"))
	entries := make([]*LogEntry, 0, len(lines))

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		entry, err := s.parseLogLine(line)
		if err != nil {
			logrus.Debugf("Failed to parse log line: %v", err)
			continue
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// parseLogLine 解析日志行
func (s *storage) parseLogLine(line []byte) (*LogEntry, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(line, &data); err != nil {
		return nil, err
	}

	entry := &LogEntry{
		Labels: make(map[string]string),
	}

	if ts, ok := data["ts"].(string); ok {
		entry.Timestamp, _ = time.Parse(time.RFC3339Nano, ts)
	}
	if level, ok := data["level"].(string); ok {
		entry.Level = LogLevel(level)
	}
	if msg, ok := data["msg"].(string); ok {
		entry.Message = msg
	}
	if src, ok := data["src"].(string); ok {
		entry.Source = LogSource(src)
	}
	if raw, ok := data["raw"].(string); ok {
		entry.Raw = raw
	}

	return entry, nil
}

// matchesFilter 检查日志是否匹配过滤器
func (s *storage) matchesFilter(entry *LogEntry, req *QueryRequest) bool {
	// 时间范围过滤
	if !req.StartTime.IsZero() && entry.Timestamp.Before(req.StartTime) {
		return false
	}
	if !req.EndTime.IsZero() && entry.Timestamp.After(req.EndTime) {
		return false
	}

	// 级别过滤
	if req.Level != "" && entry.Level != req.Level {
		return false
	}

	// Grep 过滤
	if req.Grep != "" {
		if !containsString(entry.Message, req.Grep) && !containsString(entry.Raw, req.Grep) {
			return false
		}
	}

	// TODO: Regex 过滤

	return true
}

// Tail 获取最新日志
func (s *storage) Tail(ctx context.Context, req *TailRequest) ([]*LogEntry, error) {
	// 转换为查询请求
	queryReq := &QueryRequest{
		Labels:    req.Labels,
		Level:     req.Level,
		Limit:     req.Lines,
		Direction: QueryDirectionBackward,
		EndTime:   time.Now(),
		StartTime: time.Now().Add(-24 * time.Hour), // 默认查询最近24小时
	}

	result, err := s.Query(ctx, queryReq)
	if err != nil {
		return nil, err
	}

	return result.Entries, nil
}

// GetStats 获取统计信息
func (s *storage) GetStats(ctx context.Context) (*LogStats, error) {
	stats := &LogStats{
		LevelCounts:     make(map[LogLevel]int64),
		ContainerCounts: make(map[string]int64),
	}

	// 统计流和块
	s.db.QueryRow(`SELECT COUNT(*) FROM log_streams`).Scan(&stats.TotalStreams)
	s.db.QueryRow(`SELECT COUNT(*) FROM log_chunks`).Scan(&stats.TotalChunks)
	s.db.QueryRow(`SELECT SUM(line_count) FROM log_chunks`).Scan(&stats.TotalLines)
	s.db.QueryRow(`SELECT SUM(compressed_size) FROM log_chunks`).Scan(&stats.TotalBytes)

	// 获取时间范围
	var oldestUnix, newestUnix sql.NullInt64
	s.db.QueryRow(`SELECT MIN(start_time) FROM log_chunks`).Scan(&oldestUnix)
	s.db.QueryRow(`SELECT MAX(end_time) FROM log_chunks`).Scan(&newestUnix)

	if oldestUnix.Valid {
		stats.OldestLog = time.Unix(oldestUnix.Int64, 0)
	}
	if newestUnix.Valid {
		stats.NewestLog = time.Unix(newestUnix.Int64, 0)
	}

	return stats, nil
}

// GetStream 获取日志流
func (s *storage) GetStream(ctx context.Context, streamID string) (*LogStream, error) {
	var labelsJSON string
	var firstSeen, lastSeen int64

	err := s.db.QueryRow(
		`SELECT labels, first_seen, last_seen FROM log_streams WHERE stream_id = ?`,
		streamID,
	).Scan(&labelsJSON, &firstSeen, &lastSeen)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("stream not found: %s", streamID)
	}
	if err != nil {
		return nil, err
	}

	var labels map[string]string
	json.Unmarshal([]byte(labelsJSON), &labels)

	return &LogStream{
		StreamID:  streamID,
		Labels:    labels,
		FirstSeen: time.Unix(firstSeen, 0),
		LastSeen:  time.Unix(lastSeen, 0),
	}, nil
}

// ListStreams 列出所有日志流
func (s *storage) ListStreams(ctx context.Context, labels map[string]string) ([]*LogStream, error) {
	var streamIDs []string
	var err error

	if len(labels) == 0 {
		// 列出所有流
		rows, err := s.db.Query(`SELECT stream_id FROM log_streams ORDER BY last_seen DESC`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var streamID string
			if err := rows.Scan(&streamID); err != nil {
				continue
			}
			streamIDs = append(streamIDs, streamID)
		}
	} else {
		// 根据标签过滤
		streamIDs, err = s.findMatchingStreams(labels)
		if err != nil {
			return nil, err
		}
	}

	// 获取流详细信息
	streams := make([]*LogStream, 0, len(streamIDs))
	for _, streamID := range streamIDs {
		stream, err := s.GetStream(ctx, streamID)
		if err != nil {
			logrus.Errorf("Failed to get stream %s: %v", streamID, err)
			continue
		}
		streams = append(streams, stream)
	}

	return streams, nil
}

// Cleanup 清理过期日志
func (s *storage) Cleanup(ctx context.Context) error {
	if s.config.RetentionPeriod == 0 {
		return nil // 不清理
	}

	cutoffTime := time.Now().Add(-s.config.RetentionPeriod).Unix()

	// 查询需要删除的块
	rows, err := s.db.Query(
		`SELECT chunk_id, file_path FROM log_chunks WHERE end_time < ?`,
		cutoffTime,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	deletedCount := 0
	for rows.Next() {
		var chunkID, filePath string
		if err := rows.Scan(&chunkID, &filePath); err != nil {
			continue
		}

		// 删除文件
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			logrus.Errorf("Failed to delete chunk file %s: %v", filePath, err)
		}

		// 删除数据库记录
		s.db.Exec(`DELETE FROM log_chunks WHERE chunk_id = ?`, chunkID)
		deletedCount++
	}

	if deletedCount > 0 {
		logrus.Infof("Cleaned up %d expired log chunks", deletedCount)
	}

	// 清理空的流
	s.db.Exec(`DELETE FROM log_streams WHERE stream_id NOT IN (SELECT DISTINCT stream_id FROM log_chunks)`)

	return nil
}

// Close 关闭存储
func (s *storage) Close() error {
	s.chunkMu.Lock()
	defer s.chunkMu.Unlock()

	// 刷新所有活动块
	for streamID, chunk := range s.activeChunks {
		if err := s.flushChunk(context.Background(), streamID, chunk); err != nil {
			logrus.Errorf("Failed to flush chunk on close: %v", err)
		}
	}

	return s.db.Close()
}

// reverseEntries 反转日志条目顺序
func (s *storage) reverseEntries(entries []*LogEntry) {
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
}

// 辅助函数
func joinPlaceholders(placeholders []string) string {
	result := ""
	for i, p := range placeholders {
		if i > 0 {
			result += ", "
		}
		result += p
	}
	return result
}

func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && bytes.Contains([]byte(s), []byte(substr))
}
