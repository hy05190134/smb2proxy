// TcpProxy project main.go
package main

import (
	"bytes"
	"encoding/binary"
	//"fmt"
	"github.com/hy05190134/smb/common"
	//"github.com/hy05190134/smb/smb"
	//"github.com/hy05190134/smb/smb/encoder"
	"io"
	"net"
	"os"
	//"regexp"
	"strconv"
	"time"
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

type SmbMessage struct {
	length uint64
	data   []byte
}

type ShareData struct {
	RcvReqChan  chan SmbMessage
	SendReqChan chan SmbMessage

	RcvRspChan  chan SmbMessage
	SendRspChan chan SmbMessage
}

func Channal(client *net.TCPConn, addr string) {
	tcpAddr, _ := net.ResolveTCPAddr("tcp4", addr)
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		common.Log.Debug("connection error: %s", err)
		client.Close()
		return
	}

	//go ReadRequest(client, conn)
	//ReadResponse(conn, client)

	sh := ShareData{
		RcvReqChan:  make(chan SmbMessage, 30),
		SendReqChan: make(chan SmbMessage, 30),
		RcvRspChan:  make(chan SmbMessage, 30),
		SendRspChan: make(chan SmbMessage, 30),
	}

	go ReceiveRequest(client, sh)
	go SendRequest(conn, sh)

	//client->handle->conn; conn->handle->client
	go HandleMessage(sh)

	go ReceiveResponse(conn, sh)
	SendResponse(client, sh)

	common.Log.Debug("end of proxy")
}

func ReceiveRequest(lconn *net.TCPConn, sh ShareData) {
	for {
		buf := make([]byte, 1024)

		lconn.SetReadDeadline(time.Now().Add(time.Second * 10))
		n, err := io.ReadAtLeast(lconn, buf, 68)
		if err != nil {
			common.Log.Debug("Read request buf error: %s", err)
			break
		}

		common.Log.Trace("read request %d success", n)

		//read first 4 byte
		var messageLen uint32 = 0
		r := bytes.NewBuffer(buf)
		if err := binary.Read(r, binary.BigEndian, &messageLen); err != nil {
			common.Log.Debug("parse message len failed, err: %s", err)
			break
		} else {
			common.Log.Trace("message len: %d", messageLen)
		}

		if int(messageLen)+4 <= n {
			smbMsg := SmbMessage{
				length: uint64(n),
				data:   buf[:n],
			}

			sh.RcvReqChan <- smbMsg
		} else {
			remainingLen := int(messageLen) + 4 - n
			remainingBuf := make([]byte, messageLen+4)
			lconn.SetReadDeadline(time.Now().Add(time.Second * 10))
			num, err := io.ReadAtLeast(lconn, remainingBuf[n:], remainingLen)
			if err != nil {
				common.Log.Debug("Read request buf error: %s", err)
				break
			}

			for i := 0; i < n; i++ {
				remainingBuf[i] = buf[i]
			}

			smbMsg := SmbMessage{
				length: uint64(messageLen) + 4,
				data:   remainingBuf[:n+num],
			}

			sh.RcvReqChan <- smbMsg
		}
	}

	lconn.Close()
	close(sh.RcvReqChan)
	close(sh.SendReqChan)

	common.Log.Debug("exit receive request")
}

func SendRequest(rconn *net.TCPConn, sh ShareData) {
	for smbMsg := range sh.SendReqChan {
		if n, err := rconn.Write(smbMsg.data); err != nil {
			common.Log.Debug("send request buf error: %s", err)
			break
		} else {
			common.Log.Debug("send request buf: %d", n)
		}
	}

	common.Log.Debug("exit send request")
	return
}

func HandleMessage(sh ShareData) {
	for {
		select {
		case smbMsg, ok := <-sh.RcvReqChan:
			if ok {
				sh.SendReqChan <- smbMsg
			} else {
				common.Log.Debug("exit handle message")
				return
			}
		case smbMsg, ok := <-sh.RcvRspChan:
			if ok {
				sh.SendRspChan <- smbMsg
			} else {
				common.Log.Debug("exit handle message")
				return
			}
		}
	}
}

func ReceiveResponse(lconn *net.TCPConn, sh ShareData) {
	for {
		buf := make([]byte, 1024)

		lconn.SetReadDeadline(time.Now().Add(time.Second * 10))
		n, err := io.ReadAtLeast(lconn, buf, 68)
		if err != nil {
			common.Log.Debug("Read response buf error: %s", err)
			break
		}

		common.Log.Trace("read response %d success", n)

		//read first 4 byte
		var messageLen uint32 = 0
		r := bytes.NewBuffer(buf)
		if err := binary.Read(r, binary.BigEndian, &messageLen); err != nil {
			common.Log.Debug("parse message len failed, err: %s", err)
			break
		} else {
			common.Log.Trace("message len: %d", messageLen)
		}

		if int(messageLen)+4 <= n {
			smbMsg := SmbMessage{
				length: uint64(n),
				data:   buf[:n],
			}

			sh.RcvRspChan <- smbMsg
		} else {
			remainingLen := int(messageLen) + 4 - n
			remainingBuf := make([]byte, messageLen+4)
			lconn.SetReadDeadline(time.Now().Add(time.Second * 10))
			num, err := io.ReadAtLeast(lconn, remainingBuf[n:], remainingLen)
			if err != nil {
				common.Log.Debug("Read response buf error: %s", err)
				break
			}

			for i := 0; i < n; i++ {
				remainingBuf[i] = buf[i]
			}

			smbMsg := SmbMessage{
				length: uint64(messageLen) + 4,
				data:   remainingBuf[:n+num],
			}

			sh.RcvRspChan <- smbMsg
		}
	}

	lconn.Close()
	close(sh.RcvRspChan)
	close(sh.SendRspChan)
	common.Log.Debug("exit receive response")
}

func SendResponse(rconn *net.TCPConn, sh ShareData) {
	for smbMsg := range sh.SendRspChan {
		if n, err := rconn.Write(smbMsg.data); err != nil {
			common.Log.Debug("send response buf error: %s", err)
			break
		} else {
			common.Log.Debug("send response buf: %d", n)
		}
	}

	common.Log.Debug("exit send response")
	return
}
