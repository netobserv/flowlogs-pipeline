// Package proxy normalizes proxy access logs independently from flow records.
package proxy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
)

const (
	HAProxyPrefix = "netobserv-proxy-v1 "
	RecordType    = "proxyRequest"
	AppLabel      = "netobserv-proxy"
)

var (
	ErrUnsupported = errors.New("not a supported HAProxy correlation log")
	ErrNoBackend   = errors.New("no established backend connection")
	ErrIncomplete  = errors.New("incomplete request timing")
	syslogHeader   = regexp.MustCompile(`^<[0-9]{1,3}>[^\r\n]*haproxy\[[0-9]+\]: $`)
	instanceName   = regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,253}$`)
)

type haproxyRecord struct {
	ClientAddr       string `json:"client_addr"`
	ClientPort       string `json:"client_port"`
	FrontendAddr     string `json:"frontend_addr"`
	FrontendPort     string `json:"frontend_port"`
	BackendAddr      string `json:"backend_addr"`
	BackendPort      string `json:"backend_port"`
	ServerAddr       string `json:"server_addr"`
	ServerPort       string `json:"server_port"`
	RequestTime      string `json:"request_time"`
	RequestTimeLocal string `json:"request_time_local"`
	Duration         string `json:"duration_ms"`
	Instance         string `json:"instance"`
	Process          string `json:"process"`
	RequestID        string `json:"request_id"`
}

// DecodeHAProxy accepts a versioned payload, optionally wrapped in HAProxy's
// RFC3164 syslog envelope. It never interprets an arbitrary default access log.
func DecodeHAProxy(data []byte, received time.Time) (config.GenericMap, error) {
	line := string(bytes.TrimSpace(data))
	pos := strings.Index(line, HAProxyPrefix)
	if pos < 0 || (pos > 0 && !syslogHeader.MatchString(line[:pos])) {
		return nil, ErrUnsupported
	}
	var input haproxyRecord
	decoder := json.NewDecoder(strings.NewReader(line[pos+len(HAProxyPrefix):]))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return nil, fmt.Errorf("invalid proxy payload: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("unexpected trailing proxy payload")
	}
	if input.BackendAddr == "" || input.BackendAddr == "-" || input.BackendPort == "0" || input.ServerAddr == "" || input.ServerAddr == "-" {
		return nil, ErrNoBackend
	}
	out := config.GenericMap{"_RecordType": RecordType, "ProxyType": "haproxy", "ProxySchemaVersion": 1,
		"Proto": 6, "TimeReceived": received.Unix()}
	for _, endpoint := range []struct{ addr, port, key string }{
		{input.ClientAddr, input.ClientPort, "ProxyClient"},
		{input.FrontendAddr, input.FrontendPort, "ProxyFrontend"},
		{input.BackendAddr, input.BackendPort, "Src"},
		{input.ServerAddr, input.ServerPort, "Dst"},
	} {
		addr, err := netip.ParseAddr(endpoint.addr)
		if err != nil || addr.IsUnspecified() || addr.Zone() != "" {
			return nil, fmt.Errorf("invalid %s address", endpoint.key)
		}
		port, err := strconv.ParseUint(endpoint.port, 10, 16)
		if err != nil || port == 0 {
			return nil, fmt.Errorf("invalid %s port", endpoint.key)
		}
		out[endpoint.key+"Addr"] = addr.Unmap().String()
		out[endpoint.key+"Port"] = int(port)
	}
	startMs, endMs, err := requestInterval(&input)
	if err != nil {
		return nil, err
	}
	process, err := proxyProcess(&input)
	if err != nil {
		return nil, err
	}
	out["ProxyStartMs"], out["ProxyEndMs"] = startMs, endMs
	out["ProxyInstance"], out["ProxyProcess"] = input.Instance, process
	out["ProxyRequestID"] = fmt.Sprintf("%s/%s/%s/%d", input.Instance, input.Process, input.RequestID, startMs)
	return out, nil
}

func requestInterval(input *haproxyRecord) (int64, int64, error) {
	// %trg is UTC with an explicit offset but has second precision. %tr carries
	// the request's milliseconds (unlike %ms, which belongs to connection accept).
	start, err := time.Parse("02/Jan/2006:15:04:05 -0700", input.RequestTime)
	if err != nil || len(input.RequestTimeLocal) != len("02/Jan/2006:15:04:05.000") {
		return 0, 0, ErrIncomplete
	}
	local, err := time.Parse("02/Jan/2006:15:04:05.000", input.RequestTimeLocal)
	if err != nil {
		return 0, 0, ErrIncomplete
	}
	startMs := start.UnixMilli() + int64(local.Nanosecond()/int(time.Millisecond))
	duration, err := strconv.ParseInt(input.Duration, 10, 64)
	if err != nil || strings.HasPrefix(input.Duration, "+") || duration < 0 || duration > int64((24*time.Hour)/time.Millisecond) || startMs <= 0 {
		return 0, 0, ErrIncomplete
	}
	return startMs, startMs + duration, nil
}

func proxyProcess(input *haproxyRecord) (uint64, error) {
	if !instanceName.MatchString(input.Instance) {
		return 0, errors.New("invalid proxy instance")
	}
	process, err := strconv.ParseUint(input.Process, 10, 32)
	if err != nil || process == 0 {
		return 0, errors.New("invalid proxy process")
	}
	if _, err := strconv.ParseUint(input.RequestID, 10, 64); err != nil {
		return 0, errors.New("invalid proxy request ID")
	}
	return process, nil
}
