package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync/atomic"
	"time"
)

type ProxyServer struct {
	Name     string
	Port     int
	store    *Store
	requests atomic.Int64
	bytes    atomic.Int64
}

func NewProxyServer(name string, port int, store *Store) *ProxyServer {
	return &ProxyServer{Name: name, Port: port, store: store}
}

func (p *ProxyServer) Start() error {
	addr := fmt.Sprintf(":%d", p.Port)
	log.Printf("[%s] listening on %s", p.Name, addr)
	return http.ListenAndServe(addr, p)
}

func (p *ProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.requests.Add(1)
	if r.Method == http.MethodConnect {
		p.handleConnect(w, r)
	} else {
		p.handleHTTP(w, r)
	}
}

func (p *ProxyServer) handleConnect(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	dest, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		log.Printf("[%s] CONNECT %s: %v", p.Name, r.Host, err)
		p.record("CONNECT", r.Host, 502, 0, start)
		return
	}
	defer dest.Close()

	w.WriteHeader(http.StatusOK)

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		return
	}
	client, _, err := hijacker.Hijack()
	if err != nil {
		log.Printf("[%s] hijack: %v", p.Name, err)
		return
	}
	defer client.Close()

	log.Printf("[%s] CONNECT %s", p.Name, r.Host)
	n := tunnel(p, client, dest)
	p.record("CONNECT", r.Host, 200, n, start)
}

func (p *ProxyServer) handleHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	r.RequestURI = ""
	r.Header.Del("Proxy-Connection")

	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		log.Printf("[%s] %s %s: %v", p.Name, r.Method, r.URL, err)
		p.record(r.Method, r.Host, 502, 0, start)
		return
	}
	defer resp.Body.Close()

	log.Printf("[%s] %s %s -> %d", p.Name, r.Method, r.URL, resp.StatusCode)

	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	n, _ := io.Copy(w, resp.Body)
	p.bytes.Add(n)
	p.record(r.Method, r.Host, resp.StatusCode, n, start)
}

func (p *ProxyServer) record(method, host string, status int, bytes int64, start time.Time) {
	if p.store == nil {
		return
	}
	go p.store.Record(RequestRecord{
		Proxy:      p.Name,
		Method:     method,
		Host:       host,
		Status:     status,
		Bytes:      bytes,
		DurationMs: time.Since(start).Milliseconds(),
		Ts:         start,
	})
}

func tunnel(p *ProxyServer, a, b net.Conn) int64 {
	done := make(chan int64, 2)
	go func() {
		n, _ := io.Copy(b, a)
		done <- n
	}()
	go func() {
		n, _ := io.Copy(a, b)
		done <- n
	}()
	n1 := <-done
	n2 := <-done
	total := n1 + n2
	p.bytes.Add(total)
	return total
}

func (p *ProxyServer) Stats() (requests int64, bytesTotal int64) {
	return p.requests.Load(), p.bytes.Load()
}
