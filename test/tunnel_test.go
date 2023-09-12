package test

import (
	"errors"
	"github.com/golang/mock/gomock"
	tunnelpb "github.com/yhzs8/grpcl4tunnel/api/tunnel"
	"github.com/yhzs8/grpcl4tunnel/internal/protocols"
	"github.com/yhzs8/grpcl4tunnel/tunnel"
	"google.golang.org/grpc"
	"testing"
)

var (
	remoteHost = "localhost"
	remotePort = int32(1234)
	localHost  = "0.0.0.0"
	localPort  = int32(1234)
)
var serverInitiatedTunnel = tunnelpb.Tunnel{
	IsServerInitiated: true,
	Protocol:          tunnelpb.Protocol_tcp,
	RemoteHost:        remoteHost,
	RemotePort:        remotePort,
	LocalHost:         localHost,
	LocalPort:         localPort,
}

var clientInitiatedTunnel = tunnelpb.Tunnel{
	IsServerInitiated: false,
	Protocol:          tunnelpb.Protocol_tcp,
	RemoteHost:        remoteHost,
	RemotePort:        remotePort,
	LocalHost:         localHost,
	LocalPort:         localPort,
}

type TunnelChatServerImpl struct {
	grpc.ServerStream
}

func (TunnelChatServerImpl) Send(tunnelMessage *tunnelpb.TunnelMessage) error {
	return nil
}

func (TunnelChatServerImpl) Recv() (*tunnelpb.TunnelMessage, error) {
	return &tunnelpb.TunnelMessage{Content: []byte("response received"), Closed: false, Tunnel: &serverInitiatedTunnel}, nil
}

type TunnelChatClientImpl struct {
	grpc.ClientStream
}

func (TunnelChatClientImpl) Send(tunnelMessage *tunnelpb.TunnelMessage) error {
	return nil
}

func (TunnelChatClientImpl) Recv() (*tunnelpb.TunnelMessage, error) {
	return &tunnelpb.TunnelMessage{Content: []byte("response received"), Closed: false, Tunnel: &serverInitiatedTunnel}, nil
}

var (
	clientId            = "client_123"
	getProtocolImplMock *MockGetProtocolImplInterface
	protocolsImplMock   *MockProtocolInterface
	tunnelServerMock    *MockTunnelService_TunnelChatServer
	tunnelClientMock    *MockTunnelService_TunnelChatClient
	protocolListener    protocols.ProtocolListener
	protocolConn        protocols.ProtocolConn
	tunnelContent       []byte
	tunnelMessage       tunnelpb.TunnelMessage
	tunnelClosedMessage tunnelpb.TunnelMessage
)

func setupMocks(ctrl *gomock.Controller, serverInitiated bool) {
	getProtocolImplMock = NewMockGetProtocolImplInterface(ctrl)
	protocolsImplMock = NewMockProtocolInterface(ctrl)
	tunnelServerMock = NewMockTunnelService_TunnelChatServer(ctrl)
	tunnelClientMock = NewMockTunnelService_TunnelChatClient(ctrl)

	protocolListener = protocols.ProtocolListener{}
	protocolConn = protocols.ProtocolConn{}

	tunnelContent = []byte("data received")

	if serverInitiated {
		tunnelMessage = tunnelpb.TunnelMessage{
			Content: tunnelContent,
			Tunnel:  &serverInitiatedTunnel,
			Closed:  false,
		}
		tunnelClosedMessage = tunnelpb.TunnelMessage{
			Tunnel: &serverInitiatedTunnel,
			Closed: true,
		}
	} else {
		tunnelMessage = tunnelpb.TunnelMessage{
			Content: tunnelContent,
			Tunnel:  &clientInitiatedTunnel,
			Closed:  false,
		}
		tunnelClosedMessage = tunnelpb.TunnelMessage{
			Tunnel: &clientInitiatedTunnel,
			Closed: true,
		}
	}
}

func TestServerInitiatedTunnelAsServer(t *testing.T) {
	ctrl := gomock.NewController(t)
	setupMocks(ctrl, true)

	getProtocolImplMock.EXPECT().GetProtocolImpl(tunnelpb.Protocol_tcp).AnyTimes().Return(protocolsImplMock)

	protocolsImplMock.EXPECT().SetupIncomingSocket(localHost, localPort).Return(&protocolListener, nil)
	protocolsImplMock.EXPECT().ListenIncomingBytes(&protocolListener, localHost, localPort).Return(&protocolConn, nil)
	protocolsImplMock.EXPECT().ReceiveBytesFromSocket(&protocolConn).AnyTimes().Return(tunnelContent, nil)

	tunnelServerMock.EXPECT().Send(gomock.Any()).AnyTimes().Return(nil)
	tunnelServerMock.EXPECT().Recv().Times(100).Return(&tunnelMessage, nil)

	protocolsImplMock.EXPECT().SendBytesToSocket(&protocolConn, tunnelContent).AnyTimes().Return(len(tunnelContent), nil)

	tunnelServerMock.EXPECT().Recv().Return(nil, errors.New("stream failed"))

	protocolsImplMock.EXPECT().ShutdownSocket(&protocolConn).AnyTimes().Return(nil)
	protocolsImplMock.EXPECT().ShutdownListener(&protocolListener).AnyTimes().Return(nil)

	tunnel.HandleTunnel(clientId, []*tunnelpb.Tunnel{&serverInitiatedTunnel}, getProtocolImplMock, true, tunnelServerMock, nil)
}

func TestClientInitiatedTunnelAsServer(t *testing.T) {
	ctrl := gomock.NewController(t)
	setupMocks(ctrl, false)

	getProtocolImplMock.EXPECT().GetProtocolImpl(tunnelpb.Protocol_tcp).AnyTimes().Return(protocolsImplMock)

	tunnelServerMock.EXPECT().Recv().Times(100).Return(&tunnelMessage, nil)

	protocolsImplMock.EXPECT().SetupOutgoingSocket(remoteHost, remotePort).Return(&protocolConn, nil)
	protocolsImplMock.EXPECT().SendBytesToSocket(&protocolConn, tunnelContent).AnyTimes().Return(len(tunnelContent), nil)
	protocolsImplMock.EXPECT().ReceiveBytesFromSocket(&protocolConn).AnyTimes().Return(tunnelContent, nil)

	tunnelServerMock.EXPECT().Send(gomock.Any()).AnyTimes().Return(nil)
	tunnelServerMock.EXPECT().Recv().Return(nil, errors.New("stream failed"))

	protocolsImplMock.EXPECT().ShutdownSocket(&protocolConn).AnyTimes().Return(nil)
	protocolsImplMock.EXPECT().ShutdownListener(&protocolListener).AnyTimes().Return(nil)

	tunnel.HandleTunnel(clientId, []*tunnelpb.Tunnel{}, getProtocolImplMock, true, tunnelServerMock, nil)
}

func TestServerInitiatedTunnelAsClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	setupMocks(ctrl, true)

	getProtocolImplMock.EXPECT().GetProtocolImpl(tunnelpb.Protocol_tcp).AnyTimes().Return(protocolsImplMock)

	tunnelClientMock.EXPECT().Recv().Times(100).Return(&tunnelMessage, nil)

	protocolsImplMock.EXPECT().SetupOutgoingSocket(remoteHost, remotePort).Return(&protocolConn, nil)
	protocolsImplMock.EXPECT().SendBytesToSocket(&protocolConn, tunnelContent).AnyTimes().Return(len(tunnelContent), nil)
	protocolsImplMock.EXPECT().ReceiveBytesFromSocket(&protocolConn).AnyTimes().Return(tunnelContent, nil)

	tunnelClientMock.EXPECT().Send(gomock.Any()).AnyTimes().Return(nil)
	tunnelClientMock.EXPECT().Recv().Return(nil, errors.New("stream failed"))

	protocolsImplMock.EXPECT().ShutdownSocket(&protocolConn).AnyTimes().Return(nil)
	protocolsImplMock.EXPECT().ShutdownListener(&protocolListener).AnyTimes().Return(nil)

	tunnel.HandleTunnel(clientId, []*tunnelpb.Tunnel{}, getProtocolImplMock, false, nil, tunnelClientMock)
}

func TestClientInitiatedTunnelAsClient(t *testing.T) {

	ctrl := gomock.NewController(t)
	setupMocks(ctrl, false)

	getProtocolImplMock.EXPECT().GetProtocolImpl(tunnelpb.Protocol_tcp).AnyTimes().Return(protocolsImplMock)

	protocolsImplMock.EXPECT().SetupIncomingSocket(localHost, localPort).Return(&protocolListener, nil)
	protocolsImplMock.EXPECT().ListenIncomingBytes(&protocolListener, localHost, localPort).Return(&protocolConn, nil)
	protocolsImplMock.EXPECT().ReceiveBytesFromSocket(&protocolConn).AnyTimes().Return(tunnelContent, nil)

	tunnelClientMock.EXPECT().Send(gomock.Any()).AnyTimes().Return(nil)
	tunnelClientMock.EXPECT().Recv().Times(100).Return(&tunnelMessage, nil)

	protocolsImplMock.EXPECT().SendBytesToSocket(gomock.Any(), tunnelContent).AnyTimes().Return(len(tunnelContent), nil)

	tunnelClientMock.EXPECT().Recv().Return(nil, errors.New("stream failed"))

	protocolsImplMock.EXPECT().ShutdownSocket(&protocolConn).AnyTimes().Return(nil)
	protocolsImplMock.EXPECT().ShutdownListener(&protocolListener).AnyTimes().Return(nil)

	tunnel.HandleTunnel(clientId, []*tunnelpb.Tunnel{&clientInitiatedTunnel}, getProtocolImplMock, false, nil, tunnelClientMock)
}
