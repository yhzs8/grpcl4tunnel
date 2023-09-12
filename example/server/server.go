package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	pbtunnel "github.com/yhzs8/grpcl4tunnel/api/tunnel"
	"github.com/yhzs8/grpcl4tunnel/internal/protocols"
	"github.com/yhzs8/grpcl4tunnel/tunnel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/examples/data"
	"google.golang.org/grpc/metadata"
	"log"
	"net"
	"net/http"
	_ "net/http"
	_ "net/http/pprof"
	"sync"
)

var (
	tls        = flag.Bool("tls", false, "Connection uses TLS if true, else plain TCP")
	certFile   = flag.String("cert_file", "", "The TLS cert file")
	keyFile    = flag.String("key_file", "", "The TLS key file")
	jsonDBFile = flag.String("json_db_file", "", "A json file containing a list of features")
	port       = flag.Int("port", 50051, "The server port")
)

type tunnelServer struct {
	pbtunnel.UnimplementedTunnelServiceServer

	mu sync.Mutex // protects routeNotes
}

func parseClientId(ctx context.Context) (string, error) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if _, exists := md["client_id"]; !exists {
			return "", errors.New("context metadata did not include client_id")
		}
		if len(md["client_id"]) > 1 {
			return "", errors.New("context metadata included multiple client_id values")
		}
		return md["client_id"][0], nil
	}
	return "", errors.New("can not extract metadata from context")
}

func getClientInitiatedTunnels(clientId string) []*pbtunnel.Tunnel {
	tcpTunnel := pbtunnel.Tunnel{
		IsServerInitiated: false,
		Protocol:          pbtunnel.Protocol_tcp,
		RemoteHost:        "localhost",
		RemotePort:        4567,
		LocalHost:         "0.0.0.0",
		LocalPort:         4567,
	}
	udpTunnel := pbtunnel.Tunnel{
		IsServerInitiated: false,
		Protocol:          pbtunnel.Protocol_udp,
		RemoteHost:        "localhost",
		RemotePort:        4568,
		LocalHost:         "0.0.0.0",
		LocalPort:         4568,
	}
	sctpTunnel := pbtunnel.Tunnel{
		IsServerInitiated: false,
		Protocol:          pbtunnel.Protocol_sctp,
		RemoteHost:        "localhost",
		RemotePort:        4569,
		LocalHost:         "0.0.0.0",
		LocalPort:         4569,
	}
	var tunnels []*pbtunnel.Tunnel
	tunnels = append(tunnels, &tcpTunnel)
	tunnels = append(tunnels, &udpTunnel)
	tunnels = append(tunnels, &sctpTunnel)

	return tunnels
}

func getServerInitiatedTunnels(clientId string) []*pbtunnel.Tunnel {
	tcpTunnel := pbtunnel.Tunnel{
		IsServerInitiated: true,
		Protocol:          pbtunnel.Protocol_tcp,
		RemoteHost:        "localhost",
		RemotePort:        1234,
		LocalHost:         "0.0.0.0",
		LocalPort:         1234,
	}
	udpTunnel := pbtunnel.Tunnel{
		IsServerInitiated: true,
		Protocol:          pbtunnel.Protocol_udp,
		RemoteHost:        "localhost",
		RemotePort:        1235,
		LocalHost:         "0.0.0.0",
		LocalPort:         1235,
	}
	sctpTunnel := pbtunnel.Tunnel{
		IsServerInitiated: true,
		Protocol:          pbtunnel.Protocol_sctp,
		RemoteHost:        "localhost",
		RemotePort:        1236,
		LocalHost:         "0.0.0.0",
		LocalPort:         1236,
	}
	var tunnels []*pbtunnel.Tunnel
	tunnels = append(tunnels, &tcpTunnel)
	tunnels = append(tunnels, &udpTunnel)
	tunnels = append(tunnels, &sctpTunnel)

	return tunnels
}

func (s *tunnelServer) GetClientInitiatedTunnels(context context.Context, payload *pbtunnel.TunnelGetPayload) (*pbtunnel.TunnelList, error) {
	tunnels := getClientInitiatedTunnels(payload.ClientId)
	return &pbtunnel.TunnelList{Tunnel: tunnels}, nil
}
func (s *tunnelServer) GetServerInitiatedTunnels(context context.Context, payload *pbtunnel.TunnelGetPayload) (*pbtunnel.TunnelList, error) {
	tunnels := getServerInitiatedTunnels(payload.ClientId)
	return &pbtunnel.TunnelList{Tunnel: tunnels}, nil
}

func (s *tunnelServer) TunnelChat(stream pbtunnel.TunnelService_TunnelChatServer) error {
	clientId, err := parseClientId(stream.Context())
	if err != nil {
		return err
	}
	tunnels := getServerInitiatedTunnels(clientId)
	return tunnel.HandleTunnel(clientId, tunnels, protocols.ProductionProtocolImpl{}, true, stream, nil)
}

func main() {
	flag.Parse()
	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	if *tls {
		if *certFile == "" {
			*certFile = data.Path("x509/server_cert.pem")
		}
		if *keyFile == "" {
			*keyFile = data.Path("x509/server_key.pem")
		}
		creds, err := credentials.NewServerTLSFromFile(*certFile, *keyFile)
		if err != nil {
			log.Fatalf("Failed to generate credentials: %v", err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}
	go func() {
		http.ListenAndServe(fmt.Sprintf("%s:%d", "0.0.0.0", 6060), nil)
	}()
	grpcServer := grpc.NewServer(opts...)
	pbtunnel.RegisterTunnelServiceServer(grpcServer, &tunnelServer{})
	grpcServer.Serve(lis)
}
