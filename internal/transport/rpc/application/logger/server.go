package logger

import (
	"errors"
	"io"

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

func (s *Server) StreamLogs(stream grpc.BidiStreamingServer[loggerpb.LogStreamMessage, loggerpb.LogStreamResponse]) error {
	ctx := stream.Context()
	var processed int

	for {
		req, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				finalResp := &loggerpb.LogStreamResponse{
					Success:        true,
					ProcessedCount: int32(processed),
					Message:        "stream closed",
				}
				if sendErr := stream.Send(finalResp); sendErr != nil {
					return sendErr
				}
				return nil
			}
			return err
		}

		if req == nil {
			continue
		}

		appID := req.GetApplicationId()
		if appID == "" {
			if err := stream.Send(&loggerpb.LogStreamResponse{
				Success:        false,
				Error:          "application_id is required",
				ProcessedCount: int32(processed),
			}); err != nil {
				return err
			}
			continue
		}

		entry := protoLogEntryToDomain(req.GetEntry())
		if entry == nil {
			if err := stream.Send(&loggerpb.LogStreamResponse{
				ApplicationId:  appID,
				Success:        false,
				Error:          "log entry is required",
				ProcessedCount: int32(processed),
			}); err != nil {
				return err
			}
			continue
		}

		_, svcErr := s.svc.SubmitLog(ctx, appID, entry)
		if svcErr != nil {
			if err := stream.Send(&loggerpb.LogStreamResponse{
				ApplicationId:  appID,
				Success:        false,
				Error:          svcErr.Error(),
				ProcessedCount: int32(processed),
			}); err != nil {
				return err
			}
			continue
		}

		// 保存成功暂时不返回响应
	}
}
