package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"
)

type sqliteStore struct {
	path string
	id   string
}

func (s *sqliteStore) sql(statement string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sqlite3", "-batch", "-bail", "-json",
		"-cmd", ".timeout 1500", s.path, statement)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("sqlite3: %w", err)
	}
	return out, nil
}

func sqlText(s string) string {
	return "CAST(X'" + hex.EncodeToString([]byte(s)) + "' AS TEXT)"
}

func (l *lab) openSQLite(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	s := &sqliteStore{path: absolute}
	out, err := s.sql(`
PRAGMA synchronous=FULL;
BEGIN IMMEDIATE;
CREATE TABLE IF NOT EXISTS report_flow_meta(id TEXT NOT NULL);
INSERT INTO report_flow_meta SELECT lower(hex(randomblob(16)))
WHERE NOT EXISTS (SELECT 1 FROM report_flow_meta);
CREATE TABLE IF NOT EXISTS report_flow_jobs(
 seq INTEGER PRIMARY KEY,
 id TEXT NOT NULL UNIQUE,
 owner TEXT NOT NULL,
 idem TEXT NOT NULL,
 enqueued_ns INTEGER NOT NULL,
 instance TEXT NOT NULL DEFAULT '',
 result TEXT NOT NULL DEFAULT '',
 result_writes INTEGER NOT NULL DEFAULT 0 CHECK(result_writes IN (0,1)),
 UNIQUE(owner,idem)
);
CREATE TABLE IF NOT EXISTS report_flow_outbox(
 id TEXT PRIMARY KEY,
 acked INTEGER NOT NULL DEFAULT 0 CHECK(acked IN (0,1))
);
COMMIT;
SELECT id FROM report_flow_meta;`)
	if err != nil {
		return err
	}
	var identity []struct{ ID string }
	if err := json.Unmarshal(out, &identity); err != nil || len(identity) != 1 {
		return fmt.Errorf("invalid SQLite lab identity")
	}
	s.id = identity[0].ID
	out, err = s.sql(`SELECT j.*, o.acked FROM report_flow_jobs j
JOIN report_flow_outbox o ON o.id=j.id ORDER BY j.seq;`)
	if err != nil {
		return err
	}
	var rows []struct {
		Seq, Acked                        int
		ID, Owner, Idem, Instance, Result string
		EnqueuedNS                        int64 `json:"enqueued_ns"`
		ResultWrites                      int   `json:"result_writes"`
	}
	if len(out) > 0 {
		if err := json.Unmarshal(out, &rows); err != nil {
			return err
		}
	}
	l.store = s
	for _, row := range rows {
		j := job{ID: row.ID, Status: "queued", owner: row.Owner,
			enqueued: time.Unix(0, row.EnqueuedNS), Instance: row.Instance, Result: row.Result, ResultWrites: row.ResultWrites}
		if j.ResultWrites == 1 {
			j.Status = "succeeded"
			l.counts.Completed++
		}
		l.jobs[j.ID] = j
		l.keys[submissionKey{row.Owner, row.Idem}] = j.ID
		l.counts.Accepted = max(l.counts.Accepted, row.Seq)
		if row.Acked == 0 {
			l.queue = append(l.queue, j.ID)
			l.recovered++
		}
	}
	l.counts.PeakDepth = len(l.queue)
	l.signal()
	return nil
}

func (s *sqliteStore) accept(seq int, j job, key submissionKey) error {
	_, err := s.sql(fmt.Sprintf(`PRAGMA synchronous=FULL;
BEGIN IMMEDIATE;
INSERT INTO report_flow_jobs(seq,id,owner,idem,enqueued_ns) VALUES(%d,%s,%s,%s,%d);
INSERT INTO report_flow_outbox(id) VALUES(%s);
COMMIT;`, seq, sqlText(j.ID), sqlText(j.owner), sqlText(key.key), j.enqueued.UnixNano(), sqlText(j.ID)))
	return err
}

func (s *sqliteStore) finish(j job) error {
	out, err := s.sql(fmt.Sprintf(`PRAGMA synchronous=FULL;
UPDATE report_flow_jobs SET result=%s, instance=%s, result_writes=result_writes+1
WHERE id=%s AND result_writes=0;
SELECT changes() AS writes;`, sqlText(j.Result), sqlText(j.Instance), sqlText(j.ID)))
	if err != nil {
		return err
	}
	var changes []struct{ Writes int }
	if err := json.Unmarshal(out, &changes); err != nil || len(changes) != 1 || changes[0].Writes != 1 {
		return fmt.Errorf("expected exactly one SQLite result write")
	}
	return nil
}

func (s *sqliteStore) ack(id string) error {
	_, err := s.sql("PRAGMA synchronous=FULL; UPDATE report_flow_outbox SET acked=1 WHERE id=" + sqlText(id))
	return err
}
