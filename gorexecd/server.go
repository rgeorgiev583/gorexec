package main

import (
	"crypto/tls"
	"flag"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"syscall"

	"github.com/docker/libchan"
	"github.com/docker/libchan/spdy"
)

// Some constants related to command-line options
const (
	portFlagName         = "p"
	portFlagDefaultValue = "9323"
	portFlagDescription  = "specify the port number on which to listen for connections"
)

// RemoteCommand specifies the command sent by the client to be executed locally
type RemoteCommand struct {
	Cmd        string
	Args       []string
	Stdin      io.Reader
	Stdout     io.WriteCloser
	Stderr     io.WriteCloser
	StatusChan libchan.Sender
}

// CommandResponse specifies the response code to return to the client
type CommandResponse struct {
	Status int
}

// Prints the usage info for the program
func usage() {
	log.Printf("usage: %s [<port>]\noptions:", os.Args[0])
	flag.PrintDefaults()
	log.Fatalln("")
}

func main() {
	var port string
	flag.StringVar(&port, portFlagName, portFlagDefaultValue, portFlagDescription)
	flag.Parse()
	if !flag.Parsed() {
		log.Printf("%s: invalid argument(s)\n", os.Args[0])
		usage()
	}

	cert := os.Getenv("TLS_CERT")
	key := os.Getenv("TLS_KEY")

	var listener net.Listener
	if cert != "" && key != "" {
		tlsCert, err := tls.LoadX509KeyPair(cert, key)
		if err != nil {
			log.Fatal(err)
		}

		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
			Certificates:       []tls.Certificate{tlsCert},
		}

		listener, err = tls.Listen("tcp", "127.0.0.1:"+port, tlsConfig)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		var err error
		listener, err = net.Listen("tcp", "127.0.0.1:"+port)
		if err != nil {
			log.Fatal(err)
		}
	}

	for {
		c, err := listener.Accept()
		if err != nil {
			log.Print(err)
			break
		}
		p, err := spdy.NewSpdyStreamProvider(c, true)
		if err != nil {
			log.Print(err)
			break
		}
		t := spdy.NewTransport(p)

		go func() {
			for {
				receiver, err := t.WaitReceiveChannel()
				if err != nil {
					log.Print(err)
					break
				}

				go func() {
					for {
						command := &RemoteCommand{}
						err := receiver.Receive(command)
						if err != nil {
							log.Print(err)
							break
						}

						cmd := exec.Command(command.Cmd, command.Args...)
						cmd.Stdout = command.Stdout
						cmd.Stderr = command.Stderr

						stdin, err := cmd.StdinPipe()
						if err != nil {
							log.Print(err)
							break
						}
						go func() {
							io.Copy(stdin, command.Stdin)
							stdin.Close()
						}()

						res := cmd.Run()
						command.Stdout.Close()
						command.Stderr.Close()
						returnResult := &CommandResponse{}
						if res != nil {
							if exiterr, ok := res.(*exec.ExitError); ok {
								returnResult.Status = exiterr.Sys().(syscall.WaitStatus).ExitStatus()
							} else {
								log.Print(res)
								returnResult.Status = 10
							}
						}

						err = command.StatusChan.Send(returnResult)
						if err != nil {
							log.Print(err)
						}
					}
				}()
			}
		}()
	}
}
