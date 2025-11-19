package logger

import (
	"errors"
	"io"
	"time"

	domainlogger "github.com/9triver/iarnet/internal/domain/resource/logger"
	commonpb "github.com/9triver/iarnet/internal/proto/common"
	loggerpb "github.com/9triver/iarnet/internal/proto/resource/logger"
	"google.golang.org/grpc"
)

type Server struct {
	loggerpb.UnimplementedLoggerServiceServer
	svc domainlogger.Service
}

func NewServer(svc domainlogger.Service) *Server {
	return &Server{svc: svc}
}

func (s *Server) StreamLogs(stream grpc.BidiStreamingServer[loggerpb.LogStreamMessage, loggerpb.LogStreamResponse]) error {
	ctx := stream.Context()
	var processed int

	for {
		req, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return stream.Send(&loggerpb.LogStreamResponse{
					Success:        true,
					ProcessedCount: int32(processed),
					Message:        "stream closed",
				})
			}
			return err
		}

		if req == nil {
			continue
		}

		componentID := req.GetComponentId()
		if componentID == "" {
			if err := stream.Send(&loggerpb.LogStreamResponse{
				Success:        false,
				Error:          "component_id is required",
				ProcessedCount: int32(processed),
			}); err != nil {
				return err
			}
			continue
		}

		if control := req.GetControl(); control != nil {
			if err := stream.Send(&loggerpb.LogStreamResponse{
				ComponentId:    componentID,
				Success:        true,
				ProcessedCount: int32(processed),
				Message:        control.GetType().String(),
			}); err != nil {
				return err
			}
			continue
		}

		entry := protoLogEntryToDomain(req.GetEntry())
		if entry == nil {
			if err := stream.Send(&loggerpb.LogStreamResponse{
				ComponentId:    componentID,
				Success:        false,
				Error:          "log entry is required",
				ProcessedCount: int32(processed),
			}); err != nil {
				return err
			}
			continue
		}

		if _, svcErr := s.svc.SubmitLog(ctx, componentID, entry); svcErr != nil {
			if err := stream.Send(&loggerpb.LogStreamResponse{
				ComponentId:    componentID,
				Success:        false,
				Error:          svcErr.Error(),
				ProcessedCount: int32(processed),
			}); err != nil {
				return err
			}
			continue
		}

		processed++
	}
}

func protoLogEntryToDomain(entry *commonpb.LogEntry) *domainlogger.Entry {
	if entry == nil {
		return nil
	}

	domainEntry := &domainlogger.Entry{
		Timestamp: time.Unix(0, entry.Timestamp),
		Level:     protoLogLevelToDomain(entry.Level),
		Message:   entry.Message,
	}

	if len(entry.Fields) > 0 {
		fields := make([]domainlogger.LogField, len(entry.Fields))
		for i, f := range entry.Fields {
			fields[i] = domainlogger.LogField{
				Key:   f.Key,
				Value: f.Value,
			}
		}
		domainEntry.Fields = fields
	}

	if entry.Caller != nil {
		domainEntry.Caller = &domainlogger.CallerInfo{
			File:     entry.Caller.File,
			Line:     int(entry.Caller.Line),
			Function: entry.Caller.Function,
		}
	}

	return domainEntry
}

func protoLogLevelToDomain(level commonpb.LogLevel) domainlogger.LogLevel {
	switch level {
	case commonpb.LogLevel_LOG_LEVEL_TRACE:
		return domainlogger.LogLevelTrace
	case commonpb.LogLevel_LOG_LEVEL_DEBUG:
		return domainlogger.LogLevelDebug
	case commonpb.LogLevel_LOG_LEVEL_INFO:
		return domainlogger.LogLevelInfo
	case commonpb.LogLevel_LOG_LEVEL_WARN:
		return domainlogger.LogLevelWarn
	case commonpb.LogLevel_LOG_LEVEL_ERROR:
		return domainlogger.LogLevelError
	case commonpb.LogLevel_LOG_LEVEL_FATAL:
		return domainlogger.LogLevelFatal
	case commonpb.LogLevel_LOG_LEVEL_PANIC:
		return domainlogger.LogLevelPanic
	default:
		return domainlogger.LogLevelUnknown
	}
}
