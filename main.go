package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"
)

const defaultBufferSize = 4096

// CommandLine struct stores command-line arguments.
type CommandLine struct {
	host    string
	port    uint64
	name    string
	tag     string
	xtrn    *string
	timeout time.Duration
}

// Read method parses command line args using the flag package.
func Read() *CommandLine {
	host := flag.String("host", "", "GoldMine host address")
	port := flag.Uint64("port", 0, "Goldmine rlogin port")
	name := flag.String("name", "", "Username")
	tag := flag.String("tag", "", "BBS tag (no brackets)")
	xtrn := flag.String("xtrn", "", "Gold Mine xtrn code (optional)") // Optional flag
	timeout := flag.Duration("timeout", 1*time.Second, "Byte receiving timeout after the input EOF occurs")

	flag.Parse()

	// Validate required flags
	if *host == "" || *port == 0 || *name == "" || *tag == "" {
		log.Fatalf(`Error: Missing required arguments.
Usage: goldmine-connect -host <host> -port <port> -name <username> -tag <BBS tag> [-xtrn <xtrn code>] [-timeout <timeout>]

Example: goldmine-connect -host example.com -port 2513 -name myUsername -tag myBBS

Required arguments:
  -host    The GoldMine host address to connect to.
  -port    The GoldMine rlogin port number.
  -name    Your username for the connection.
  -tag     The BBS tag (without brackets).

Optional arguments:
  -xtrn    Optional Gold Mine xtrn code.
  -timeout Byte receiving timeout, e.g., 1s, 500ms (default: 1s).`)
	}

	return &CommandLine{
		host:    *host,
		port:    *port,
		name:    *name,
		tag:     *tag,
		xtrn:    xtrn,
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

// Implementing Options interface methods for CommandLine
func (c *CommandLine) Host() string           { return c.host }
func (c *CommandLine) Port() uint64           { return c.port }
func (c *CommandLine) Timeout() time.Duration { return c.timeout }
func (c *CommandLine) Name() string           { return c.name }
func (c *CommandLine) Xtrn() *string          { return c.xtrn }
func (c *CommandLine) Tag() string            { return c.tag }

// TelnetClient represents a TCP client which is responsible for writing input data and printing response.
type TelnetClient struct {
	destination     *net.TCPAddr
	responseTimeout time.Duration
}

// NewTelnetClient creates a new TelnetClient instance.
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

// ProcessData method establishes a connection to the server and processes input/output data.
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

	// Conditionally include xtrn if it's provided
	localUsername := ""              // Placeholder: replace with actual local username if needed
	remoteUsername := options.Name() // Use the name from CommandLine struct
	tag := options.Tag()             // BBS tag from CommandLine struct

	handshake := fmt.Sprintf("\x00%s\x00[%s]%s\x00", localUsername, tag, remoteUsername)

	// Check if xtrn (termtype) is provided
	if options.Xtrn() != nil && *options.Xtrn() != "" {
		handshake += "xtrn=" + *options.Xtrn() + "\x00"
	} else {
		// Send an empty string followed by a null character for termtype if not provided
		handshake += "\x00"
	}

	// Write handshake to the connection
	if _, err := connection.Write([]byte(handshake)); err != nil {
		log.Fatalf("Failed to send rlogin handshake: %v", err)
		return
	}

	requestDataChannel := make(chan []byte)
	doneChannel := make(chan bool)
	responseDataChannel := make(chan []byte)
	closeSignal := make(chan bool) // Channel to signal server disconnection
	closing := false               // Flag to indicate if we're closing

	// Start data handling goroutines
	go t.readInputData(inputData, requestDataChannel, doneChannel)
	go t.readServerData(connection, responseDataChannel, closeSignal)

	afterEOFResponseTicker := time.NewTicker(t.responseTimeout)
	defer afterEOFResponseTicker.Stop()

	var afterEOFMode bool
	var somethingRead bool

	for {
		select {
		case request := <-requestDataChannel:
			if closing {
				log.Println("Connection closing; stopping writes.")
				return
			}
			if _, err := connection.Write(request); err != nil {
				log.Printf("Error occurred while writing to TCP socket: %v\n", err)
				return
			}
		case <-doneChannel:
			afterEOFMode = true
			closing = true // Set closing flag
		case response := <-responseDataChannel:
			if closing {
				log.Println("Connection closing; stopping reads.")
				return
			}
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
		case <-closeSignal:
			log.Println("Server disconnected. Exiting.")
			return
		}
	}
}

func (t *TelnetClient) readInputData(inputData io.Reader, toSend chan<- []byte, doneChannel chan<- bool) {
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
		// Send raw data
		toSend <- buffer[:n]
	}
}

func (t *TelnetClient) readServerData(connection *net.TCPConn, received chan<- []byte, closeSignal chan<- bool) {
	buffer := make([]byte, defaultBufferSize)

	for {
		n, err := connection.Read(buffer)
		if err != nil {
			if err == io.EOF {
				log.Println("Server closed the connection.")
				closeSignal <- true
				close(received)
				return
			}
			log.Printf("Error occurred while reading from server: %v\n", err)
			closeSignal <- true
			close(received)
			return
		}
		// Send raw bytes as-is
		received <- buffer[:n]
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

// resolveTCPAddr resolves a TCP address string.
func resolveTCPAddr(addr string) (*net.TCPAddr, error) {
	resolved, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("error occurred while resolving TCP address \"%v\": %v", addr, err)
	}
	return resolved, nil
}

// Main function
func main() {
	commandLine := Read()

	telnetClient, err := NewTelnetClient(commandLine)
	if err != nil {
		log.Fatalf("Failed to create TelnetClient: %v", err)
	}

	telnetClient.ProcessData(os.Stdin, os.Stdout, commandLine)
}
