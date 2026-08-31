/*
 * Copyright (C) 2026 NetObserv Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package flowbuffer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/schema"
	"github.com/sirupsen/logrus"
)

var qlog = logrus.WithField("component", "write.FlowBuffer.query")

// Warning codes returned in cluster query responses.
const (
	WarningPeerQueryFailed = "PEER_QUERY_FAILED"
)

// Warning is a machine-readable query warning for the console.
type Warning struct {
	Code    string `json:"code"`
	Peer    string `json:"peer,omitempty"`
	Message string `json:"message,omitempty"`
}

// ClusterQueryResult extends QueryResult with peer fan-in metadata.
type ClusterQueryResult struct {
	QueryResult
	Warnings     []Warning `json:"warnings,omitempty"`
	PeersQueried int       `json:"peersQueried"`
	PeersFailed  int       `json:"peersFailed"`
}

// PeerLister returns base URLs of sibling flowBuffer instances (excluding self).
type PeerLister interface {
	ListPeers(ctx context.Context) ([]string, error)
}

// StaticPeers is a PeerLister backed by an explicit URL list (tests / offline).
type StaticPeers struct {
	URLs []string
}

func (s StaticPeers) ListPeers(_ context.Context) ([]string, error) {
	out := make([]string, len(s.URLs))
	copy(out, s.URLs)
	return out, nil
}

// Server serves local and cluster flowBuffer query HTTP APIs.
type Server struct {
	ring         *Ring
	peers        PeerLister
	httpClient   *http.Client
	queryTimeout time.Duration
	selfAddr     string // used to skip self when peers include us
}

// NewServer builds a query server.
func NewServer(ring *Ring, peers PeerLister, timeout time.Duration, selfListenAddr string) *Server {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &Server{
		ring:         ring,
		peers:        peers,
		queryTimeout: timeout,
		selfAddr:     selfListenAddr,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Handler returns an http.Handler with:
//   - GET/POST /api/flowbuffer/local/flows  — this pod's buffer only (peer calls)
//   - GET/POST /api/flowbuffer/flows        — cluster fan-in (console)
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/flowbuffer/local/flows", s.handleLocal)
	mux.HandleFunc("/api/flowbuffer/flows", s.handleCluster)
	return mux
}

func (s *Server) handleLocal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	filter, err := parseQueryFilter(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, s.ring.Query(filter))
}

func (s *Server) handleCluster(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	filter, err := parseQueryFilter(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.queryTimeout)
	defer cancel()
	writeJSON(w, s.queryCluster(ctx, filter))
}

func (s *Server) queryCluster(ctx context.Context, filter QueryFilter) ClusterQueryResult {
	local := s.ring.Query(filter)
	merged := ClusterQueryResult{
		QueryResult:  local,
		PeersQueried: 1, // local
	}

	if s.peers == nil {
		return merged
	}
	peerURLs, err := s.peers.ListPeers(ctx)
	if err != nil {
		merged.Warnings = append(merged.Warnings, Warning{
			Code:    WarningPeerQueryFailed,
			Message: fmt.Sprintf("peer discovery failed: %v", err),
		})
		return merged
	}

	type peerResult struct {
		url    string
		result QueryResult
		err    error
	}
	ch := make(chan peerResult, len(peerURLs))
	var wg sync.WaitGroup
	for _, base := range peerURLs {
		base = strings.TrimRight(base, "/")
		if s.isSelf(base) {
			continue
		}
		wg.Add(1)
		go func(peerBase string) {
			defer wg.Done()
			res, err := s.queryPeer(ctx, peerBase, filter)
			ch <- peerResult{url: peerBase, result: res, err: err}
		}(base)
	}
	wg.Wait()
	close(ch)

	allFlows := append([]config.GenericMap{}, local.Flows...)
	oldest, newest := local.OldestTimestamp, local.NewestTimestamp
	totalSize, totalCap := local.Size, local.Capacity
	anyTruncated := local.Truncated

	for pr := range ch {
		merged.PeersQueried++
		if pr.err != nil {
			merged.PeersFailed++
			merged.Warnings = append(merged.Warnings, Warning{
				Code:    WarningPeerQueryFailed,
				Peer:    pr.url,
				Message: pr.err.Error(),
			})
			continue
		}
		allFlows = append(allFlows, pr.result.Flows...)
		totalSize += pr.result.Size
		totalCap += pr.result.Capacity
		anyTruncated = anyTruncated || pr.result.Truncated
		if pr.result.OldestTimestamp > 0 && (oldest == 0 || pr.result.OldestTimestamp < oldest) {
			oldest = pr.result.OldestTimestamp
		}
		if pr.result.NewestTimestamp > newest {
			newest = pr.result.NewestTimestamp
		}
	}

	// Sort newest-first by flow timestamp and apply limit
	sort.SliceStable(allFlows, func(i, j int) bool {
		ti, _ := schema.FlowTimestampMs(allFlows[i])
		tj, _ := schema.FlowTimestampMs(allFlows[j])
		return ti > tj
	})
	limit := filter.Limit
	if limit <= 0 {
		limit = len(allFlows)
	}
	truncated := anyTruncated
	if len(allFlows) > limit {
		allFlows = allFlows[:limit]
		truncated = true
	}

	merged.Flows = allFlows
	merged.OldestTimestamp = oldest
	merged.NewestTimestamp = newest
	merged.Size = totalSize
	merged.Capacity = totalCap
	merged.Truncated = truncated
	return merged
}

func (s *Server) isSelf(peerBase string) bool {
	if s.selfAddr == "" {
		return false
	}
	// peerBase like http://10.0.0.1:9200 — compare host:port loosely
	u, err := url.Parse(peerBase)
	if err != nil {
		return false
	}
	self := strings.TrimPrefix(s.selfAddr, ":")
	if strings.HasSuffix(u.Host, ":"+self) || u.Host == self {
		return true
	}
	return false
}

func (s *Server) queryPeer(ctx context.Context, base string, filter QueryFilter) (QueryResult, error) {
	u, err := url.Parse(base + "/api/flowbuffer/local/flows")
	if err != nil {
		return QueryResult{}, err
	}
	q := u.Query()
	if filter.StartMs > 0 {
		q.Set("start", strconv.FormatInt(filter.StartMs, 10))
	}
	if filter.EndMs > 0 {
		q.Set("end", strconv.FormatInt(filter.EndMs, 10))
	}
	if filter.Limit > 0 {
		q.Set("limit", strconv.Itoa(filter.Limit))
	}
	for field, values := range filter.FieldEquals {
		for _, v := range values {
			q.Add("filter."+field, v)
		}
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return QueryResult{}, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return QueryResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return QueryResult{}, fmt.Errorf("peer %s returned %d: %s", base, resp.StatusCode, string(body))
	}
	var result QueryResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return QueryResult{}, err
	}
	return result, nil
}

func parseQueryFilter(r *http.Request) (QueryFilter, error) {
	f := QueryFilter{FieldEquals: map[string][]string{}}

	// Optional JSON body (POST)
	if r.Method == http.MethodPost && r.Body != nil && r.Header.Get("Content-Type") == "application/json" {
		var body struct {
			Start   interface{}         `json:"start"`
			End     interface{}         `json:"end"`
			Limit   int                 `json:"limit"`
			Filters map[string][]string `json:"filters"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil && err != io.EOF {
			return f, fmt.Errorf("invalid JSON body: %w", err)
		}
		f.Limit = body.Limit
		if body.Filters != nil {
			f.FieldEquals = body.Filters
		}
		var err error
		f.StartMs, err = parseTimeMs(body.Start)
		if err != nil {
			return f, fmt.Errorf("invalid start: %w", err)
		}
		f.EndMs, err = parseTimeMs(body.End)
		if err != nil {
			return f, fmt.Errorf("invalid end: %w", err)
		}
	}

	q := r.URL.Query()
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return f, fmt.Errorf("invalid limit: %w", err)
		}
		f.Limit = n
	}
	if v := q.Get("start"); v != "" {
		n, err := parseTimeMs(v)
		if err != nil {
			return f, fmt.Errorf("invalid start: %w", err)
		}
		f.StartMs = n
	}
	if v := q.Get("end"); v != "" {
		n, err := parseTimeMs(v)
		if err != nil {
			return f, fmt.Errorf("invalid end: %w", err)
		}
		f.EndMs = n
	}
	for key, values := range q {
		if strings.HasPrefix(key, "filter.") {
			field := strings.TrimPrefix(key, "filter.")
			f.FieldEquals[field] = append(f.FieldEquals[field], values...)
		}
	}
	return f, nil
}

func parseTimeMs(v interface{}) (int64, error) {
	if v == nil {
		return 0, nil
	}
	switch t := v.(type) {
	case float64:
		return int64(t), nil
	case int64:
		return t, nil
	case int:
		return int64(t), nil
	case json.Number:
		return t.Int64()
	case string:
		if t == "" {
			return 0, nil
		}
		if n, err := strconv.ParseInt(t, 10, 64); err == nil {
			return n, nil
		}
		tm, err := time.Parse(time.RFC3339, t)
		if err != nil {
			return 0, err
		}
		return tm.UnixMilli(), nil
	default:
		return 0, fmt.Errorf("unsupported time type %T", v)
	}
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if err := enc.Encode(v); err != nil {
		qlog.WithError(err).Warn("failed to encode query response")
	}
}
