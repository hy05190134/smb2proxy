// TcpProxy project main.go
package main

import (
	"./common"
	"time"
	//"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
)

func main() {
	common.SetLogger(common.NewConsoleLogger(common.LogLevelTrace))

	if len(os.Args) < 4 {
		common.Log.Debug("missing message!")
		return
	}
	ip := os.Args[1]
	port, err := strconv.Atoi(os.Args[2])
	if err != nil {
		common.Log.Debug("error happened ,exit")
		return
	}
	addr := os.Args[3]

	Service(ip, port, addr)
}

func Service(ip string, port int, dstaddr string) {
	// listen and accept
	listen, err := net.ListenTCP("tcp", &net.TCPAddr{net.ParseIP(ip), port, ""})
	if err != nil {
		common.Log.Debug("listen error: %s", err)
		return
	}
	common.Log.Trace("init done...")

	for {
		client, err := listen.AcceptTCP()
		if err != nil {
			common.Log.Debug("accept error: %s", err)
			continue
		}

		go Channal(client, dstaddr)
	}
}

func Channal(client *net.TCPConn, addr string) {
	tcpAddr, _ := net.ResolveTCPAddr("tcp4", addr)
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		common.Log.Debug("connection error: %s", err)
		client.Close()
		return
	}

	go ReadRequest(client, conn)
	ReadResponse(conn, client)
	common.Log.Debug("end of proxy")
}

func ReadRequest(lconn *net.TCPConn, rconn *net.TCPConn) {
	for {
		//read first 4 bytes
		buf := make([]byte, 1024)

		lconn.SetReadDeadline(time.Now().Add(time.Second * 10))
		n, err := lconn.Read(buf)
		if err != nil {
			common.Log.Debug("Read request buf error: %s", err)
			break
		}

		common.Log.Trace("read request %d success", n)

		//TODO: hook func, modify Host header in HTTP
		//fmt.Println(string(buf[:n])), proxy close will throw err, server close will not
		if _, err := rconn.Write(buf[:n]); err != nil {
			common.Log.Debug("send request buf error: %s", err)
			break
		}
	}
	lconn.Close()
}

func ReadResponse(lconn *net.TCPConn, rconn *net.TCPConn) {
	for {
		buf := make([]byte, 1024)

		lconn.SetReadDeadline(time.Now().Add(time.Second * 10))
		n, err := lconn.Read(buf)
		if err != nil {
			common.Log.Debug("Read response buf error: %s", err)
			break
		}

		common.Log.Trace("read response %d success", n)

		//fmt.Println(string(buf[:n])), proxy close will throw err, client close will not
		if _, err := rconn.Write(buf[:n]); err != nil {
			common.Log.Debug("send response buf error: %s", err)
			break
		}
	}
	lconn.Close()
}

//hook func, change "Host: 127.0.0.1" => "Host: www.baidu.com"
func changeHost(request string, newhost string) string {
	reg := regexp.MustCompile(`Host[^\r\n]+`)
	return reg.ReplaceAllString(request, newhost)
}
