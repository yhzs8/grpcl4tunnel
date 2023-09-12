package protocols

import (
	"github.com/ishidawataru/sctp"
	pbtunnel "github.com/yhzs8/grpcl4tunnel/api/tunnel"
	"log"
	"net"
)

type SctpProtocol struct {
}

func (sp SctpProtocol) SetupIncomingSocket(localHost string, localPort int32) (*ProtocolListener, error) {
	listener, err := sctp.ListenSCTP(pbtunnel.Protocol_sctp.String(), &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.ParseIP(localHost)}}, Port: int(localPort)})
	if err != nil {
		return nil, err
	}
	return &ProtocolListener{sctpListener: listener}, nil
}

func (sp SctpProtocol) ListenIncomingBytes(listener *ProtocolListener, localHost string, localPort int32) (*ProtocolConn, error) {
	conn, err := listener.sctpListener.AcceptSCTP()
	if err != nil {
		return nil, err
	}
	return &ProtocolConn{sctpConn: conn, ClosedChan: make(chan error)}, nil
}

func (sp SctpProtocol) SetupOutgoingSocket(remoteHost string, remotePort int32) (*ProtocolConn, error) {
	addr, err := net.ResolveIPAddr("ip", remoteHost)
	if err != nil {
		return nil, err
	}
	conn, err := sctp.DialSCTP(pbtunnel.Protocol_sctp.String(), nil, &sctp.SCTPAddr{IPAddrs: []net.IPAddr{*addr}, Port: int(remotePort)})
	if err != nil {
		return nil, err
	}
	return &ProtocolConn{sctpConn: conn, ClosedChan: make(chan error)}, nil
}

func (sp SctpProtocol) ReceiveBytesFromSocket(conn *ProtocolConn) ([]byte, error) {
	buffer := make([]byte, BufferSize)
	read, err := conn.sctpConn.Read(buffer)
	if err != nil {
		return nil, err
	}
	log.Printf("%d bytes Received from SCTP peer, %v", read, string(buffer[:read]))
	return buffer[:read], nil
}

func (sp SctpProtocol) SendBytesToSocket(conn *ProtocolConn, bytes []byte) (int, error) {
	log.Printf("sending %d bytes to SCTP peer, %v", len(bytes), string(bytes))
	return conn.sctpConn.Write(bytes)
}

func (sp SctpProtocol) ShutdownSocket(conn *ProtocolConn) error {
	close(conn.ClosedChan)
	return conn.sctpConn.Close()
}

func (sp SctpProtocol) ShutdownListener(listener *ProtocolListener) error {
	return listener.sctpListener.Close()
}
