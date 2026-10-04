package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/qlog"
)

var (
	DefaultServerIP   = "127.0.0.1"
	DefaultServerPort = "54321"
	ServerType        = "udp4"
	BufferSize        = 2048
	AppLayerProto     = "jarkom-quic-sample-ghozam"
	LogDir            = "logs"
	SSLKeyLogFileName = "ssl-key.log"
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

func main() {
	keylogFlag := flag.Bool("keylog", false, "Enable TLS key logging to logs/ssl-key.log for Wireshark inspection")
	qlogFlag := flag.Bool("qlog", false, "Enable QLOG event tracing to logs/*.sqlog")
	flag.Parse()

	if *keylogFlag || *qlogFlag {
		if err := os.MkdirAll(LogDir, 0755); err != nil {
			log.Fatalf("failed to create logs directory: %v", err)
		}
	}

	serverIP, serverPort := ResolveConfig()
	alpn := ResolveALPN()

	fmt.Printf("QUIC Client Socket Program Example in Go\n")
	fmt.Printf("[%s] Connecting to %s\n", ServerType, net.JoinHostPort(serverIP, serverPort))

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true, // Self-signed test certificate
		NextProtos:         []string{alpn},
	}

	if *keylogFlag {
		keyLogPath := filepath.Join(LogDir, SSLKeyLogFileName)
		keyLogFile, err := os.OpenFile(keyLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			log.Fatalf("failed to open keylog file: %v", err)
		}
		defer keyLogFile.Close()
		tlsConfig.KeyLogWriter = keyLogFile
		fmt.Printf("[quic] TLS Key logging enabled -> %s\n", keyLogPath)
	}

	quicConfig := &quic.Config{}
	if *qlogFlag {
		if os.Getenv("QLOGDIR") == "" {
			_ = os.Setenv("QLOGDIR", LogDir)
		}
		quicConfig.Tracer = qlog.DefaultConnectionTracer
		fmt.Printf("[quic] QLOG event tracing enabled -> %s/*.sqlog\n", LogDir)
	}

	connection, err := quic.DialAddr(context.Background(), net.JoinHostPort(serverIP, serverPort), tlsConfig, quicConfig)
	if err != nil {
		log.Fatalln(err)
	}
	defer connection.CloseWithError(0x0, "No Error")

	fmt.Printf("[quic] Dialling from %s to %s\n", connection.LocalAddr(), connection.RemoteAddr())

	fmt.Printf("[quic] Input message to be sent to server: ")
	message, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		log.Fatalln(err)
	}

	stream1, err := connection.OpenStreamSync(context.Background())
	if err != nil {
		log.Fatalln(err)
	}

	stream2, err := connection.OpenStreamSync(context.Background())
	if err != nil {
		log.Fatalln(err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go streamHelper(stream1, message, &wg)
	go streamHelper(stream2, message, &wg)

	wg.Wait()
}

func streamHelper(stream *quic.Stream, message string, wg *sync.WaitGroup) {
	defer wg.Done()
	defer stream.Close()

	fmt.Printf("[quic] Opened bidirectional stream %d\n", stream.StreamID())
	fmt.Printf("[quic] [Stream ID: %d] Sending message '%s'\n",
		stream.StreamID(), message)

	_, err := stream.Write([]byte(message))
	if err != nil {
		log.Printf("[quic] [Stream ID: %d] Write error: %v\n",
			stream.StreamID(), err)
		return
	}

	receiveBuffer := make([]byte, BufferSize)

	receiveLength, err := stream.Read(receiveBuffer)
	if err != nil && err != io.EOF {
		log.Printf("[quic] [Stream ID: %d] Read error: %v\n",
			stream.StreamID(), err)
		return
	}

	response := string(receiveBuffer[:receiveLength])

	fmt.Printf("[quic] [Stream ID: %d] Response from server: %s\n",
		stream.StreamID(), response)
}
