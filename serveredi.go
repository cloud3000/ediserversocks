package serveredi

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
)

// Edistatus holds status and error info.
type Edistatus struct {
	Op      string
	Number  int
	Message string
	Len     int16
}

// Connect : Accept Connection from EDI client.
func Connect() (net.Conn, Edistatus) {
	var retval Edistatus
	retval.Len = 0
	reply := make([]byte, 1024)
	socket := os.NewFile(uintptr(3), "socket")
	conn, neterr := net.FileConn(socket)
	if neterr != nil {
		fmt.Printf("FileConn Error: %s\n", neterr.Error())
		return nil, retval
	}

	// Read security string from client.
	_, err := conn.Read(reply)
	if err != nil {
		retval.Op = "serveredi.Connect Read security"
		retval.Number = 1
		retval.Message = fmt.Sprintf("Connect failed: %s", err.Error())
		return nil, retval
	}
	sendAck(conn)

	// Send initial PASS/FAIL to client.
	passfail := fmt.Sprintf("PASSED%08d", os.Getpid())
	_, err = conn.Write([]byte(passfail))
	if err != nil {
		retval.Op = "serveredi.Connect Write security PASS/FAIL"
		retval.Number = 1
		retval.Message = fmt.Sprintf("Connect failed: %s", err.Error())
		return nil, retval
	}
	recvAck(conn)
	return conn, retval
}

// InLength Get Length of data to be received, before Recv
func inLength(c net.Conn) int16 {
	lenbuf := new(bytes.Buffer)
	netlen := int16(0)
	binary.Write(lenbuf, binary.BigEndian, uint16(netlen))
	_, err := c.Read(lenbuf.Bytes())
	if err != nil {
		println("InLength Read failed:", err.Error())
		return 1
	}
	binary.Read(lenbuf, binary.BigEndian, &netlen)
	return netlen
}
func sendAck(c net.Conn) int {
	strAck := "Y"
	_, err := c.Write([]byte(strAck))
	if err != nil {
		println("sendAck Write ACK to server failed:", err.Error())
		return 1
	}
	return 0
}

// Recv Receive data from HP3000
func Recv(c net.Conn) (string, Edistatus) {
	EDIEOF := int16(-9999)
	var retval Edistatus
	retval.Len = 0
	len := inLength(c)
	if len == EDIEOF {
		//fmt.Printf("EOF encountered\n")
		return "EOF", retval
	}
	reply := make([]byte, 4096)
	buffer := new(bytes.Buffer)
	buffer.Grow(int(len))
	var bufcnt = 0
	for index := 0; index < int(len); {
		reply = make([]byte, 4096)
		len, err := c.Read(reply)
		if err != nil {
			retval.Op = "serveredi.Recv"
			retval.Number = 1
			retval.Message = fmt.Sprintf("Recv failed: %s", err.Error())
			return "", retval
		}
		index = index + len
		buffer.WriteString(string(reply))
		bufcnt = index
	}
	sendAck(c)
	retval.Len = int16(bufcnt)
	return buffer.String(), retval
}

func recvAck(c net.Conn) int {
	reply := make([]byte, 1)
	_, err := c.Read(reply)
	if err != nil {
		println("recvAck Read ACK failed:", err.Error())
		return 2
	}
	if string(reply) == "Y" {
		return 0
	}
	return 1
}

func sendLength(c net.Conn, str string) int16 {
	lenbuf := new(bytes.Buffer)
	netlen := int16(len(str))
	//fmt.Printf("str=[%s]", str)
	//fmt.Printf("sendLength sending length=%d\n", len([]rune(str)))
	binary.Write(lenbuf, binary.BigEndian, uint16(netlen))
	_, err := c.Write(lenbuf.Bytes())
	if err != nil {
		println("sendLength Write length to server failed:", err.Error())
		return 0
	}
	return netlen
}

func sendData(c net.Conn, str string) int16 {
	cnt, err := c.Write([]byte(str))
	if err != nil {
		println("sendData Write server failed:", err.Error())
		return 0
	}
	return int16(cnt)
}

// Send data to the HP3000.
func Send(c net.Conn, str string) (int, Edistatus) {
	var retval Edistatus
	retval.Len = 0
	readyCount := sendLength(c, str)
	sentCount := sendData(c, str)
	if readyCount < sentCount {
		retval.Op = "serveredi.Send"
		retval.Number = 1
		retval.Message = fmt.Sprintf("Send failed , expected %d but sent %d\n", readyCount, sentCount)
		return 1, retval
	}
	acknowledgement := recvAck(c)
	retval.Len = sentCount
	return acknowledgement, retval
}

// Disconnect HP3000 socket
func Disconnect(c net.Conn) (int, Edistatus) {
	var retval Edistatus
	retval.Len = 0
	lenbuf := new(bytes.Buffer)
	netlen := int16(-9999)
	//println("Disconnect Write length -9999")
	binary.Write(lenbuf, binary.BigEndian, uint16(netlen))
	_, err := c.Write(lenbuf.Bytes())
	if err != nil {
		retval.Op = "serveredi.Disconnect"
		retval.Number = 1
		retval.Message = fmt.Sprintf("Disconnect failed, %s ", err.Error())
		return 1, retval
	}
	//println("Disconnect receiving data")
	Recv(c)
	//println("Disconnect sending final ACK")
	sendAck(c)
	c.Close()
	return 0, retval
}
