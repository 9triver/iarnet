package main

import (
	"context"
	"encoding/json"
	"sync"

	commonpb "github.com/9triver/iarnet/runner/proto/common"
	loggerpb "github.com/9triver/iarnet/runner/proto/logger"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type remoteLogHook struct {
	appID  string
	conn   *grpc.ClientConn
	client loggerpb.LoggerServiceClient
	stream loggerpb.LoggerService_StreamLogsClient

	mu sync.Mutex
}

func newRemoteLogHook(ctx context.Context, address, appID string) (*remoteLogHook, error) {
	conn, err := grpc.DialContext(ctx, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	client := loggerpb.NewLoggerServiceClient(conn)
	stream, err := client.StreamLogs(ctx)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return &remoteLogHook{
		appID:  appID,
		conn:   conn,
		client: client,
		stream: stream,
	}, nil
}

func (h *remoteLogHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *remoteLogHook) Fire(entry *logrus.Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.stream == nil {
		var err error
		h.stream, err = h.client.StreamLogs(context.Background())
		if err != nil {
			return err
		}
	}

	logEntry := &commonpb.LogEntry{
		Timestamp: entry.Time.UnixNano(),
		Level:     logrusLevelToProto(entry.Level),
		Message:   entry.Message,
	}

	if len(entry.Data) > 0 {
		logEntry.Fields = make([]*commonpb.LogField, 0, len(entry.Data))
		for k, v := range entry.Data {
			valueBytes, err := json.Marshal(v)
			if err != nil {
				valueBytes = []byte(`"unknown"`)
			}
			logEntry.Fields = append(logEntry.Fields, &commonpb.LogField{
				Key:   k,
				Value: string(valueBytes),
			})
		}
	}

	if entry.HasCaller() && entry.Caller != nil {
		logEntry.Caller = &commonpb.CallerInfo{
			File:     entry.Caller.File,
			Line:     int32(entry.Caller.Line),
			Function: entry.Caller.Function,
		}
	}

	msg := &loggerpb.LogStreamMessage{
		ApplicationId: h.appID,
		Entry:         logEntry,
	}

	if err := h.stream.Send(msg); err != nil {
		h.stream.CloseSend()
		h.stream = nil
		return err
	}

	return nil
}

func (h *remoteLogHook) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.stream != nil {
		h.stream.CloseSend()
		h.stream = nil
	}
	if h.conn != nil {
		return h.conn.Close()
	}
	return nil
}

func logrusLevelToProto(level logrus.Level) commonpb.LogLevel {
	switch level {
	case logrus.TraceLevel:
		return commonpb.LogLevel_LOG_LEVEL_TRACE
	case logrus.DebugLevel:
		return commonpb.LogLevel_LOG_LEVEL_DEBUG
	case logrus.InfoLevel:
		return commonpb.LogLevel_LOG_LEVEL_INFO
	case logrus.WarnLevel:
		return commonpb.LogLevel_LOG_LEVEL_WARN
	case logrus.ErrorLevel:
		return commonpb.LogLevel_LOG_LEVEL_ERROR
	case logrus.FatalLevel:
		return commonpb.LogLevel_LOG_LEVEL_FATAL
	case logrus.PanicLevel:
		return commonpb.LogLevel_LOG_LEVEL_PANIC
	default:
		return commonpb.LogLevel_LOG_LEVEL_UNKNOWN
	}
}
