// socks5 server
package main

import (
	"crypto/tls"
	"flag"
	"io"
	"log"
	"net"
	"time"

	"github.com/zii/pipe6/mux"

	"github.com/zii/pipe6/base"
	"github.com/zii/pipe6/socks5"
)

var LocalCert tls.Certificate

var args = struct {
	RemoteAddr string
}{}

var workerPool *mux.WorkerPool

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
	workerPool = mux.NewWorkerPool(dialRemote)
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

func handleConnection(src net.Conn) {
	log.Println("new src connection:", src.RemoteAddr())
	defer func() {
		src.Close()
	}()
	//src.(*net.TCPConn).SetKeepAlive(true)
	src.(*net.TCPConn).SetNoDelay(true)
	// socks5 handshake
	result := socks5.Handshake(src)
	if result == nil {
		return
	}
	// alloc worker and create a new session to pipe between remote and src
	session := workerPool.GetWorker()
	if session == nil {
		return
	}
	log.Println("workers:", workerPool.Size())
	stream, err := session.Open()
	if err != nil {
		return
	}
	defer stream.Close()
	// send hello
	hello := mux.EncodeHello(1, result.Address())
	_, err = stream.Write(hello)
	if err != nil {
		return
	}
	go io.Copy(src, stream)
	io.Copy(stream, src)
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
