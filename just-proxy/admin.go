package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type AdminServer struct {
	port    int
	proxies []*ProxyServer
	store   *Store
}

func NewAdminServer(port int, proxies []*ProxyServer, store *Store) *AdminServer {
	return &AdminServer{port: port, proxies: proxies, store: store}
}

func (a *AdminServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.handleIndex)
	mux.HandleFunc("/status", a.handleStatus)
	mux.HandleFunc("/stats/hosts", a.handleTopHosts)
	mux.HandleFunc("/stats/proxies", a.handleProxySummaries)
	mux.HandleFunc("/stats/hourly", a.handleHourly)

	addr := fmt.Sprintf(":%d", a.port)
	log.Printf("[admin] listening on http://localhost%s", addr)
	return http.ListenAndServe(addr, mux)
}

type proxyStatus struct {
	Name     string `json:"name"`
	Port     int    `json:"port"`
	Addr     string `json:"addr"`
	Requests int64  `json:"requests"`
	Bytes    int64  `json:"bytes"`
}

func (a *AdminServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	statuses := make([]proxyStatus, 0, len(a.proxies))
	for _, p := range a.proxies {
		reqs, b := p.Stats()
		statuses = append(statuses, proxyStatus{
			Name:     p.Name,
			Port:     p.Port,
			Addr:     fmt.Sprintf("localhost:%d", p.Port),
			Requests: reqs,
			Bytes:    b,
		})
	}
	json.NewEncoder(w).Encode(statuses)
}

func (a *AdminServer) handleTopHosts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if a.store == nil {
		json.NewEncoder(w).Encode([]struct{}{})
		return
	}
	hosts, err := a.store.TopHosts(20)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(hosts)
}

func (a *AdminServer) handleProxySummaries(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if a.store == nil {
		json.NewEncoder(w).Encode([]struct{}{})
		return
	}
	summaries, err := a.store.ProxySummaries()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(summaries)
}

func (a *AdminServer) handleHourly(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if a.store == nil {
		json.NewEncoder(w).Encode([]struct{}{})
		return
	}
	stats, err := a.store.HourlyStats(24)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(stats)
}

func (a *AdminServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>just-proxy admin</title>
<style>
body { font-family: monospace; padding: 2em; background: #1a1a1a; color: #eee; }
h1, h2 { color: #7af; margin-bottom: 0.5em; }
h2 { font-size: 1em; margin-top: 1.5em; color: #9cf; }
table { border-collapse: collapse; width: 100%%; margin-bottom: 1em; }
th, td { border: 1px solid #444; padding: 6px 12px; text-align: left; }
th { background: #333; }
tr:hover { background: #2a2a2a; }
.tag { display: inline-block; padding: 1px 6px; border-radius: 3px; font-size: 0.85em; }
.ok { background: #1a4; color: #afa; }
.err { background: #600; color: #faa; }
p.note { color: #888; font-size: 0.85em; }
</style>
<script>
async function j(url) {
  const r = await fetch(url);
  return r.json();
}
function bytes(b) {
  if (b < 1024) return b + ' B';
  if (b < 1048576) return (b/1024).toFixed(1) + ' KB';
  return (b/1048576).toFixed(1) + ' MB';
}
function row(...cells) {
  return '<tr>' + cells.map(c => '<td>' + c + '</td>').join('') + '</tr>';
}

async function refresh() {
  // proxy status (in-memory)
  const status = await j('/status');
  document.getElementById('t-status').innerHTML = status.map(p =>
    row(p.name, p.addr, p.requests, bytes(p.bytes))
  ).join('');

  // per-proxy summary (sqlite)
  const proxies = await j('/stats/proxies');
  document.getElementById('t-proxies').innerHTML = proxies.length === 0
    ? '<tr><td colspan="5" style="color:#888">no data yet</td></tr>'
    : proxies.map(p =>
        row(p.proxy, p.requests, bytes(p.bytes), p.avg_ms + ' ms',
          '<span class="tag ' + (p.error_count > 0 ? 'err' : 'ok') + '">' + p.error_count + '</span>')
      ).join('');

  // top hosts (sqlite)
  const hosts = await j('/stats/hosts');
  document.getElementById('t-hosts').innerHTML = hosts.length === 0
    ? '<tr><td colspan="3" style="color:#888">no data yet</td></tr>'
    : hosts.map(h => row(h.host, h.requests, bytes(h.bytes))).join('');

  // hourly (sqlite)
  const hourly = await j('/stats/hourly');
  document.getElementById('t-hourly').innerHTML = hourly.length === 0
    ? '<tr><td colspan="3" style="color:#888">no data yet</td></tr>'
    : hourly.map(h => row(h.hour, h.requests, bytes(h.bytes))).join('');
}

refresh();
setInterval(refresh, 3000);
</script>
</head>
<body>
<h1>just-proxy admin</h1>

<h2>Proxy status (live)</h2>
<table>
<thead><tr><th>Name</th><th>Address</th><th>Requests</th><th>Traffic</th></tr></thead>
<tbody id="t-status"></tbody>
</table>

<h2>Per-proxy summary (all time)</h2>
<table>
<thead><tr><th>Proxy</th><th>Requests</th><th>Traffic</th><th>Avg latency</th><th>Errors</th></tr></thead>
<tbody id="t-proxies"></tbody>
</table>

<h2>Top hosts (all time)</h2>
<table>
<thead><tr><th>Host</th><th>Requests</th><th>Traffic</th></tr></thead>
<tbody id="t-hosts"></tbody>
</table>

<h2>Hourly traffic (last 24h)</h2>
<table>
<thead><tr><th>Hour (UTC)</th><th>Requests</th><th>Traffic</th></tr></thead>
<tbody id="t-hourly"></tbody>
</table>

<p class="note">Auto-refreshes every 3s &nbsp;|&nbsp;
  <a href="/status" style="color:#7af">/status</a> &nbsp;
  <a href="/stats/proxies" style="color:#7af">/stats/proxies</a> &nbsp;
  <a href="/stats/hosts" style="color:#7af">/stats/hosts</a> &nbsp;
  <a href="/stats/hourly" style="color:#7af">/stats/hourly</a>
</p>
</body>
</html>`)
}
