# gorexec


**gorexec** is a simple application written in Go that executes a shell command
specified as arguments on the command line on a remote host.
This was written for a course project for the Network Programming course at
FMI @ Sofia University.


## Architecture

It consists of a server part (where the command is executed) and a client part
(which sends the command for execution).  They use the **libchan** Golang
library to communicate with each other.
**libchan** uses a mechanism similar to Go channels to implement the connection
layer of the **gorexec** communication protocol.  It provides the functionality
of **channels**, which operate in a way similar to websockets, or the SCTP
protocol (they main PDU of the protocol is a **message**, which is a string of
bytes with a fixed length where there is a strictly defined boundary between
units).  Programmers using the library may implement their own (sub)protocols
and/or data structures which are wrapped in messages.
Messages arrive in the same order they are sent, and can be sent in only one
direction (for this reason, two channels: one for receiving, and one for
sending, are used for full-duplex communication).
**libchan** works atop a transport protocol that provides a reliable two-way
bytestream (TCP, UDP, etc.)  The reference implementation uses the
[SPDY](https://en.wikipedia.org/wiki/SPDY) protocol.
(It can also be used over Unix sockets, websockets, HTTP, etc.)


## Protocol

The server and client communicate in the following manner:

On the client side:

1. A generic TCP connection is created from the client to the server to the
   host provided through the `-h` command-line option on the port provided
   through the `-p` option.
   If the `USE_TLS` environment variable is set, the connection is created
   with a TLS encryption layer.
2. A new SPDY stream provider (a.k.a. codec for the SPDY protocol) is created.
3. A new SPDY transport layer is created (SPDY is the underlying protocol) using
   the provider.
4. A new SPDY send-channel is created using the transport layer.
5. A new receive-channel is created using the `Pipe` library function.
6. A new `RemoteCommand` message structure is created containing the
   command-line arguments passed to the client CLI utility for `Cmd` and `Args`
   respectively, and the STDIN, STDOUT and STDERR handles of the CLI utility.
7. The command is sent to the server through the send-channel.
8. A `CommandResponse` structure is created which would contain the exit status
   code of the process started by the command.
9. The client exits with the provided exit code.

On the server side:

1.  The server looks for the files specified using the `TLS_CERT` and `TLS_KEY`
    environment variables.
2.  If it detects a TLS certificate and a key file, it loads the X.509 key pair
    from them and starts listening for a TCP+TLS connection.  Otherwise, it tries
    to listen for a pure TCP connection.
3.  When a connection is detected, the server accepts it.
4.  A new SPDY stream provider (a.k.a. codec for the SPDY protocol) is created.
5.  A concurrent routine is started for handling the connection
    (so that multiple parallel connections could be serviced).
6.  The routine blocks until the server receives a notification that data has
    been sent from the client (namely, a `RemoteCommand` containing the command,
    its arguments, the STDIN/STDOUT/STDERR handles, and a `Sender` through which
    the exit status of the process would be sent when it finishes execution).
7.  When input is received, another routine is created for receiving said
    message.
8.  The message is read into a `RemoteCommand` structure.
9.  A `Cmd` structure (for local command execution) is created using the info
    inside the message, and the STDOUT and STDERR inside the structure are tied
    to the handles inside the message.
10. The `Cmd`'s STDIN is piped into the STDIN handle from the message.
11. The `Cmd` is executed using its `Run` method (i.e. synchronous execution).
12. When the process finishes, the STDOUT and STDERR handles inside the
    `RemoteCommand` are closed.
13. The exit status of the process is retrieved from the return value of `Run`,
    and is assigned to a newly created `CommandResponse` structure.
14. The return status is sent through the `Sender`.
15. The server continues with the next command and, respectively, connection.


## Installation and usage (instructions for Linux)

### Before installation

1. Install Go using your distribution's package manager.
2. Verify that Go is installed on the system and that the GOPATH environment
variable is set to point to a "go" directory somewhere, and is included in PATH:

~~~~
$ sudo apt-get install go
$ mkdir ~/go
$ cat >> ~/.profile
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin
~~~~
3. Install the following dependencies:

~~~~
$ go get github.com/dmcgowan/msgpack
$ go get github.com/docker/spdystream
$ go get github.com/docker/libchan
~~~~


### Quick installation

~~~~
$ go get github.com/rgeorgiev583/gorexec/gorexec
~~~~


### Installation from source

1. Clone the repo into `$GOPATH/src/github.com/rgeorgiev583/gorexec`.
2. In each of the two subdirecties there:

~~~~
$ go install
~~~~


### Usage (when installed)

**Server (IP: 10.0.0.1)**

~~~~
$ gorexecd 1234
~~~~

**Client**

~~~~
$ gorexec -a 10.0.0.1:1234 /bin/echo "hello"
hello
$ gorexec -a 10.0.0.1:1234 /bin/sh -c "exit 4"
$ echo $?
4
~~~~


### Usage (without installation)

**Server**

~~~~
$ cd gorexecd
$ go build
$ ./gorexecd
~~~~

**Client**

~~~~
$ cd gorexec
$ go build
$ ./gorexec /bin/echo "hello"
hello
$ ./gorexec /bin/sh -c "exit 4"
$ echo $?
4
~~~~


### Usage with custom IP and port (without installation)

**Server (IP: 10.0.0.1)**

~~~~
$ cd gorexecd
$ go build
$ ./gorexecd 1234
~~~~

**Client**

~~~~
$ cd gorexec
$ go build
$ ./gorexec -a 10.0.0.1:1234 /bin/echo "hello"
hello
$ ./gorexec -a 10.0.0.1:1234 /bin/sh -c "exit 4"
$ echo $?
4
~~~~


### Usage with TLS (without installation)

**Server**

~~~~
$ TLS_CERT=./cert.pem TLS_KEY=./key.pem ./gorexecd
~~~~

**Client**

~~~~
$ USE_TLS=true ./gorexec /bin/echo "hello"
hello
~~~~
