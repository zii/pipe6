package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"

	"github.com/zii/pipe6/mux"

	"github.com/hashicorp/yamux"

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
	var err error
	cert, err := tls.LoadX509KeyPair("./remote.pem", "./remote.key")
	base.Raise(err)

	localPem, err := ioutil.ReadFile("./local.pem")
	base.Raise(err)

	cas := x509.NewCertPool()
	cas.AppendCertsFromPEM(localPem)

	laddr := fmt.Sprintf("0.0.0.0:%d", args.Port)
	config := &tls.Config{
		Certificates:             []tls.Certificate{cert},
		MinVersion:               tls.VersionTLS13,
		PreferServerCipherSuites: true,
		ClientAuth:               tls.RequireAndVerifyClientCert,
		ClientCAs:                cas,
	}
	ls, err := tls.Listen("tcp", laddr, config)

	base.Raise(err)
	defer ls.Close()
	log.Println("listening on", laddr)
	for {
		conn, err := ls.Accept()
		if err != nil {
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(master net.Conn) {
	log.Println("new connection:", master.RemoteAddr())
	session, err := yamux.Server(master, nil)
	base.Raise(err)
	defer func() {
		session.Close()
		log.Println("session closed.", master.RemoteAddr())
	}()
	for {
		stream, err := session.Accept()
		if err != nil {
			break
		}
		go handleStream(stream)
	}
}

// connect to dst addr
func dialDst(addr string) net.Conn {
	dstAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		log.Println("resolve dst err:", err)
		return nil
	}
	dst, err := net.DialTCP("tcp", nil, dstAddr)
	if err != nil {
		log.Println("dial dst err:", err)
		return nil
	}
	return dst
}

func handleStream(stream net.Conn) {
	defer func() {
		stream.Close()
		log.Println("stream closed.", stream.LocalAddr())
	}()
	hello := mux.DecodeHello(stream)
	if hello == nil {
		return
	}
	log.Println("hello:", hello.Addr)
	dst := dialDst(hello.Addr)
	if dst == nil {
		return
	}
	go io.Copy(stream, dst)
	io.Copy(dst, stream)
}
