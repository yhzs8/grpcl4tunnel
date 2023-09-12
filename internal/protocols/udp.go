package protocols

import (
	"fmt"
	pbtunnel "github.com/yhzs8/grpcl4tunnel/api/tunnel"
	"log"
	"net"
)

type UdpProtocol struct {
}

func (up UdpProtocol) SetupIncomingSocket(localHost string, localPort int32) (*ProtocolListener, error) {
	return nil, nil
}

func (up UdpProtocol) ListenIncomingBytes(listener *ProtocolListener, localHost string, localPort int32) (*ProtocolConn, error) {
	conn, err := net.ListenPacket(pbtunnel.Protocol_udp.String(), fmt.Sprintf("%s:%d", localHost, localPort))
	if err != nil {
		return nil, err
	}
	return &ProtocolConn{udpListenConn: &conn, ClosedChan: make(chan error)}, nil
}

func (up UdpProtocol) SetupOutgoingSocket(remoteHost string, remotePort int32) (*ProtocolConn, error) {
	conn, err := net.DialUDP(pbtunnel.Protocol_udp.String(), nil, &net.UDPAddr{IP: net.ParseIP(remoteHost), Port: int(remotePort)})
	if err != nil {
		return nil, err
	}
	return &ProtocolConn{udpDialConn: conn, ClosedChan: make(chan error)}, nil
}

func (up UdpProtocol) ReceiveBytesFromSocket(conn *ProtocolConn) ([]byte, error) {
	buffer := make([]byte, BufferSize)
	var (
		read int
		addr net.Addr
		err  error
	)
	if conn.udpListenConn != nil {
		udpListenConn := *conn.udpListenConn
		read, addr, err = udpListenConn.ReadFrom(buffer)
		if err != nil {
			return nil, err
		}
		conn.udpRemoteAddr = &addr
	} else {
		udpDialConn := *conn.udpDialConn
		read, addr, err = udpDialConn.ReadFrom(buffer)
		if err != nil {
			return nil, err
		}
	}
	log.Printf("%d bytes Received from UDP peer, %v", read, string(buffer[:read]))
	return buffer[:read], nil
}

func (up UdpProtocol) SendBytesToSocket(conn *ProtocolConn, bytes []byte) (int, error) {
	if conn.udpListenConn != nil {
		udpListenConn := *conn.udpListenConn
		log.Printf("sending %d bytes to UDP peer, %v", len(bytes), string(bytes))
		return udpListenConn.WriteTo(bytes, *conn.udpRemoteAddr)
	} else {
		udpDialConn := *conn.udpDialConn
		log.Printf("sending %d bytes to UDP peer, %v", len(bytes), string(bytes))
		return udpDialConn.Write(bytes)
	}
}

func (up UdpProtocol) ShutdownSocket(conn *ProtocolConn) error {
	close(conn.ClosedChan)
	if conn.udpListenConn != nil {
		udpListenConn := *conn.udpListenConn
		return udpListenConn.Close()
	} else if conn.udpDialConn != nil {
		udpDialConn := *conn.udpDialConn
		return udpDialConn.Close()
	}
	return nil
}

func (up UdpProtocol) ShutdownListener(listener *ProtocolListener) error {
	//UDP don't use listeners, nothing to do
	return nil
}
