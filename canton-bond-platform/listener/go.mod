module canton-bond-platform/listener

go 1.26

require (
	canton-bond-platform/pkg/cantonledger v0.0.0
	github.com/gorilla/websocket v1.5.3
)

require (
	github.com/golang/protobuf v1.5.4 // indirect
	golang.org/x/net v0.51.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260226221140-a57be14db171 // indirect
	google.golang.org/grpc v1.81.1 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace canton-bond-platform/pkg/cantonledger => ../pkg/cantonledger
