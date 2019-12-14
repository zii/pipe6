package httpx

import (
	"bufio"
	"net"
	"net/http"
	"strconv"
	"strings"
)

func Host2Addr(hostport string, defport int) (string, error) {
	if hostport == "" {
		return "", nil
	}
	if strings.LastIndex(hostport, ":") < 0 {
		return hostport + ":" + strconv.Itoa(defport), nil
	}
	host, port, err := net.SplitHostPort(hostport)
	if host == "" && err != nil {
		return "", err
	}
	if port != "" {
		return hostport, nil
	}
	return host + ":" + strconv.Itoa(defport), nil
}

// RemoveHopByHopHeaders remove hop by hop headers in http header list.
func RemoveHopByHopHeaders(header http.Header) {
	// Strip hop-by-hop header based on RFC:
	// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html#sec13.5.1
	// https://www.mnot.net/blog/2011/07/11/what_proxies_must_do

	header.Del("Proxy-Connection")
	header.Del("Proxy-Authenticate")
	header.Del("Proxy-Authorization")
	header.Del("TE")
	header.Del("Trailers")
	header.Del("Transfer-Encoding")
	header.Del("Upgrade")

	//connections := header.Get("Connection")
	//header.Del("Connection")
	//if connections == "" {
	//	return
	//}
	//for _, h := range strings.Split(connections, ",") {
	//	header.Del(strings.TrimSpace(h))
	//}
}

// return if want to keep-alive
func Transfer(req *http.Request, src, stream net.Conn, keepAlive bool) bool {
	RemoveHopByHopHeaders(req.Header)
	// Prevent UA from being set to golang's default ones
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "")
	}
	//req.Header.Set("Connection", "close")
	req.Write(stream)
	var out bool
	rspReader := bufio.NewReaderSize(stream, 8192)
	rsp, err := http.ReadResponse(rspReader, req)
	if err != nil {
		rsp = &http.Response{
			Status:        "Service Unavailable",
			StatusCode:    503,
			Proto:         "HTTP/1.1",
			ProtoMajor:    1,
			ProtoMinor:    1,
			Header:        http.Header(make(map[string][]string)),
			Body:          nil,
			ContentLength: 0,
			Close:         true,
		}
		rsp.Header.Set("Connection", "close")
		rsp.Header.Set("Proxy-Connection", "close")
	} else {
		RemoveHopByHopHeaders(rsp.Header)
		if rsp.ContentLength >= 0 {
			rsp.Header.Set("Proxy-Connection", "keep-alive")
			rsp.Header.Set("Connection", "keep-alive")
			rsp.Header.Set("Keep-Alive", "timeout=60")
			rsp.Close = false
			out = true
		} else {
			rsp.Close = true
		}
	}
	if err := rsp.Write(src); err != nil {
		return false
	}
	return out
}
