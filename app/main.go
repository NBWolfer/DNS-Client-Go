package main

import (
	"fmt"
	"net"
)

// Ensures gofmt doesn't remove the "net" import in stage 1 (feel free to remove this!)
var _ = net.ListenUDP

type DNSHeader struct {
	ID      uint16
	QR      bool
	OPCODE  uint8
	AA      bool
	TC      bool
	RD      bool
	RA      bool
	Z       uint8
	RCODE   uint8
	QDCOUNT uint16
	ANCOUNT uint16
	NSCOUNT uint16
	ARCOUNT uint16
}

type DNSMessage struct {
	Header DNSHeader
}

func main() {
	fmt.Println("Logs from your program will appear here!")

	udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:2053")
	if err != nil {
		fmt.Println("Failed to resolve UDP address:", err)
		return
	}

	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		fmt.Println("Failed to bind to address:", err)
		return
	}
	defer udpConn.Close()

	buf := make([]byte, 512)

	for {
		size, source, err := udpConn.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("Error receiving data:", err)
			break
		}

		receivedData := string(buf[:size])
		fmt.Printf("Received %d bytes from %s: %s\n", size, source, receivedData)

		header := DNSHeader{
			ID:      1234,
			QR:      true,
			OPCODE:  0,
			AA:      false,
			TC:      false,
			RD:      false,
			RA:      false,
			Z:       0,
			RCODE:   0,
			QDCOUNT: 0,
			ANCOUNT: 0,
			NSCOUNT: 0,
			ARCOUNT: 0,
		}

		// Create an empty response
		response := encodeDNSHeader(header)
		fmt.Println("Response:", response)
		_, err = udpConn.WriteToUDP(response, source)
		if err != nil {
			fmt.Println("Failed to send response:", err)
		}
	}
}

func boolToByte(value bool) byte {
	if value {
		return 1
	}
	return 0
}

func encodeDNSHeader(header DNSHeader) []byte {
	buffer := make([]byte, 12)

	// Integers should be encoded in big-endian
	buffer[0] = byte(header.ID >> 8)
	buffer[1] = byte(header.ID)

	// Encode the flags
	flags := uint16(0)
	flags |= uint16(boolToByte(header.QR)) << 15
	flags |= uint16(header.OPCODE) << 11
	flags |= uint16(boolToByte(header.AA)) << 10
	flags |= uint16(boolToByte(header.TC)) << 9
	flags |= uint16(boolToByte(header.RD)) << 8
	flags |= uint16(boolToByte(header.RA)) << 7
	flags |= uint16(header.Z) << 4

	buffer[2] = byte(flags >> 8)
	buffer[3] = byte(flags)

	return buffer
}
