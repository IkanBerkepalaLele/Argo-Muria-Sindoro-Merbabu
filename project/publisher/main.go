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

func sendPacket(connection *quic.Conn, packet utils.LRTJPIDSPacket) error {
	stream, err := connection.OpenStreamSync(context.Background())
	if err != nil {
		return err
	}
	defer stream.Close()

	rawMessage := utils.Encoder(packet)

	fmt.Printf(
		"[publisher] Sending transaction %d to %s\n",
		packet.TransactionId,
		packet.Destination,
	)

	if _, err := stream.Write(rawMessage); err != nil {
		return err
	}

	rawAck, err := readPacket(stream)
	if err != nil {
		return err
	}

	ack := utils.Decoder(rawAck)

	if ack.IsAck != 1 {
		return fmt.Errorf(
			"transaction %d received invalid ACK",
			packet.TransactionId,
		)
	}

	fmt.Printf(
		"[publisher] ACK transaction %d received\n",
		ack.TransactionId,
	)

	return nil
}

func main() {
	serverIP, serverPort := ResolveConfig()
	alpn := ResolveALPN()

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{alpn},
	}

	address := net.JoinHostPort(serverIP, serverPort)

	fmt.Printf(
		"[publisher] Connecting to %s (ALPN: %s)\n",
		address,
		alpn,
	)

	connection, err := quic.DialAddr(
		context.Background(),
		address,
		tlsConfig,
		&quic.Config{},
	)
	if err != nil {
		log.Fatalln(err)
	}
	defer connection.CloseWithError(0, "No Error")

	packetA := utils.LRTJPIDSPacket{
		LRTJPIDSPacketFixed: utils.LRTJPIDSPacketFixed{
			TransactionId:          1,
			IsNewTrain:             1,
			TrainNumber:            1002,
			EstimatedArrivalHour:   5,
			EstimatedArrivalMinute: 34,
			DestinationLength:      uint8(len("Manggarai")),
		},
		Destination: "Manggarai",
	}

	packetB := utils.LRTJPIDSPacket{
		LRTJPIDSPacketFixed: utils.LRTJPIDSPacketFixed{
			TransactionId:          2,
			IsUpdateTrain:          1,
			TrainNumber:            1002,
			EstimatedArrivalHour:   5,
			EstimatedArrivalMinute: 35,
			DestinationLength:      uint8(len("Velodrome")),
		},
		Destination: "Velodrome",
	}

	if err := sendPacket(connection, packetA); err != nil {
		log.Fatalln(err)
	}

	if err := sendPacket(connection, packetB); err != nil {
		log.Fatalln(err)
	}
}
