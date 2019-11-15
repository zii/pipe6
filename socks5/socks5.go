package socks5

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
)

var std = log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lshortfile)

func debug(a ...interface{}) {
	std.Output(2, fmt.Sprintln(a...))
}

type HandshakeResult struct {
	Network string // tcp/udp
	Domain  string
	Port    uint16
}

func (h *HandshakeResult) String() string {
	return fmt.Sprintf("%s://%s:%d", h.Network, h.Domain, h.Port)
}

func (h *HandshakeResult) Address() string {
	return fmt.Sprintf("%s:%d", h.Domain, h.Port)
}

// REPLY:
//+-----+-----+-------+------+----------+----------+
//| VER | REP |  RSV  | ATYP | BND.ADDR | BND.PORT |
//+-----+-----+-------+------+----------+----------+
//|  1  |  1  | X'00' |  1   | Variable |    2     |
//+-----+-----+-------+------+----------+----------+
//REP: 回复请求的状态
//X'00' 成功代理
//X'01' SOCKS服务器出现了错误
//X'02' 不允许的连接
//X'03' 找不到网络
//X'04' 找不到主机
//X'05' 连接被拒
//X'06' TTL超时
//X'07' 不支持的CMD
//X'08' 不支持的ATYP
//X'09' to X'FF' Socks5标准中没有分配对应的状态
type Replys struct {
	ver  int
	rep  int
	rsv  byte
	atyp int
	addr []byte
	port uint16
}

func (r *Replys) Encode() []byte {
	var n = 6 + len(r.addr)
	if r.atyp == 3 {
		n++
	}
	w := bytes.NewBuffer(nil)
	w.Write([]byte{byte(r.ver), byte(r.rep), r.rsv, byte(r.atyp)})
	if r.atyp == 3 {
		w.Write([]byte{byte(len(r.addr))})
	}
	w.Write(r.addr)
	binary.Write(w, binary.BigEndian, uint16(r.port))
	return w.Bytes()
}

// https://abersheeran.com/articles/Socks5/
// 返回握手结果, nil应立即断开连接
func Handshake(conn net.Conn) *HandshakeResult {
	// 1. 协商认证
	//+-----+----------+----------+
	//| VER | NMETHODS | METHODS  |
	//+-----+----------+----------+
	//|  1  |    1     | 1 to 255 |
	//+-----+----------+----------+
	// REPLY:
	//+-----+--------+
	//| VER | STATUS |
	//+-----+--------+
	//|  1  |   1    |
	//+-----+--------+
	{
		var b1 [1]byte
		_, err := io.ReadFull(conn, b1[:])
		if err != nil {
			return nil
		}
		debug("#1 VER:", b1)
		if b1[0] != 5 {
			return nil
		}
		_, err = io.ReadFull(conn, b1[:])
		if err != nil {
			return nil
		}
		debug("#1 NMETHODS:", b1)
		nmethods := b1[0]
		if nmethods == 0 {
			return nil
		}
		var bufn = make([]byte, nmethods)
		_, err = io.ReadFull(conn, bufn)
		if err != nil {
			return nil
		}
		debug("#2 METHODS:", bufn)
		var b2 = [2]byte{5, 0}
		_, err = conn.Write(b2[:])
		if err != nil {
			return nil
		}
	}
	var out = &HandshakeResult{}
	// 3. 请求代理
	//+-----+-----+-------+------+----------+----------+
	//| VER | CMD |  RSV  | ATYP | DST.ADDR | DST.PORT |
	//+-----+-----+-------+------+----------+----------+
	//|  1  |  1  | X'00' |  1   | Variable |    2     |
	//+-----+-----+-------+------+----------+----------+
	// REPLY:
	//+-----+-----+-------+------+----------+----------+
	//| VER | REP |  RSV  | ATYP | BND.ADDR | BND.PORT |
	//+-----+-----+-------+------+----------+----------+
	//|  1  |  1  | X'00' |  1   | Variable |    2     |
	//+-----+-----+-------+------+----------+----------+
	{
		var b1 [1]byte
		_, err := io.ReadFull(conn, b1[:])
		if err != nil {
			return nil
		}
		debug("#3 VER:", b1)
		if b1[0] != 5 {
			return nil
		}
		_, err = io.ReadFull(conn, b1[:])
		if err != nil {
			return nil
		}
		debug("#3 CMD:", b1)
		cmd := b1[0]
		if cmd != 1 {
			debug("#3 unsupport CMD:", cmd)
		}
		out.Network = "tcp"
		var b2 [2]byte
		_, err = io.ReadFull(conn, b2[:])
		if err != nil {
			return nil
		}
		debug("#3 RSV ATYP:", b2)
		atyp := b2[1]
		var baddr []byte
		// IPV4地址: X'01'
		// 域名: X'03'
		// IPV6地址: X'04'
		if atyp == 1 {
			var b4 [4]byte
			_, err := io.ReadFull(conn, b4[:])
			if err != nil {
				return nil
			}
			baddr = b4[:]
			ip := net.IPv4(b4[0], b4[1], b4[2], b4[3])
			out.Domain = ip.String()
		} else if atyp == 3 {
			_, err = io.ReadFull(conn, b1[:])
			if err != nil {
				return nil
			}
			n := b1[0]
			var buf = make([]byte, n)
			_, err = io.ReadFull(conn, buf)
			if err != nil {
				return nil
			}
			baddr = buf
			out.Domain = string(buf)
		} else {
			debug("#3 unsupport ATYP:", atyp)
			return nil
		}
		// PORT
		_, err = io.ReadFull(conn, b2[:])
		if err != nil {
			return nil
		}
		port := binary.BigEndian.Uint16(b2[:])
		out.Port = port
		debug("#3 address is:", out.String())
		// REPLY
		reply := &Replys{
			ver:  5,
			rep:  0,
			atyp: int(atyp),
			addr: baddr,
			port: out.Port,
		}
		_, err = conn.Write(reply.Encode())
		if err != nil {
			return nil
		}
		debug("#3 handshake success!")
	}
	return out
}
