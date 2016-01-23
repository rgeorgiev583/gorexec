package main

import (
	"crypto/tls"
	"flag"
	"io"
	"log"
	"net"
	"os"

	"github.com/docker/libchan"
	"github.com/docker/libchan/spdy"
)

// Some constants related to command-line options
const (
	addressFlagName         = "a"
	addressFlagDefaultValue = "127.0.0.1:9323"
	addressFlagDescription  = `specify the socket on the server node to which to connect
(<address> is of the format <host>:<port> where <host> may be an IP address or a hostname)`
)

// RemoteCommand specifies the command sent to the server to be executed remotely
type RemoteCommand struct {
	Cmd        string
	Args       []string
	Stdin      io.Writer
	Stdout     io.Reader
	Stderr     io.Reader
	StatusChan libchan.Sender
}

// CommandResponse specifies the returned status code from the remote execution
type CommandResponse struct {
	Status int
}

// Prints usage info for the program
func usage() {
	log.Printf("usage: %s [-a <address>] <command> [<arg> ...]\noptions:", os.Args[0])
	flag.PrintDefaults()
	log.Fatalln(`
  <command> [<arg> ...]  the command to execute remotely and its arguments
  (the arguments are optional)")
`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var addr string
	flag.StringVar(&addr, addressFlagName, addressFlagDefaultValue, addressFlagDescription)
	flag.Parse()
	if !flag.Parsed() {
		log.Printf("%s: invalid argument(s)\n", os.Args[0])
		usage()
	}

	var client net.Conn
	var err error
	if os.Getenv("USE_TLS") != "" {
		client, err = tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true})
	} else {
		client, err = net.Dial("tcp", addr)
	}
	if err != nil {
		log.Fatal(err)
	}

	p, err := spdy.NewSpdyStreamProvider(client, false)
	if err != nil {
		log.Fatal(err)
	}
	transport := spdy.NewTransport(p)
	sender, err := transport.NewSendChannel()
	if err != nil {
		log.Fatal(err)
	}

	receiver, remoteSender := libchan.Pipe()

	command := &RemoteCommand{
		Cmd:        flag.Args[0],
		Args:       flag.Args[1:],
		Stdin:      os.Stdin,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		StatusChan: remoteSender,
	}

	err = sender.Send(command)
	if err != nil {
		log.Fatal(err)
	}

	response := &CommandResponse{}
	err = receiver.Receive(response)
	if err != nil {
		log.Fatal(err)
	}

	os.Exit(response.Status)
}
