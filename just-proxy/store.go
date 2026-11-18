package main

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func NewStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS requests (
			id        INTEGER PRIMARY KEY,
			proxy     TEXT    NOT NULL,
			method    TEXT    NOT NULL,
			host      TEXT    NOT NULL,
			status    INTEGER NOT NULL,
			bytes     INTEGER NOT NULL,
			duration_ms INTEGER NOT NULL,
			ts        DATETIME NOT NULL
		)
	`)
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

type RequestRecord struct {
	Proxy      string
	Method     string
	Host       string
	Status     int
	Bytes      int64
	DurationMs int64
	Ts         time.Time
}

func (s *Store) Record(r RequestRecord) error {
	_, err := s.db.Exec(
		`INSERT INTO requests (proxy, method, host, status, bytes, duration_ms, ts) VALUES (?,?,?,?,?,?,?)`,
		r.Proxy, r.Method, r.Host, r.Status, r.Bytes, r.DurationMs, r.Ts.UTC(),
	)
	return err
}

type HostStat struct {
	Host     string `json:"host"`
	Requests int64  `json:"requests"`
	Bytes    int64  `json:"bytes"`
}

type ProxySummary struct {
	Proxy      string `json:"proxy"`
	Requests   int64  `json:"requests"`
	Bytes      int64  `json:"bytes"`
	AvgMs      int64  `json:"avg_ms"`
	ErrorCount int64  `json:"error_count"`
}

func (s *Store) TopHosts(limit int) ([]HostStat, error) {
	rows, err := s.db.Query(`
		SELECT host, COUNT(*) AS requests, SUM(bytes) AS bytes
		FROM requests
		GROUP BY host
		ORDER BY requests DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]HostStat, 0)
	for rows.Next() {
		var h HostStat
		if err := rows.Scan(&h.Host, &h.Requests, &h.Bytes); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, nil
}

func (s *Store) ProxySummaries() ([]ProxySummary, error) {
	rows, err := s.db.Query(`
		SELECT
			proxy,
			COUNT(*) AS requests,
			SUM(bytes) AS bytes,
			AVG(duration_ms) AS avg_ms,
			SUM(CASE WHEN status >= 500 OR status = 0 THEN 1 ELSE 0 END) AS error_count
		FROM requests
		GROUP BY proxy
		ORDER BY proxy`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ProxySummary, 0)
	for rows.Next() {
		var p ProxySummary
		if err := rows.Scan(&p.Proxy, &p.Requests, &p.Bytes, &p.AvgMs, &p.ErrorCount); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// 過去N時間のリクエスト数を1時間単位で返す
type HourlyStat struct {
	Hour     string `json:"hour"`
	Requests int64  `json:"requests"`
	Bytes    int64  `json:"bytes"`
}

func (s *Store) HourlyStats(hours int) ([]HourlyStat, error) {
	rows, err := s.db.Query(`
		SELECT
			strftime('%Y-%m-%d %H:00', ts) AS hour,
			COUNT(*) AS requests,
			SUM(bytes) AS bytes
		FROM requests
		WHERE ts >= datetime('now', ? || ' hours')
		GROUP BY hour
		ORDER BY hour`, -hours)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]HourlyStat, 0)
	for rows.Next() {
		var h HourlyStat
		if err := rows.Scan(&h.Hour, &h.Requests, &h.Bytes); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, nil
}
