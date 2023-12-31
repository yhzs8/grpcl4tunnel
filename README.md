[![Go Report Card](https://goreportcard.com/badge/github.com/yhzs8/grpcl4tunnel)](https://goreportcard.com/report/github.com/yhzs8/grpcl4tunnel)

# A Go network tunnel implementation using gRPC encapsulation

## Protocols supported:
* TCP
* UDP
* SCTP

## Directions supported
### Server-initiated traffic:
* TCP/SCTP listeners will be set up on the gRPC server side (the listening ports will be determined by the tunnel objects, more about it later)
* TCP/UDP/SCTP clients send traffic to the gRPC server
* gRPC Server tunnels the TCP/UDP/SCTP traffic to the gRPC client
* gRPC client forwards the TCP/UDP/SCTP traffic to the TCP/UDP/SCTP servers on the gRPC client side

### Client-initiated traffic
* TCP/SCTP listeners will be set up on the gRPC client side (the listening ports will be determined by the tunnel objects, more about it later)
* TCP/UDP/SCTP clients send traffic to the gRPC client
* gRPC client tunnels the TCP/UDP/SCTP traffic to the gRPC server
* gRPC server forwards the TCP/UDP/SCTP traffic to the TCP/UDP/SCTP servers on the gRPC server side

## The `tunnel` object
The `tunnel` object defines properties of a tunnel instance, it contains the following fields:
```go
	IsServerInitiated bool     `protobuf:"varint,1,opt,name=isServerInitiated,proto3" json:"isServerInitiated,omitempty"`
	Protocol          Protocol `protobuf:"varint,2,opt,name=protocol,proto3,enum=tunnel.Protocol" json:"protocol,omitempty"`
	RemoteHost        string   `protobuf:"bytes,3,opt,name=RemoteHost,proto3" json:"RemoteHost,omitempty"`
	RemotePort        int32    `protobuf:"varint,4,opt,name=RemotePort,proto3" json:"RemotePort,omitempty"`
	LocalHost         string   `protobuf:"bytes,5,opt,name=LocalHost,proto3" json:"LocalHost,omitempty"`
	LocalPort         int32    `protobuf:"varint,6,opt,name=LocalPort,proto3" json:"LocalPort,omitempty"`
```
* `IsServerInitiated` is the server-initiated or client-initiated tunnel traffic
* `Protocol` can be `tcp`, `udp` or `sctp`
* `RemoteHost` 
  * if `IsServerInitiated` is true, it will be the destination server IP address that the gRPC client communicates with
  * if `IsServerInitiated` is false, it will be the destination server IP address that the gRPC server communicates with
* `RemotePort`
  * if `IsServerInitiated` is true, it will be the destination server port that the gRPC client communicates with
  * if `IsServerInitiated` is false, it will be the destination server port that the gRPC server communicates with
* `LocalHost`
    * if `IsServerInitiated` is true, it will be the listening server IP address that the gRPC server listening on
    * if `IsServerInitiated` is false, it will be the listening server IP address that the gRPC client listening on
* `LocalPort`
    * if `IsServerInitiated` is true, it will be the listening server port that the gRPC server listening on
    * if `IsServerInitiated` is false, it will be the listening server port that the gRPC client listening on

Sample tunnel objects (both server-initiated and client-initiated) are defined in `server.go` example and the gRPC client do retrieve the client-initiated tunnel objects through a gRPC unary call before starting the tunnel endpoints:
```go
tunnelGetResponseList, err := client.GetClientInitiatedTunnels(context.Background(), &pbtunnel.TunnelGetPayload{ClientId: *clientId})
```

## How to run the example:
### gRPC server:
```shell
cd examples
go run server/server.go
```
(the server is listening on port `50051` by default)

### gRPC client:
```shell
cd examples
go run client/client.go --addr=<server IP>:50051
```
## Embed the tunnel implementation in your own gRPC server/client:
### server side:
Invoke `tunnel.HandleTunnel` with `clientId`, `tunnelList` (must be server initiated tunnels), `protocols.ProductionProtocolImpl{}`, `serverStream` and `clientStream=nil`
```go
import (
...
pbtunnel "github.com/yhzs8/grpcl4tunnel/api/tunnel"
"github.com/yhzs8/grpcl4tunnel/internal/protocols"
"github.com/yhzs8/grpcl4tunnel/tunnel"
...
)

func (s *tunnelServer) TunnelChat(stream pbtunnel.TunnelService_TunnelChatServer) error {
    clientId, err := parseClientId(stream.Context())
    if err != nil {
        return err
    }
    tunnels := getServerInitiatedTunnels(clientId)
    return tunnel.HandleTunnel(clientId, tunnels, protocols.ProductionProtocolImpl{}, stream, nil)
}
```
### client side:
Invoke `tunnel.HandleTunnel` with `clientId`, `tunnelList` (retreived from `GetClientInitiatedTunnels()`), `protocols.ProductionProtocolImpl{}`, `serverStream=nil` and `clientStream`
```go
import (
...
pbtunnel "github.com/yhzs8/grpcl4tunnel/api/tunnel"
"github.com/yhzs8/grpcl4tunnel/internal/protocols"
"github.com/yhzs8/grpcl4tunnel/tunnel"
...
)

func runTunnelChat(client pbtunnel.TunnelServiceClient, tunnelList []*pbtunnel.Tunnel) error {
	stream, err := client.TunnelChat(metadata.NewOutgoingContext(
		context.WithoutCancel(context.Background()),
		metadata.MD{"client_id": []string{*clientId}},
	))
	if err != nil {
		return err
	}
	return tunnel.HandleTunnel(*clientId, tunnelList, protocols.ProductionProtocolImpl{}, nil, stream)
}
```
