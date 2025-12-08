module github.com/9triver/iarnet/providers/process

go 1.25.0

replace github.com/9triver/iarnet => ../..

require (
	github.com/9triver/iarnet v0.0.0-00010101000000-000000000000
	github.com/sirupsen/logrus v1.9.3
	google.golang.org/grpc v1.75.0
	google.golang.org/protobuf v1.36.8
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/lithammer/shortuuid/v4 v4.2.0 // indirect
	golang.org/x/net v0.41.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.26.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250707201910-8d1bb00bc6a7 // indirect
)
