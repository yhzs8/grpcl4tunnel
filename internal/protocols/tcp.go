package protocols

import (
	"fmt"
	tunnelpb "github.com/yhzs8/grpcl4tunnel/api/tunnel"
	"log"
	"net"
)

type TcpProtocol struct {
}

func (tp TcpProtocol) SetupIncomingSocket(localHost string, localPort int32) (*ProtocolListener, error) {
	listener, err := net.Listen(tunnelpb.Protocol_tcp.String(), fmt.Sprintf("%s:%d", localHost, localPort))
	if err != nil {
		return nil, err
	}
	return &ProtocolListener{tcpListener: &listener}, nil
}

func (tp TcpProtocol) ListenIncomingBytes(listener *ProtocolListener, localHost string, localPort int32) (*ProtocolConn, error) {
	tcpListener := *listener.tcpListener
	conn, err := tcpListener.Accept()
	if err != nil {
		return nil, err
	}
	return &ProtocolConn{tcpConn: &conn, ClosedChan: make(chan error)}, nil
}

func (tp TcpProtocol) SetupOutgoingSocket(remoteHost string, remotePort int32) (*ProtocolConn, error) {
	conn, err := net.Dial(tunnelpb.Protocol_tcp.String(), fmt.Sprintf("%s:%d", remoteHost, remotePort))
	if err != nil {
		return nil, err
	}
	return &ProtocolConn{tcpConn: &conn, ClosedChan: make(chan error)}, nil
}

func (tp TcpProtocol) ReceiveBytesFromSocket(conn *ProtocolConn) ([]byte, error) {
	buffer := make([]byte, BufferSize)
	tcpConn := *conn.tcpConn
	read, err := tcpConn.Read(buffer)
	if err != nil {
		return nil, err
	}
	log.Printf("%d bytes Received from TCP peer, %v", read, string(buffer[:read]))
	return buffer[:read], nil
}

func (tp TcpProtocol) SendBytesToSocket(conn *ProtocolConn, bytes []byte) (int, error) {
	tcpConn := *conn.tcpConn
	log.Printf("sending %d bytes to TCP peer, %v", len(bytes), string(bytes))
	return tcpConn.Write(bytes)
}

func (tp TcpProtocol) ShutdownSocket(conn *ProtocolConn) error {
	close(conn.ClosedChan)
	tcpConn := *conn.tcpConn
	return tcpConn.Close()
}

func (tp TcpProtocol) ShutdownListener(listener *ProtocolListener) error {
	tcpListener := *listener.tcpListener
	return tcpListener.Close()
}
