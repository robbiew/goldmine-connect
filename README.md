# goldmine-connect

`goldmine-connect` is a command-line tool designed to connect to a GoldMine server using the rlogin protocol. This tool allows you to specify a host, port, username, and BBS tag, along with optional parameters for connection customization.

## Installation

1. **Clone the Repository**:
   ```bash
   git clone <repository-url>
   cd goldmine-connect
   ```

2. **Build the Project**:
   Make sure you have [Go installed](https://golang.org/doc/install), then run:
   ```bash
   go build -o goldmine-connect
   ```

3. **Run the Executable**:
   After building, you can run the executable:
   ```bash
   ./goldmine-connect -host <host> -port <port> -name <username> -tag <BBS tag>
   ```

## Usage

The `goldmine-connect` command requires several arguments to function properly. Some arguments are required, while others are optional.

### Required Arguments

- `-host` – Gold Mine server’s host address to connect to (set it to goldminedoors.com)
- `-port` – Gold Mine server’s rlogin port number (set it to 2513)
- `-name` – The BBS username for connecting to the server.
- `-tag` – The BBS tag (without brackets).

### Optional Arguments

- `-xtrn` – The optional Gold Mine xtrn code (leave empty if not needed or for the main menu).
- `-timeout` – Timeout for receiving bytes after EOF occurs (default: `1s`). Accepts durations such as `500ms`, `2s`, etc.

### Example Usage

```bash
./goldmine-connect -host example.com -port 2513 -name myUsername -tag myBBS
```

In this example:
- `-host` is set to `example.com`
- `-port` is set to `2513`
- `-name` is `myUsername`
- `-tag` is `myBBS`

You may also specify optional parameters, like so:

```bash
./goldmine-connect -host example.com -port 2513 -name myUsername -tag myBBS -xtrn WORD -timeout 500ms
```

### Error Messages

If required arguments are missing, you’ll see an error message like this:

```plaintext
Error: Missing required arguments.
Usage: goldmine-connect -host <host> -port <port> -name <username> -tag <BBS tag> [-xtrn <xtrn code>] [-timeout <timeout>]

Example: goldmine-connect -host example.com -port 2513 -name myUsername -tag myBBS

Required arguments:
  -host    The GoldMine host address to connect to.
  -port    The GoldMine rlogin port number.
  -name    Your username for the connection.
  -tag     The BBS tag (without brackets).

Optional arguments:
  -xtrn    Optional Gold Mine xtrn code.
  -timeout Byte receiving timeout, e.g., 1s, 500ms (default: 1s).
```

## Contributing

Feel free to open issues and submit pull requests to improve `goldmine-connect`. Please follow [Go’s best practices](https://golang.org/doc/effective_go.html) when submitting code.

## License

This project is licensed under the MIT License.
