package main

import (
	"context"
	"flag"
	pbtunnel "github.com/yhzs8/grpcl4tunnel/api/tunnel"
	"github.com/yhzs8/grpcl4tunnel/internal/protocols"
	"github.com/yhzs8/grpcl4tunnel/tunnel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"log"
)

var (
	clientId           = flag.String("client_id", "client_1", "the client identifier")
	tls                = flag.Bool("tls", false, "Connection uses TLS if true, else plain TCP")
	caFile             = flag.String("ca_file", "", "The file containing the CA root cert file")
	serverAddr         = flag.String("addr", "192.168.0.1:50051", "The server address in the format of host:port")
	serverHostOverride = flag.String("server_host_override", "x.test.example.com", "The server name used to verify the hostname returned by the TLS handshake")
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

func main() {
	flag.Parse()
	var opts []grpc.DialOption
	if *tls {
		if *caFile == "" {
			log.Fatalf("CA File is not defined")
		}
		creds, err := credentials.NewClientTLSFromFile(*caFile, *serverHostOverride)
		if err != nil {
			log.Fatalf("Failed to create TLS credentials: %v", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.Dial(*serverAddr, opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()

	client := pbtunnel.NewTunnelServiceClient(conn)

	tunnelGetResponseList, err := client.GetClientInitiatedTunnels(context.Background(), &pbtunnel.TunnelGetPayload{ClientId: *clientId})
	if err != nil {
		log.Fatalf("fail to fetch tunnels from gRPC server: %v", err)
	}

	runTunnelChat(client, tunnelGetResponseList.Tunnel)
}
