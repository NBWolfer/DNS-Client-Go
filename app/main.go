package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
)

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
	QNAME  string // ASCII olarak tutuyoruz (örn: "google.com")
	QTYPE  uint16
	QCLASS uint16
}

// Sunucu tarafında ANSWER’ı da “ham wire format” domain şeklinde encode edeceğiz.
type DNSAnswer struct {
	NAME     []byte // Wire formatta (örn: [6 g o o g l e 3 c o m 0])
	TYPE     uint16
	CLASS    uint16
	TTL      uint32
	RDLENGTH uint16
	RDATA    []byte
}

func main() {
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

	fmt.Println("DNS Server listening on", udpAddr)

	buf := make([]byte, 512)

	for {
		size, source, err := udpConn.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("Error receiving data:", err)
			break
		}

		if size < 12 {
			continue
		}

		testerId := binary.BigEndian.Uint16(buf[0:2])
		flags := binary.BigEndian.Uint16(buf[2:4])
		qdCount := binary.BigEndian.Uint16(buf[4:6])

		qr := (flags & 0x8000) != 0
		opcode := uint8((flags >> 11) & 0x0F)
		rd := (flags & 0x0100) != 0

		// Sunucu cevabında:
		// - QR = 1 (cevap paketi)
		// - OPCODE = aynısı
		// - RD = aynısı
		// - RCODE = 0 (hata yok) (eğer opcode=0 ise)
		rcodeValue := uint8(0)
		if opcode != 0 {
			// Sadece opcode=0 (standard query) destekliyoruz
			rcodeValue = 4 // Not Implemented
		}

		// DNS header inşa edelim
		dnsHeader := DNSHeader{
			ID:      testerId,
			QR:      qr,
			OPCODE:  opcode,
			AA:      false,
			TC:      false,
			RD:      rd,
			RA:      false,
			Z:       0,
			RCODE:   rcodeValue,
			QDCOUNT: qdCount,
			ANCOUNT: 0,
			NSCOUNT: 0,
			ARCOUNT: 0,
		}

		offset := 12
		questions := make([]DNSQuestion, 0)

		for i := 0; i < int(qdCount); i++ {
			qname, nextOffset := decodeDomainNameToASCII(buf, offset)
			qtype := binary.BigEndian.Uint16(buf[nextOffset : nextOffset+2])
			qclass := binary.BigEndian.Uint16(buf[nextOffset+2 : nextOffset+4])

			questions = append(questions, DNSQuestion{
				QNAME:  qname,
				QTYPE:  qtype,
				QCLASS: qclass,
			})

			offset = nextOffset + 4
		}

		// Cevapları hazırlayalım
		answers := make([]DNSAnswer, 0)
		for _, q := range questions {
			// Yalnızca A kaydı (QTYPE=1) ve IN class (QCLASS=1) için cevap veriyoruz
			if q.QTYPE == 1 && q.QCLASS == 1 {
				wireDomain := domainNameEncoding(q.QNAME)
				ans := DNSAnswer{
					NAME:     wireDomain,
					TYPE:     1, // A kaydı
					CLASS:    1, // IN
					TTL:      60,
					RDLENGTH: 4,
					RDATA:    []byte{8, 8, 8, 8},
				}
				answers = append(answers, ans)
			}
		}

		dnsHeader.ANCOUNT = uint16(len(answers))

		response := make([]byte, 0)
		response = append(response, encodeDNSHeader(dnsHeader)...)

		// 2) Questions
		for _, q := range questions {
			wireDomain := domainNameEncoding(q.QNAME)
			response = append(response, wireDomain...)

			tmp := make([]byte, 2)
			binary.BigEndian.PutUint16(tmp, q.QTYPE)
			response = append(response, tmp...)

			binary.BigEndian.PutUint16(tmp, q.QCLASS)
			response = append(response, tmp...)
		}

		// 3) Answers
		for _, ans := range answers {
			response = append(response, encodeDNSAnswer(ans)...)
		}

		_, err = udpConn.WriteToUDP(response, source)
		if err != nil {
			fmt.Println("Failed to send response:", err)
		}
	}
}

func decodeDomainNameToASCII(buf []byte, offset int) (string, int) {
	labels := []string{}
	for {
		length := int(buf[offset])
		if length == 0 {
			offset++
			break
		}
		// Label’ın uzunluğu kadar ilerle
		offset++
		label := buf[offset : offset+length]
		offset += length
		labels = append(labels, string(label))
	}
	// labels birleştir
	domain := strings.Join(labels, ".")
	return domain, offset
}

func domainNameEncoding(domain string) []byte {
	labels := strings.Split(domain, ".")
	var out []byte
	for _, label := range labels {
		out = append(out, byte(len(label)))
		out = append(out, label...)
	}
	out = append(out, 0)
	return out
}

func encodeDNSHeader(h DNSHeader) []byte {
	buf := make([]byte, 12)
	binary.BigEndian.PutUint16(buf[0:2], h.ID)

	// Flags (QR, OPCODE, RD vs.)
	var flags uint16
	if h.QR {
		flags |= 1 << 15
	}
	flags |= (uint16(h.OPCODE) & 0xF) << 11
	if h.AA {
		flags |= 1 << 10
	}
	if h.TC {
		flags |= 1 << 9
	}
	if h.RD {
		flags |= 1 << 8
	}
	if h.RA {
		flags |= 1 << 7
	}

	// RCODE
	flags |= uint16(h.RCODE) & 0xF

	binary.BigEndian.PutUint16(buf[2:4], flags)
	binary.BigEndian.PutUint16(buf[4:6], h.QDCOUNT)
	binary.BigEndian.PutUint16(buf[6:8], h.ANCOUNT)
	binary.BigEndian.PutUint16(buf[8:10], h.NSCOUNT)
	binary.BigEndian.PutUint16(buf[10:12], h.ARCOUNT)
	return buf
}

// encodeDNSAnswer: NAME, TYPE, CLASS, TTL, RDLENGTH, RDATA
func encodeDNSAnswer(ans DNSAnswer) []byte {
	var buf []byte

	buf = append(buf, ans.NAME...)

	tmp2 := make([]byte, 2)
	binary.BigEndian.PutUint16(tmp2, ans.TYPE)
	buf = append(buf, tmp2...)

	binary.BigEndian.PutUint16(tmp2, ans.CLASS)
	buf = append(buf, tmp2...)

	tmp4 := make([]byte, 4)
	binary.BigEndian.PutUint32(tmp4, ans.TTL)
	buf = append(buf, tmp4...)

	binary.BigEndian.PutUint16(tmp2, ans.RDLENGTH)
	buf = append(buf, tmp2...)

	buf = append(buf, ans.RDATA...)

	return buf
}
