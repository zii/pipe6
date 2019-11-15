package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"pipe6/base"
	"pipe6/proto"
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

func handleConnection(conn net.Conn) {
	log.Println("new connection:", conn.RemoteAddr())
	defer func() {
		conn.Close()
		log.Println("pipe closed:", conn.RemoteAddr())
	}()
	// read hello from local
	hello := &proto.Hello{}
	ok := hello.Decode(conn)
	if !ok {
		return
	}
	log.Println("hello success:", hello.NetworkString(), hello.Addr)
	// connection to dst addr
	dstAddr, err := net.ResolveTCPAddr("tcp", hello.Addr)
	if err != nil {
		log.Println("resolve dst err:", err)
		return
	}
	dst, err := net.DialTCP("tcp", nil, dstAddr)
	if err != nil {
		log.Println("dial dst err:", err)
		return
	}
	defer dst.Close()
	// upstream
	go func() {
		var buf = make([]byte, 8192)
		for {
			n, err := dst.Read(buf)
			if n > 0 {
				_, errw := conn.Write(buf[:n])
				if errw != nil {
					err = errw
				}
			}
			if err != nil {
				break
			}
		}
		dst.Close()
		conn.Close()
	}()
	// downstream
	var buf = make([]byte, 8192)
	for {
		n, err := conn.Read(buf)
		// read may return EOF with n > 0
		// should always process n > 0 bytes before handling error
		if n > 0 {
			_, errw := dst.Write(buf[:n])
			if errw != nil {
				err = errw
			}
		}
		if err != nil {
			break
		}
	}
}
