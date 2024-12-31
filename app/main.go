package main

import (
	"fmt"
	"net"
	"strings"
)

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

type DNSQuestion struct {
	QNAME  []byte
	QTYPE  uint16
	QCLASS uint16
}

type DNSMessage struct {
	Header    []byte
	Questions []DNSQuestion
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

		// Test response
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
			QDCOUNT: 1,
			ANCOUNT: 0,
			NSCOUNT: 0,
			ARCOUNT: 0,
		}

		question := DNSQuestion{
			QNAME:  domainNameEncoding("google.com"),
			QTYPE:  1,
			QCLASS: 1,
		}

		dnsMessage := DNSMessage{
			Header:    encodeDNSHeader(header),
			Questions: []DNSQuestion{question},
		}

		response := append(dnsMessage.Header, encodeDNSQuestion(dnsMessage.Questions[0])...)

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

func encodeDNSQuestion(question DNSQuestion) []byte {
	buffer := make([]byte, 0)
	buffer = append(buffer, question.QNAME...)
	buffer = append(buffer, byte(question.QTYPE>>8), byte(question.QTYPE))
	buffer = append(buffer, byte(question.QCLASS>>8), byte(question.QCLASS))
	return buffer
}

func domainNameEncoding(domain string) []byte {
	// <length> <label> <length> <label> ...
	// google.com -> \x06google\x03com\x00 in hex (06 67 6f 6f 67 6c 65 03 63 6f 6d 00)

	labels := strings.Split(domain, ".")

	buffer := make([]byte, 0)

	for _, label := range labels {
		buffer = append(buffer, byte(len(label)))

		buffer = append(buffer, []byte(label)...)
	}
	buffer = append(buffer, 0)

	return buffer
}
