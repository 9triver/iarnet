package logger

import (
	"context"

	domainlogger "github.com/9triver/iarnet/internal/domain/application/logger"
	loggerpb "github.com/9triver/iarnet/internal/proto/application/logger"
	"google.golang.org/grpc"
)

type Server struct {
	loggerpb.UnimplementedLoggerServiceServer
	svc domainlogger.Service
}

func NewServer(svc domainlogger.Service) *Server {
	return &Server{svc: svc}
}

func (s *Server) SubmitLog(ctx context.Context, req *loggerpb.SubmitLogRequest) (*loggerpb.SubmitLogResponse, error) {
	// 转换 proto 类型到 domain 类型
	entry := protoLogEntryToDomain(req.Entry)

	// 调用 domain 服务
	result, err := s.svc.SubmitLog(ctx, req.ApplicationId, entry)
	if err != nil {
		return &loggerpb.SubmitLogResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// 转换 domain 结果到 proto 响应
	return domainSubmitLogResultToProto(result), nil
}

func (s *Server) SubmitLogBatch(ctx context.Context, req *loggerpb.BatchSubmitLogRequest) (*loggerpb.BatchSubmitLogResponse, error) {
	// 转换 proto 类型到 domain 类型
	entries := make([]*domainlogger.Entry, len(req.Entries))
	for i, entry := range req.Entries {
		entries[i] = protoLogEntryToDomain(entry)
	}

	// 调用 domain 服务
	result, err := s.svc.BatchSubmitLogs(ctx, req.ApplicationId, entries)
	if err != nil {
		return &loggerpb.BatchSubmitLogResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// 转换 domain 结果到 proto 响应
	return domainBatchSubmitLogResultToProto(result), nil
}

func (s *Server) StreamLogs(stream grpc.BidiStreamingServer[loggerpb.LogStreamMessage, loggerpb.LogStreamResponse]) error {
	// TODO: 实现流式日志处理
	// 需要将 proto 流转换为 domain 类型，然后调用 domain 服务
	ctx := stream.Context()
	_ = ctx // 使用 context 进行后续实现
	return nil
}
