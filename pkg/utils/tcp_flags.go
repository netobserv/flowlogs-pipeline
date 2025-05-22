package utils

type tcpFlag struct {
	value16 uint16
	value32 uint32
	name    string
}

var tcpFlags = []tcpFlag{
	{value16: 1, value32: 1, name: "FIN"},
	{value16: 2, value32: 2, name: "SYN"},
	{value16: 4, value32: 4, name: "RST"},
	{value16: 8, value32: 8, name: "PSH"},
	{value16: 16, value32: 16, name: "ACK"},
	{value16: 32, value32: 32, name: "URG"},
	{value16: 64, value32: 64, name: "ECE"},
	{value16: 128, value32: 128, name: "CWR"},
	{value16: 256, value32: 256, name: "SYN_ACK"},
	{value16: 512, value32: 512, name: "FIN_ACK"},
	{value16: 1024, value32: 1024, name: "RST_ACK"},
}

func DecodeTCPFlagsU16(bitfield uint16) []string {
	var values []string
	for _, flag := range tcpFlags {
		if bitfield&flag.value16 != 0 {
			values = append(values, flag.name)
		}
	}
	return values
}

func DecodeTCPFlagsU32(bitfield uint32) []string {
	var values []string
	for _, flag := range tcpFlags {
		if bitfield&flag.value32 != 0 {
			values = append(values, flag.name)
		}
	}
	return values
}
