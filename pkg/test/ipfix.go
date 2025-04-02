package test

import (
	"fmt"
	"net"
	"time"

	"github.com/vmware/go-ipfix/pkg/entities"
)

const (
	templateID          = 256
	observationDomainID = 1
)

// Values taken from https://www.iana.org/assignments/ipfix/ipfix.xhtml
var (
	sourceIPv4Address = entities.NewInfoElement("sourceIPv4Address", 8, entities.Ipv4Address, 0, 4)
	flowStartSeconds  = entities.NewInfoElement("flowStartSeconds", 150, entities.DateTimeSeconds, 0, 4)
	flowEndSeconds    = entities.NewInfoElement("flowEndSeconds", 151, entities.DateTimeSeconds, 0, 4)
)

// IPFIXClient for IPFIX tests
type IPFIXClient struct {
	conn net.Conn
}

// NewIPFIXClient returns an IPFIXClient that sends data to the given port
func NewIPFIXClient(port int) (*IPFIXClient, error) {
	conn, err := net.Dial("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, fmt.Errorf("can't open UDP connection on port %d :%w",
			port, err)
	}
	return &IPFIXClient{
		conn: conn,
	}, nil
}

// SendTemplate must be executed before sending any flow
func (ke *IPFIXClient) SendTemplate() error {
	// TODO: add more fields
	templateElements := []entities.InfoElementWithValue{
		entities.NewIPAddressInfoElement(sourceIPv4Address, nil),
		entities.NewDateTimeSecondsInfoElement(flowStartSeconds, 0),
		entities.NewDateTimeSecondsInfoElement(flowEndSeconds, 0),
	}
	set := entities.NewSet(false)
	if err := set.PrepareSet(entities.Template, templateID); err != nil {
		return err
	}
	if err := set.AddRecord(templateElements, templateID); err != nil {
		return nil
	}
	set.UpdateLenInHeader()
	return ke.sendMessage(set)
}

// SendFlow containing the information passed as an argument
func (ke *IPFIXClient) SendFlow(timestamp uint32, srcIP string) error {
	// TODO: add more fields
	templateElements := []entities.InfoElementWithValue{
		entities.NewIPAddressInfoElement(sourceIPv4Address, net.ParseIP(srcIP)),
		entities.NewDateTimeSecondsInfoElement(flowStartSeconds, timestamp),
		entities.NewDateTimeSecondsInfoElement(flowEndSeconds, timestamp),
	}
	set := entities.NewSet(false)
	if err := set.PrepareSet(entities.Data, templateID); err != nil {
		return err
	}
	if err := set.AddRecord(templateElements, templateID); err != nil {
		return nil
	}
	set.UpdateLenInHeader()
	return ke.sendMessage(set)
}

func (ke *IPFIXClient) sendMessage(set entities.Set) error {
	msg := entities.NewMessage(false)
	msg.SetVersion(10)
	msg.AddSet(set)
	msgLen := entities.MsgHeaderLength + set.GetSetLength()
	msg.SetVersion(10)
	msg.SetObsDomainID(observationDomainID)
	msg.SetMessageLen(uint16(msgLen))
	msg.SetExportTime(uint32(time.Now().Unix()))
	msg.SetSequenceNum(0)
	bytesSlice := make([]byte, msgLen)
	copy(bytesSlice[:entities.MsgHeaderLength], msg.GetMsgHeader())
	copy(bytesSlice[entities.MsgHeaderLength:entities.MsgHeaderLength+entities.SetHeaderLen], set.GetHeaderBuffer())
	index := entities.MsgHeaderLength + entities.SetHeaderLen
	for _, record := range set.GetRecords() {
		l := record.GetRecordLength()
		s, _ := record.GetBuffer()
		copy(bytesSlice[index:index+l], s)
		index += l
	}
	_, err := ke.conn.Write(bytesSlice)
	return err
}

// UDPPort asks the kernel for a free open port that is ready to use.
func UDPPort() (int, error) {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenUDP("udp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.LocalAddr().(*net.UDPAddr).Port, nil
}
