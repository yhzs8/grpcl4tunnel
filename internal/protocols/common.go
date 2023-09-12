package protocols

import (
	"github.com/ishidawataru/sctp"
	pbtunnel "github.com/yhzs8/grpcl4tunnel/api/tunnel"
	"net"
)

type ProtocolListener struct {
	tcpListener  *net.Listener
	sctpListener *sctp.SCTPListener
}

type ProtocolConn struct {
	tcpConn       *net.Conn
	udpListenConn *net.PacketConn
	udpRemoteAddr *net.Addr
	udpDialConn   *net.UDPConn
	sctpConn      *sctp.SCTPConn
	ClosedChan    chan error
}

type ProtocolInterface interface {
	SetupIncomingSocket(localHost string, localPort int32) (*ProtocolListener, error)
	ListenIncomingBytes(listener *ProtocolListener, localHost string, localPort int32) (*ProtocolConn, error)
	SetupOutgoingSocket(remoteHost string, remotePort int32) (*ProtocolConn, error)
	ReceiveBytesFromSocket(conn *ProtocolConn) ([]byte, error)
	SendBytesToSocket(conn *ProtocolConn, bytes []byte) (int, error)
	ShutdownSocket(conn *ProtocolConn) error
	ShutdownListener(listener *ProtocolListener) error
}

const BufferSize = 1500

type GetProtocolImplInterface interface {
	GetProtocolImpl(protocol pbtunnel.Protocol) ProtocolInterface
}

type ProductionProtocolImpl struct {
}

func (ProductionProtocolImpl) GetProtocolImpl(protocol pbtunnel.Protocol) ProtocolInterface {
	switch protocol {
	case pbtunnel.Protocol_tcp:
		return TcpProtocol{}
	case pbtunnel.Protocol_udp:
		return UdpProtocol{}
	case pbtunnel.Protocol_sctp:
		return SctpProtocol{}
	}
	return TcpProtocol{}
}
