// socks5 server
package main

import (
	"crypto/tls"
	"flag"
	"io"
	"log"
	"net"
	"pipe6/base"
	"pipe6/proto"
	"pipe6/socks5"
	"time"
)

var LocalCert tls.Certificate

var args = struct {
	RemoteAddr string
}{}

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	flag.StringVar(&args.RemoteAddr, "remote", "127.0.0.1:18443", "remote server address")
}

func Init() {
	flag.Parse()
	log.Println("remote addr:", args.RemoteAddr)
	cert, err := tls.LoadX509KeyPair("local.pem", "local.key")
	base.Raise(err)
	LocalCert = cert
}

func dialRemote() (net.Conn, error) {
	st := time.Now()

	config := &tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{LocalCert},
	}

	conn, err := tls.Dial("tcp", args.RemoteAddr, config)
	if err != nil {
		return nil, err
	}
	log.Println("dial remote took:", time.Since(st))
	return conn, nil
}

func handleConnection(local net.Conn) {
	log.Println("new connection:", local.RemoteAddr())
	defer func() {
		local.Close()
		log.Println("pipe closed:", local.RemoteAddr())
	}()
	local.(*net.TCPConn).SetKeepAlive(true)
	// socks5 handshake
	result := socks5.Handshake(local)
	if result == nil {
		return
	}
	remote, err := dialRemote()
	if err != nil {
		log.Println("dial remote err:", err)
		return
	}
	defer remote.Close()
	// send hello to remote
	hello := &proto.Hello{
		Network: 1,
		Addr:    result.Address(),
	}
	b := hello.Encode()
	_, err = remote.Write(b)
	if err != nil {
		return
	}

	// downstream
	go func() {
		var buf = make([]byte, 8192)
		n, _ := io.CopyBuffer(local, remote, buf)
		log.Println("total read:", n)
		local.Close()
		remote.Close()
	}()
	// upstream
	{
		var buf = make([]byte, 8192)
		n, _ := io.CopyBuffer(remote, local, buf)
		log.Println("total write:", n)
	}
}

func main() {
	Init()
	ls, err := net.Listen("tcp", "0.0.0.0:8087")
	base.Raise(err)
	log.Println("listening..")
	for {
		conn, err := ls.Accept()
		if err != nil {
			continue
		}
		go handleConnection(conn)
	}
}
