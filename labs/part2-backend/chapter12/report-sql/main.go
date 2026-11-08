package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func sql(db, statement string) (string, error) {
	cmd := exec.Command("sqlite3", "-batch", "-bail", "-noheader", db, statement)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, output)
	}
	return strings.TrimSpace(string(output)), nil
}

func expect(db, statement, want string) error {
	got, err := sql(db, statement)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("got %q, want %q", got, want)
	}
	return nil
}

func plan(db string) error {
	if _, err := sql(db, `
CREATE TABLE reports(id INTEGER PRIMARY KEY, owner INTEGER NOT NULL, created_at INTEGER NOT NULL);
WITH RECURSIVE numbers(n) AS (VALUES(1) UNION ALL SELECT n+1 FROM numbers WHERE n<20000)
INSERT INTO reports SELECT n,n%100,n/5 FROM numbers;`); err != nil {
		return err
	}
	query := "SELECT id FROM reports WHERE owner=7 ORDER BY created_at DESC,id DESC LIMIT 20"
	before, err := sql(db, "EXPLAIN QUERY PLAN "+query)
	if err != nil {
		return err
	}
	rows, err := sql(db, query)
	if err != nil {
		return err
	}
	if _, err = sql(db, "CREATE INDEX reports_list ON reports(owner,created_at DESC,id DESC); ANALYZE"); err != nil {
		return err
	}
	after, err := sql(db, "EXPLAIN QUERY PLAN "+query)
	if err != nil {
		return err
	}
	fmt.Printf("before:\n%s\nafter:\n%s\n", before, after)
	if !strings.Contains(before, "SCAN reports") || !strings.Contains(before, "TEMP B-TREE") ||
		!strings.Contains(after, "reports_list") || strings.Contains(after, "TEMP B-TREE") {
		return errors.New("expected scan plus sort before, ordered index search after")
	}
	return expect(db, query, rows)
}

func compete(db string) error {
	if _, err := sql(db, "CREATE TABLE reports(id INTEGER PRIMARY KEY, status TEXT); INSERT INTO reports VALUES(1,'queued')"); err != nil {
		return err
	}
	holder := exec.Command("sqlite3", "-batch", "-bail", db)
	input, err := holder.StdinPipe()
	if err != nil {
		return err
	}
	output, err := holder.StdoutPipe()
	if err != nil {
		return err
	}
	holder.Stderr = os.Stderr
	if err := holder.Start(); err != nil {
		return err
	}
	defer func() { input.Close(); holder.Wait() }()
	reader := bufio.NewReader(output)
	fmt.Fprintln(input, "BEGIN IMMEDIATE; UPDATE reports SET status='running' WHERE id=1 AND status='queued'; SELECT changes();")
	if line, err := reader.ReadString('\n'); err != nil || strings.TrimSpace(line) != "1" {
		return fmt.Errorf("A did not claim: %q, %v", line, err)
	}
	if err := expect(db, "SELECT status FROM reports WHERE id=1", "queued"); err != nil {
		return err
	}
	claim := "UPDATE reports SET status='running' WHERE id=1 AND status='queued'; SELECT changes()"
	if _, err := sql(db, claim); err == nil || !strings.Contains(err.Error(), "database is locked") {
		return fmt.Errorf("B should see SQLITE_BUSY: %v", err)
	}
	fmt.Fprintln(input, "COMMIT; SELECT 'committed';")
	if line, err := reader.ReadString('\n'); err != nil || strings.TrimSpace(line) != "committed" {
		return fmt.Errorf("A did not commit: %q, %v", line, err)
	}
	if err := expect(db, claim, "0"); err != nil {
		return err
	}
	if err := expect(db, "BEGIN; UPDATE reports SET status='succeeded'; ROLLBACK; SELECT status FROM reports", "running"); err != nil {
		return err
	}
	fmt.Println("A_claimed=1 B_during_write=SQLITE_BUSY B_after_commit=0 rollback_status=running")
	return nil
}

const schema = `
CREATE TABLE reports(id INTEGER PRIMARY KEY, result TEXT, writes INTEGER NOT NULL DEFAULT 0);
CREATE TABLE outbox(report_id INTEGER PRIMARY KEY, acked INTEGER NOT NULL DEFAULT 0);
`

func worker(db string, crash bool) error {
	// A second delivery after commit finds the same result and performs no write.
	if _, err := sql(db, `BEGIN IMMEDIATE;
UPDATE reports SET result='report-1,42', writes=writes+1
WHERE result IS NULL AND id IN (SELECT report_id FROM outbox WHERE acked=0);
COMMIT;`); err != nil {
		return err
	}
	if crash {
		os.Exit(73)
	}
	_, err := sql(db, "UPDATE outbox SET acked=1 WHERE report_id IN (SELECT id FROM reports WHERE result IS NOT NULL)")
	return err
}

func recoverReport(db string) error {
	if _, err := sql(db, schema+`
BEGIN;
INSERT INTO reports(id) VALUES(99);
INSERT INTO outbox(report_id) VALUES(99);
ROLLBACK;
BEGIN;
INSERT INTO reports(id) VALUES(1);
INSERT INTO outbox(report_id) VALUES(1);
COMMIT;`); err != nil {
		return err
	}
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	if err := expect(db, "SELECT reports.id, outbox.acked FROM reports JOIN outbox ON reports.id=outbox.report_id", "1|0"); err != nil {
		return err
	}
	crashed := exec.Command(binary, "_worker", db, "crash")
	crashed.Stderr = os.Stderr
	var exit *exec.ExitError
	if err := crashed.Run(); !errors.As(err, &exit) || exit.ExitCode() != 73 {
		return fmt.Errorf("expected crash after durable result: %v", err)
	}
	query := "SELECT reports.result, reports.writes, outbox.acked FROM reports JOIN outbox ON reports.id=outbox.report_id"
	if err := expect(db, query, "report-1,42|1|0"); err != nil {
		return err
	}
	if output, err := exec.Command(binary, "_worker", db, "resume").CombinedOutput(); err != nil {
		return fmt.Errorf("worker restart: %w: %s", err, output)
	}
	if err := expect(db, query, "report-1,42|1|1"); err != nil {
		return err
	}
	fmt.Println("after_crash=report-1,42|1|0 after_restart=report-1,42|1|1")
	return nil
}

func run(mode string) error {
	dir, err := os.MkdirTemp("", "report-sql-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	switch mode {
	case "plan":
		return plan(filepath.Join(dir, "plan.db"))
	case "transaction":
		return compete(filepath.Join(dir, "transaction.db"))
	case "recovery":
		return recoverReport(filepath.Join(dir, "recovery.db"))
	default:
		return fmt.Errorf("choose plan, transaction, or recovery")
	}
}

func main() {
	if len(os.Args) == 4 && os.Args[1] == "_worker" {
		// The parent passes only its private temporary database to this subprocess.
		if err := worker(os.Args[2], os.Args[3] == "crash"); err != nil {
			log.Fatal(err)
		}
		return
	}
	if len(os.Args) != 2 {
		log.Fatal("usage: report-sql {plan|transaction|recovery}")
	}
	if err := run(os.Args[1]); err != nil {
		log.Fatal(err)
	}
}
