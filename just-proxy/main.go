package main

import (
	"flag"
	"log"
	"os"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	dbPath := flag.String("db", "proxy-stats.db", "path to SQLite database (empty to disable)")
	flag.Parse()

	log.SetFlags(log.Ltime | log.Lshortfile)
	log.SetOutput(os.Stdout)

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	var store *Store
	if *dbPath != "" {
		store, err = NewStore(*dbPath)
		if err != nil {
			log.Fatalf("open db: %v", err)
		}
		log.Printf("[db] recording stats to %s", *dbPath)
	}

	proxies := make([]*ProxyServer, 0, len(cfg.Proxies))
	for _, pc := range cfg.Proxies {
		p := NewProxyServer(pc.Name, pc.Port, store)
		proxies = append(proxies, p)
		go func(p *ProxyServer) {
			if err := p.Start(); err != nil {
				log.Fatalf("[%s] server error: %v", p.Name, err)
			}
		}(p)
	}

	if cfg.Admin.Port > 0 {
		admin := NewAdminServer(cfg.Admin.Port, proxies, store)
		go func() {
			if err := admin.Start(); err != nil {
				log.Fatalf("[admin] server error: %v", err)
			}
		}()
	}

	log.Printf("all proxies started. press Ctrl+C to stop.")
	select {}
}
