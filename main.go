package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"

	"gopkg.in/alecthomas/kingpin.v2"
)

const defaultBufferSize = 4096

// CommandLine struct stores command-line arguments.
type CommandLine struct {
	host    string
	port    uint64
	name    string
	tag     string
	xtrn    *string // Make xtrn a pointer to distinguish if it's provided
	timeout time.Duration
}

// Implementing Options interface methods
func (c *CommandLine) Host() string           { return c.host }
func (c *CommandLine) Port() uint64           { return c.port }
func (c *CommandLine) Timeout() time.Duration { return c.timeout }
func (c *CommandLine) Name() string           { return c.name }
func (c *CommandLine) Xtrn() *string          { return c.xtrn } // Return pointer to check for nil
func (c *CommandLine) Tag() string            { return c.tag }

// Read method returns valid options read from command line args.
func Read() *CommandLine {
	host := kingpin.Arg("host", "GoldMine host address").Required().String()
	port := kingpin.Arg("port", "Goldmine rlogin port").Required().Uint64()
	name := kingpin.Arg("name", "username").Required().String()
	tag := kingpin.Arg("tag", "BBS tag (no brackets)").Required().String()
	xtrn := kingpin.Arg("xtrn", "Gold Mine xtrn code").String() // No longer required
	timeout := kingpin.Flag("timeout", "Byte receiving timeout after the input EOF occurs").Short('t').Default("1s").Duration()

	kingpin.Parse()

	return &CommandLine{
		host:    *host,
		port:    *port,
		name:    *name,
		tag:     *tag,
		xtrn:    xtrn, // Pointer is automatically assigned if not provided
		timeout: *timeout,
	}
}

// Options interface defines the client settings.
type Options interface {
	Host() string
	Port() uint64
	Timeout() time.Duration
	Name() string
	Xtrn() *string
	Tag() string
}

// TelnetClient represents a TCP client which is responsible for writing input data and printing response.
type TelnetClient struct {
	destination     *net.TCPAddr
	responseTimeout time.Duration
}

func NewTelnetClient(options Options) (*TelnetClient, error) {
	tcpAddr := createTCPAddr(options)
	resolved, err := resolveTCPAddr(tcpAddr)
	if err != nil {
		return nil, err
	}

	return &TelnetClient{
		destination:     resolved,
		responseTimeout: options.Timeout(),
	}, nil
}

func (t *TelnetClient) ProcessData(inputData io.Reader, outputData io.Writer, options Options) {
	connection, err := net.DialTCP("tcp", nil, t.destination)
	if err != nil {
		log.Fatalf("Error occurred while connecting to address \"%v\": %v\n", t.destination.String(), err)
		return
	}
	defer func() {
		connection.Close()
		log.Println("Connection closed.")
	}()

	// Send initial rlogin handshake using the parameters from CommandLine
	localUsername := "local_username" // Placeholder: replace with actual local username if needed
	remoteUsername := options.Name()  // Use the name from CommandLine struct
	tag := options.Tag()              // BBS tag from CommandLine struct

	// Conditionally include xtrn if it's provided
	handshake := fmt.Sprintf("\x00%s\x00[%s]%s\x00", localUsername, tag, remoteUsername)
	if options.Xtrn() != nil && *options.Xtrn() != "" {
		handshake += "xtrn=" + *options.Xtrn() + "\x00"
	}

	if _, err := connection.Write([]byte(handshake)); err != nil {
		log.Fatalf("Failed to send rlogin handshake: %v", err)
		return
	}

	requestDataChannel := make(chan []byte)
	doneChannel := make(chan bool)
	responseDataChannel := make(chan []byte)
	serverDisconnected := make(chan bool) // Channel to signal server disconnection

	// Start data handling goroutines
	go t.readInputData(inputData, requestDataChannel, doneChannel)
	go t.readServerData(connection, responseDataChannel, serverDisconnected)

	afterEOFResponseTicker := time.NewTicker(t.responseTimeout)
	defer afterEOFResponseTicker.Stop()

	var afterEOFMode bool
	var somethingRead bool

	for {
		select {
		case request := <-requestDataChannel:
			if _, err := connection.Write(request); err != nil {
				log.Printf("Error occurred while writing to TCP socket: %v\n", err)
				return
			}
		case <-doneChannel:
			afterEOFMode = true
		case response := <-responseDataChannel:
			outputData.Write(response)
			somethingRead = true
			if afterEOFMode {
				afterEOFResponseTicker.Stop()
				afterEOFResponseTicker = time.NewTicker(t.responseTimeout)
			}
		case <-afterEOFResponseTicker.C:
			if afterEOFMode && !somethingRead {
				log.Println("Connection timeout with no response received.")
				return
			}
		case <-serverDisconnected:
			log.Println("Server disconnected.")
			return
		}
	}
}

// Modified readServerData to detect server disconnection and send signal
func (t *TelnetClient) readServerData(connection *net.TCPConn, received chan<- []byte, serverDisconnected chan<- bool) {
	defer close(received)           // Ensure received channel is closed
	defer close(serverDisconnected) // Close serverDisconnected when done

	buffer := make([]byte, defaultBufferSize)
	for {
		n, err := connection.Read(buffer)
		if err != nil {
			if err == io.EOF {
				log.Println("Server closed the connection.")
				serverDisconnected <- true // Signal server disconnection
				return
			}
			log.Printf("Error occurred while reading from server: %v\n", err)
			serverDisconnected <- true // Signal server disconnection on error
			return
		}
		received <- buffer[:n]
	}
}

func (t *TelnetClient) readInputData(inputData io.Reader, toSent chan<- []byte, doneChannel chan<- bool) {
	buffer := make([]byte, defaultBufferSize)
	reader := bufio.NewReader(inputData)

	for {
		n, err := reader.Read(buffer)
		if err != nil {
			if err == io.EOF {
				doneChannel <- true
				return
			}
			log.Fatalf("Error reading input data: %v", err)
		}
		toSent <- buffer[:n]
	}
}

// createTCPAddr builds a TCP address string.
func createTCPAddr(options Options) string {
	var buffer bytes.Buffer
	buffer.WriteString(options.Host())
	buffer.WriteByte(':')
	buffer.WriteString(fmt.Sprintf("%d", options.Port()))
	return buffer.String()
}

func resolveTCPAddr(addr string) (*net.TCPAddr, error) {
	resolved, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("error occurred while resolving TCP address \"%v\": %v", addr, err)
	}
	return resolved, nil
}

func main() {
	commandLine := Read()

	telnetClient, err := NewTelnetClient(commandLine)
	if err != nil {
		log.Fatalf("Failed to create TelnetClient: %v", err)
	}

	telnetClient.ProcessData(os.Stdin, os.Stdout, commandLine)
}
