package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
)

func main() {
	server := "127.0.0.1:2053"

	buf := new(bytes.Buffer)
	questions := []string{"google.com", "example.com", "example.net"}

	// Header
	// ID = 0x1234, Flags = 0x0100 (QR=0, RD=1), QDCOUNT = len(questions)
	binary.Write(buf, binary.BigEndian, uint16(0x1234))         // ID
	binary.Write(buf, binary.BigEndian, uint16(0x0100))         // Flags
	binary.Write(buf, binary.BigEndian, uint16(len(questions))) // QDCOUNT
	binary.Write(buf, binary.BigEndian, uint16(0))              // ANCOUNT
	binary.Write(buf, binary.BigEndian, uint16(0))              // NSCOUNT
	binary.Write(buf, binary.BigEndian, uint16(0))              // ARCOUNT

	// Questions
	for _, q := range questions {
		encodedQ := domainNameEncoding(q)
		// fmt.Println("Encoded query:", encodedQ)
		buf.Write(encodedQ)
		binary.Write(buf, binary.BigEndian, uint16(1)) // QTYPE A
		binary.Write(buf, binary.BigEndian, uint16(1)) // QCLASS IN
	}

	fmt.Println("Sending query to", server)
	conn, err := net.Dial("udp", server)
	if err != nil {
		fmt.Println("Failed to connect:", err)
		return
	}
	defer conn.Close()

	_, err = conn.Write(buf.Bytes())
	if err != nil {
		fmt.Println("Failed to send:", err)
		return
	}

	resp := make([]byte, 512)
	n, err := conn.Read(resp)
	if err != nil {
		fmt.Println("Failed to receive:", err)
		return
	}
	n = n // Suppress warning

	// fmt.Printf("Response (%d bytes): %v\n", n, resp[:n])

	// Read header
	id := binary.BigEndian.Uint16(resp[0:2])
	qdCount := binary.BigEndian.Uint16(resp[4:6])
	anCount := binary.BigEndian.Uint16(resp[6:8])
	fmt.Printf("ID: %d, QDCOUNT: %d, ANCOUNT: %d\n", id, qdCount, anCount)

	offset := 12

	// Skip questions
	for i := 0; i < int(qdCount); i++ {
		_, nextOffset := decodeDomainName(resp, offset)
		offset = nextOffset + 4 // QTYPE(2) + QCLASS(2)
	}

	// Read answers
	for i := 0; i < int(anCount); i++ {
		name, nextOffset := decodeDomainName(resp, offset)
		rtype := binary.BigEndian.Uint16(resp[nextOffset : nextOffset+2])
		rclass := binary.BigEndian.Uint16(resp[nextOffset+2 : nextOffset+4])
		ttl := binary.BigEndian.Uint32(resp[nextOffset+4 : nextOffset+8])
		rdlength := binary.BigEndian.Uint16(resp[nextOffset+8 : nextOffset+10])
		rdata := resp[nextOffset+10 : nextOffset+10+int(rdlength)]

		fmt.Printf("Answer %d: NAME=%s, TYPE=%d, CLASS=%d, TTL=%d, RDLENGTH=%d, RDATA=%v\n",
			i+1, name, rtype, rclass, ttl, rdlength, rdata)

		offset = nextOffset + 10 + int(rdlength)
	}
}

func domainNameEncoding(domain string) []byte {
	labels := strings.Split(domain, ".")
	buffer := make([]byte, 0)
	for _, label := range labels {
		buffer = append(buffer, byte(len(label)))
		buffer = append(buffer, []byte(label)...)
	}
	buffer = append(buffer, 0)
	return buffer
}

func decodeDomainName(buf []byte, offset int) ([]byte, int) {
	originalOffset := offset
	jumped := false
	name := make([]byte, 0)

	for {
		b := buf[offset]
		if b == 0 {
			offset++
			break
		}
		// 0xC0 -> pointer compression
		if (b & 0xC0) == 0xC0 {
			pointer := int(binary.BigEndian.Uint16(buf[offset:offset+2]) & 0x3FFF)
			offset += 2
			if !jumped {
				originalOffset = offset
			}
			jumped = true
			seg, _ := decodeDomainName(buf, pointer)
			if len(name) > 0 {
				name = append(name, '.')
			}
			name = append(name, seg...)
			break
		} else {
			length := int(b)
			offset++
			if len(name) > 0 {
				name = append(name, '.')
			}
			name = append(name, buf[offset:offset+length]...)
			offset += length
		}
	}

	if jumped {
		return name, originalOffset
	}
	return name, offset
}
