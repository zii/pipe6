package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"

	"github.com/zii/pipe6/mux"

	"github.com/zii/pipe6/base"
)

var args = struct {
	Port int
}{}

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	flag.IntVar(&args.Port, "p", 18443, "listent port")
}

func main() {
	flag.Parse()
	cert, err := tls.LoadX509KeyPair("./remote.pem", "./remote.key")
	base.Raise(err)

	localPem, err := ioutil.ReadFile("./local.pem")
	base.Raise(err)

	cas := x509.NewCertPool()
	cas.AppendCertsFromPEM(localPem)

	config := &tls.Config{
		Certificates:             []tls.Certificate{cert},
		MinVersion:               tls.VersionTLS13,
		PreferServerCipherSuites: true,
		ClientAuth:               tls.RequireAndVerifyClientCert,
		ClientCAs:                cas,
	}
	laddr := fmt.Sprintf("0.0.0.0:%d", args.Port)
	ls, err := tls.Listen("tcp", laddr, config)
	base.Raise(err)
	defer ls.Close()
	log.Println("listening on", laddr)
	func() {
		for {
			conn, err := ls.Accept()
			if err != nil {
				continue
			}
			go handleConnection(conn)
		}
	}()
}

func handleConnection(master net.Conn) {
	log.Println("new connection:", master.RemoteAddr())
	sm := mux.NewSessionManager(master)
	sm.RunOnRemote()
}
