// socks5 server
package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/zii/pipe6/httpx"

	"github.com/zii/pipe6/mux"

	"github.com/zii/pipe6/base"
	"github.com/zii/pipe6/proto"
	"github.com/zii/pipe6/socks5"
)

var LocalCert tls.Certificate

var args = struct {
	RemoteAddr string // remote server host:port
	Socks5Port int    // socks5 proxy port
	HttpPort   int    // http proxy port
}{}

var workerPool *WorkerPool

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	flag.StringVar(&args.RemoteAddr, "remote", "127.0.0.1:18443", "remote server address")
	flag.IntVar(&args.Socks5Port, "socks5", 3127, "socks5 proxy port")
	flag.IntVar(&args.HttpPort, "http", 3128, "http/https proxy port")
	mux.CloseDebugLog()
}

func Init() {
	flag.Parse()
	log.Println("remote addr:", args.RemoteAddr)
	cert, err := tls.LoadX509KeyPair("local.pem", "local.key")
	base.Raise(err)
	LocalCert = cert
	workerPool = NewWorkerPool(dialRemote)
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

func handleSocks5(src net.Conn) {
	log.Println("new socks5 client:", src.RemoteAddr())
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
	hello := proto.EncodeHello(1, result.Address())
	_, err = stream.Write(hello)
	if err != nil {
		return
	}
	go io.Copy(src, stream)
	io.Copy(stream, src)
}

func handleHttp(src net.Conn) {
	log.Println("new http client:", src.RemoteAddr())
	defer func() {
		src.Close()
	}()
	src.(*net.TCPConn).SetNoDelay(true)

	reader := bufio.NewReaderSize(src, 8192)
	req, err := http.ReadRequest(reader)
	if err != nil {
		return
	}
	defport := 80
	https := req.Method == "CONNECT"
	if https {
		defport = 443
	}
	host := req.URL.Host
	destAddr, err := httpx.Host2Addr(host, defport)
	if err != nil {
		return
	}
	log.Println("dst:", req.URL)

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
	hello := proto.EncodeHello(1, destAddr)
	_, err = stream.Write(hello)
	if err != nil {
		return
	}

	if https {
		_, err := src.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
		if err != nil {
			return
		}
		go io.Copy(src, stream)
		io.Copy(stream, reader)
	} else {
		for {
			keepAlive := strings.TrimSpace(strings.ToLower(req.Header.Get("Proxy-Connection"))) == "keep-alive"
			ok := httpx.Transfer(req, src, stream, keepAlive)
			if !(ok && keepAlive) {
				break
			}
			req, err = http.ReadRequest(reader)
			if err != nil {
				break
			}
		}
	}
}

func main() {
	Init()

	// socks5 proxy server
	go func() {
		addr := fmt.Sprintf("0.0.0.0:%d", args.Socks5Port)
		ls, err := net.Listen("tcp", addr)
		base.Raise(err)
		log.Println("listening socks5 on", addr)
		for {
			conn, err := ls.Accept()
			if err != nil {
				continue
			}
			go handleSocks5(conn)
		}
	}()

	// http proxy server
	go func() {
		addr := fmt.Sprintf("0.0.0.0:%d", args.HttpPort)
		ls, err := net.Listen("tcp", addr)
		base.Raise(err)
		log.Println("listening http on", addr)
		for {
			conn, err := ls.Accept()
			if err != nil {
				continue
			}
			go handleHttp(conn)
		}
	}()

	// Interrupt handler.
	errc := make(chan error)
	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errc <- fmt.Errorf("%s", <-c)
	}()

	// Run!
	log.Println("exit", <-errc)
}
