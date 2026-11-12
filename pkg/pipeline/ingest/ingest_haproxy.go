package ingest

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/operational"
	pUtils "github.com/netobserv/flowlogs-pipeline/pkg/pipeline/utils"
	"github.com/netobserv/flowlogs-pipeline/pkg/proxy"
	"github.com/sirupsen/logrus"
)

type proxyDatagram struct {
	data     []byte
	received time.Time
}

type HAProxy struct {
	conn    net.PacketConn
	input   chan proxyDatagram
	exit    <-chan struct{}
	metrics *metrics
	maxSize int
}

func NewHAProxy(opMetrics *operational.Metrics, params config.StageParam) (*HAProxy, error) {
	if params.Ingest == nil || params.Ingest.HAProxy == nil {
		return nil, errors.New("HAProxy input configuration is required")
	}
	cfg := *params.Ingest.HAProxy
	if cfg.Address == "" {
		cfg.Address = ":10514"
	}
	if cfg.BufferLen == 0 {
		cfg.BufferLen = 1000
	}
	if cfg.MaxMessageSize == 0 {
		cfg.MaxMessageSize = 2048
	}
	if cfg.BufferLen < 1 || cfg.BufferLen > 100000 || cfg.MaxMessageSize < 512 || cfg.MaxMessageSize > 65507 {
		return nil, errors.New("invalid HAProxy buffer length or message size")
	}
	conn, err := net.ListenPacket("udp", cfg.Address)
	if err != nil {
		return nil, fmt.Errorf("listen for HAProxy syslog: %w", err)
	}
	input := make(chan proxyDatagram, cfg.BufferLen)
	return &HAProxy{conn: conn, input: input, exit: pUtils.ExitChannel(), maxSize: cfg.MaxMessageSize,
		metrics: newMetrics(opMetrics, params.Name, params.Ingest.Type, func() int { return len(input) })}, nil
}

func (h *HAProxy) Ingest(out chan<- config.GenericMap) {
	defer h.conn.Close()
	h.metrics.createOutQueueLen(out)
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-h.exit:
			_ = h.conn.Close()
		case <-done:
		}
	}()
	go h.read()
	for {
		select {
		case <-h.exit:
			return
		case datagram, ok := <-h.input:
			if !ok {
				return
			}
			record, err := proxy.DecodeHAProxy(datagram.data, datagram.received)
			if err != nil {
				code := "invalid"
				switch {
				case errors.Is(err, proxy.ErrUnsupported):
					code = "unsupported"
				case errors.Is(err, proxy.ErrNoBackend):
					code = "no_backend"
				case errors.Is(err, proxy.ErrIncomplete):
					code = "incomplete"
				}
				h.metrics.error(code)
				continue
			}
			select {
			case out <- record:
				h.metrics.flowsProcessed.Inc()
			case <-h.exit:
				return
			}
		}
	}
}

func (h *HAProxy) read() {
	defer close(h.input)
	buffer := make([]byte, h.maxSize+1)
	for {
		n, _, err := h.conn.ReadFrom(buffer)
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				h.metrics.error("read")
				logrus.WithError(err).Error("HAProxy syslog receiver stopped")
			}
			return
		}
		if n > h.maxSize {
			h.metrics.error("oversized")
			continue
		}
		packet := proxyDatagram{data: append([]byte(nil), buffer[:n]...), received: time.Now()}
		select {
		case h.input <- packet:
		default:
			h.metrics.error("queue_full")
		}
	}
}
