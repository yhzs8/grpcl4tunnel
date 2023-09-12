package tunnel

import (
	"fmt"
	pbtunnel "github.com/yhzs8/grpcl4tunnel/api/tunnel"
	"github.com/yhzs8/grpcl4tunnel/internal/protocols"
	"io"
	"log"
	"sync"
)

var connMap sync.Map = sync.Map{}

var getProtocolImpl protocols.GetProtocolImplInterface

const sendClosedMessageToGrpcStream = "Sent closed message to gRPC stream"

func lookupConn(key string) *protocols.ProtocolConn {
	conn, _ := connMap.Load(key)
	if conn != nil {
		return conn.(*protocols.ProtocolConn)
	}
	return nil
}

func constructKey(clientId string, tunnel *pbtunnel.Tunnel) string {
	//key format: <client_id>_<si/ci>_<protocol>_<localHost>_<localPort>_<remoteHost>_<remotePort>
	var serverInitiatedOrClientInitiated string
	if tunnel.IsServerInitiated {
		serverInitiatedOrClientInitiated = "si"
	} else {
		serverInitiatedOrClientInitiated = "ci"
	}
	return fmt.Sprintf("%s_%s_%s_%s_%d_%s_%d", clientId, serverInitiatedOrClientInitiated, tunnel.Protocol.String(), tunnel.LocalHost, tunnel.LocalPort, tunnel.RemoteHost, tunnel.RemotePort)
}

func HandleTunnel(clientId string, tunnels []*pbtunnel.Tunnel, getProtocolImplInput protocols.GetProtocolImplInterface, serverStream pbtunnel.TunnelService_TunnelChatServer, clientStream pbtunnel.TunnelService_TunnelChatClient) error {
	getProtocolImpl = getProtocolImplInput

	streamErrorChan := make(chan error)
	for _, tunnel := range tunnels {
		go listeningTunnelSession(clientId, tunnel, serverStream, clientStream, streamErrorChan)
	}
	for {
		received, err := recvStream(serverStream, clientStream)
		if err != nil {
			log.Printf("Permanent error when stream.Recv(): %v", err.Error())
			close(streamErrorChan)
			return err
		}
		log.Printf("%d bytes received from gRPC stream: %v", len(received.Content), string(received.Content))
		key := constructKey(clientId, received.Tunnel)
		protocolImpl := getProtocolImpl.GetProtocolImpl(received.Tunnel.Protocol)
		conn := lookupConn(key)
		//isServerInitiated = FALSE && (serverStream != nil) = TRUE ----> true
		//isServerInitiated = FALSE && (serverStream != nil) = FALSE ---> false
		//isServerInitiated = TRUE  && (serverStream != nil) = TRUE .---> false
		//isServerInitiated = TRUE  && (serverStream != nil) = FALSE ---> true
		if conn == nil && received.Tunnel.IsServerInitiated != (serverStream != nil) {
			conn, err = protocolImpl.SetupOutgoingSocket(received.Tunnel.RemoteHost, received.Tunnel.RemotePort)
			if err != nil {
				log.Printf("Error received when protocolImpl.SetupOutgoingSocket(): %v", err.Error())
				sendStream(serverStream, clientStream, received.Tunnel, true, nil)
				log.Println(sendClosedMessageToGrpcStream)
				continue
			}
			connMap.Store(key, conn)
			go dialingTunnelSession(key, received.Tunnel, protocolImpl, serverStream, clientStream, conn, streamErrorChan)
		}
		if received.Closed {
			select {
			case conn.ClosedChan <- io.EOF:
			default:
			}
			continue
		}
		_, err = protocolImpl.SendBytesToSocket(conn, received.Content)
		if err != nil {
			log.Printf("Error received when protocolImpl.SendBytesToSocket(): %v", err.Error())
			continue
		}
	}
}

func listeningTunnelSession(clientId string, tunnel *pbtunnel.Tunnel, serverStream pbtunnel.TunnelService_TunnelChatServer, clientStream pbtunnel.TunnelService_TunnelChatClient, streamErrorChan chan error) error {
	protocolImpl := getProtocolImpl.GetProtocolImpl(tunnel.Protocol)
	listener, err := protocolImpl.SetupIncomingSocket(tunnel.LocalHost, tunnel.LocalPort)
	if err != nil {
		return err
	}
	defer protocolImpl.ShutdownListener(listener)
	for {
		err, done := listeningTunnelSessionLoop(clientId, tunnel, serverStream, clientStream, streamErrorChan, protocolImpl, listener)
		if done {
			return err
		}
	}
}

func listeningTunnelSessionLoop(clientId string, tunnel *pbtunnel.Tunnel, serverStream pbtunnel.TunnelService_TunnelChatServer, clientStream pbtunnel.TunnelService_TunnelChatClient, streamErrorChan chan error, protocolImpl protocols.ProtocolInterface, listener *protocols.ProtocolListener) (error, bool) {
	socketErrorChan := make(chan error)
	socketToStreamChan := make(chan []byte)

	conn, err := protocolImpl.ListenIncomingBytes(listener, tunnel.LocalHost, tunnel.LocalPort)
	if err != nil {
		return err, true
	}
	key := constructKey(clientId, tunnel)
	connMap.Store(key, conn)
	defer func() {
		protocolImpl.ShutdownSocket(conn)
		connMap.Delete(key)
	}()

	go func() {
		defer func() {
			close(socketErrorChan)
			close(socketToStreamChan)
		}()
		for {
			toBeSentToStream, err := protocolImpl.ReceiveBytesFromSocket(conn)
			if err != nil {
				log.Printf("error received when protocolImpl.ReceiveBytesFromSocket(): %v", err)
				select {
				case socketErrorChan <- err:
				default:
				}
				return
			}
			select {
			case socketToStreamChan <- toBeSentToStream:
			default:
			}
		}
	}()
	for {
		select {
		case err := <-streamErrorChan:
			log.Println("Received permanent error from gRPC stream")
			return err, true
		case err := <-conn.ClosedChan:
			log.Println("Received closed message from gRPC stream")
			if err == io.EOF {
				return err, false
			}
			return err, true
		case err := <-socketErrorChan:
			sendStream(serverStream, clientStream, tunnel, true, nil)
			log.Printf(sendClosedMessageToGrpcStream)
			return err, false
		case toBeSentToStream := <-socketToStreamChan:
			err := sendStream(serverStream, clientStream, tunnel, false, toBeSentToStream)
			if err != nil {
				return err, true
			}
			log.Printf("%d bytes sent to gRPC stream: %v", len(toBeSentToStream), string(toBeSentToStream))
		}
	}
}

func dialingTunnelSession(key string, tunnel *pbtunnel.Tunnel, protocolImpl protocols.ProtocolInterface, serverStream pbtunnel.TunnelService_TunnelChatServer, clientStream pbtunnel.TunnelService_TunnelChatClient, conn *protocols.ProtocolConn, streamErrorChan chan error) error {
	socketErrorChan := make(chan error)
	socketToStreamChan := make(chan []byte)

	connMap.Store(key, conn)
	defer func() {
		protocolImpl.ShutdownSocket(conn)
		connMap.Delete(key)
	}()

	go func() {
		defer func() {
			close(socketErrorChan)
			close(socketToStreamChan)
		}()
		for {
			toBeSentToStream, err := protocolImpl.ReceiveBytesFromSocket(conn)
			if err != nil {
				log.Printf("Error received when protocolImpl.ReceiveBytesFromSocket(): %v", err.Error())
				select {
				case socketErrorChan <- err:
				default:
				}
				return
			}
			select {
			case socketToStreamChan <- toBeSentToStream:
			default:
			}
		}
	}()
	for {
		select {
		case err := <-streamErrorChan:
			log.Println("Received permanent error from gRPC stream")
			return err
		case err := <-conn.ClosedChan:
			log.Println("Received closed message from gRPC stream")
			return err
		case err := <-socketErrorChan:
			sendStream(serverStream, clientStream, tunnel, true, nil)
			log.Println(sendClosedMessageToGrpcStream)
			return err
		case toBeSentToStream := <-socketToStreamChan:
			err := sendStream(serverStream, clientStream, tunnel, false, toBeSentToStream)
			if err != nil {
				return err
			}
			log.Printf("%d bytes sent to gRPC stream: %v", len(toBeSentToStream), string(toBeSentToStream))
		}
	}
}

func recvStream(serverStream pbtunnel.TunnelService_TunnelChatServer, clientStream pbtunnel.TunnelService_TunnelChatClient) (*pbtunnel.TunnelMessage, error) {
	if serverStream != nil {
		return serverStream.Recv()
	} else {
		return clientStream.Recv()
	}
}

func sendStream(serverStream pbtunnel.TunnelService_TunnelChatServer, clientStream pbtunnel.TunnelService_TunnelChatClient, tunnel *pbtunnel.Tunnel, closed bool, toBeSentToStream []byte) error {
	var err error
	if serverStream != nil {
		if closed {
			err = serverStream.Send(&pbtunnel.TunnelMessage{Tunnel: tunnel, Closed: true})
		} else {
			err = serverStream.Send(&pbtunnel.TunnelMessage{Tunnel: tunnel, Content: toBeSentToStream, Closed: false})
		}
	} else {
		if closed {
			err = clientStream.Send(&pbtunnel.TunnelMessage{Tunnel: tunnel, Closed: true})
		} else {
			err = clientStream.Send(&pbtunnel.TunnelMessage{Tunnel: tunnel, Content: toBeSentToStream, Closed: false})
		}
	}
	if err != nil {
		log.Printf("Error received when stream.Send(): %v", err.Error())
		return err
	}
	return nil
}
