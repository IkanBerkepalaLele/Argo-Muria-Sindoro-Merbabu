package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"compnet-socket-labs/project/utils"

	"github.com/quic-go/quic-go"
)

var (
	DefaultServerIP   = "172.31.47.207"
	DefaultServerPort = "5565"
	ServerType        = "udp4"
	BufferSize        = 2048
	AppLayerProto     = "lrt-jakarta-2406495565"
)

func ResolveConfig() (string, string) {
	ip := os.Getenv("SERVER_ADDR")
	if ip == "" {
		ip = DefaultServerIP
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = DefaultServerPort
	}

	return ip, port
}

func ResolveALPN() string {
	alpn := os.Getenv("ALPN")
	if alpn == "" {
		alpn = AppLayerProto
	}
	return alpn
}

func Handler(packet utils.LRTJPIDSPacket) string {
	if packet.IsNewTrain == 1 {
		return fmt.Sprintf(
			"ADD | %d | %s | %02d:%02d",
			packet.TrainNumber,
			packet.Destination,
			packet.EstimatedArrivalHour,
			packet.EstimatedArrivalMinute,
		)
	}

	if packet.IsUpdateTrain == 1 {
		return fmt.Sprintf(
			"UPD | %d | %s | %02d:%02d",
			packet.TrainNumber,
			packet.Destination,
			packet.EstimatedArrivalHour,
			packet.EstimatedArrivalMinute,
		)
	}

	return ""
}

func readPacket(stream *quic.Stream) ([]byte, error) {
	header := make([]byte, 8)

	if _, err := io.ReadFull(stream, header); err != nil {
		return nil, err
	}

	destinationLength := int(header[7])

	rawMessage := make([]byte, 8+destinationLength)
	copy(rawMessage, header)

	if destinationLength > 0 {
		if _, err := io.ReadFull(stream, rawMessage[8:]); err != nil {
			return nil, err
		}
	}

	return rawMessage, nil
}

func streamHandler(stream *quic.Stream) {
	defer stream.Close()

	rawMessage, err := readPacket(stream)
	if err != nil {
		log.Printf("[subscriber] gagal membaca packet: %v\n", err)
		return
	}

	packet := utils.Decoder(rawMessage)

	message := Handler(packet)
	if message != "" {
		fmt.Println(message)
	}

	packet.IsAck = 1

	ack := utils.Encoder(packet)

	if _, err := stream.Write(ack); err != nil {
		log.Printf("[subscriber] gagal mengirim ACK: %v\n", err)
		return
	}

	fmt.Printf("[subscriber] ACK transaction %d sent\n", packet.TransactionId)
}

func connectionHandler(connection *quic.Conn) {
	fmt.Printf("[subscriber] Connection from %s\n", connection.RemoteAddr())

	for {
		stream, err := connection.AcceptStream(context.Background())
		if err != nil {
			return
		}

		go streamHandler(stream)
	}
}

func main() {
	serverIP, serverPort := ResolveConfig()
	alpn := ResolveALPN()

	address := net.JoinHostPort(serverIP, serverPort)

	udpAddress, err := net.ResolveUDPAddr(ServerType, address)
	if err != nil {
		log.Fatalln(err)
	}

	socket, err := net.ListenUDP(ServerType, udpAddress)
	if err != nil {
		log.Fatalln(err)
	}
	defer socket.Close()

	tlsConfig := &tls.Config{
		Certificates: utils.GenerateTLSSelfSignedCertificates(),
		NextProtos:   []string{alpn},
	}

	listener, err := quic.Listen(socket, tlsConfig, &quic.Config{})
	if err != nil {
		log.Fatalln(err)
	}
	defer listener.Close()

	fmt.Printf(
		"[subscriber] Listening on %s (ALPN: %s)\n",
		socket.LocalAddr(),
		alpn,
	)

	for {
		connection, err := listener.Accept(context.Background())
		if err != nil {
			log.Printf("[subscriber] accept error: %v\n", err)
			continue
		}

		go connectionHandler(connection)
	}
}
