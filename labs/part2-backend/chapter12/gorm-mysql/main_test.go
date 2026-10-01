package main

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestMySQLConfigBuildsParseableDSN(t *testing.T) {
	cfg := MySQLConfig{
		User:     "shop_app",
		Password: "p@ss word",
		Host:     "db.internal",
		Port:     3306,
		Database: "commerce",
		Charset:  "utf8mb4",
		Location: time.UTC,
	}

	dsn, err := cfg.DSN()
	if err != nil {
		t.Fatalf("DSN() error = %v", err)
	}

	before, after, ok := strings.Cut(dsn, "@tcp(")
	if !ok {
		t.Fatalf("DSN() = %q, want @tcp segment", dsn)
	}
	user, encodedPassword, ok := strings.Cut(before, ":")
	if !ok {
		t.Fatalf("credentials = %q, want user:password", before)
	}
	if user != cfg.User {
		t.Fatalf("user = %q, want %q", user, cfg.User)
	}
	password, err := url.QueryUnescape(encodedPassword)
	if err != nil {
		t.Fatalf("QueryUnescape(%q) error = %v", encodedPassword, err)
	}
	if password != cfg.Password {
		t.Fatalf("password = %q, want original password", password)
	}

	addr, rest, ok := strings.Cut(after, ")/")
	if !ok {
		t.Fatalf("connection target = %q, want addr)/db", after)
	}
	if addr != "db.internal:3306" {
		t.Fatalf("addr = %q, want db.internal:3306", addr)
	}
	database, query, ok := strings.Cut(rest, "?")
	if !ok {
		t.Fatalf("database/query = %q, want query string", rest)
	}
	if database != cfg.Database {
		t.Fatalf("database = %q, want %q", database, cfg.Database)
	}
	values, err := url.ParseQuery(query)
	if err != nil {
		t.Fatalf("ParseQuery(%q) error = %v", query, err)
	}
	if values.Get("charset") != "utf8mb4" {
		t.Fatalf("charset = %q, want utf8mb4", values.Get("charset"))
	}
	if values.Get("parseTime") != "true" {
		t.Fatalf("parseTime = %q, want true", values.Get("parseTime"))
	}
}

func TestMySQLConfigValidation(t *testing.T) {
	tests := []struct {
		name string
		cfg  MySQLConfig
		want string
	}{
		{
			name: "missing user",
			cfg:  MySQLConfig{Host: "127.0.0.1", Port: 3306, Database: "commerce"},
			want: "user",
		},
		{
			name: "missing host",
			cfg:  MySQLConfig{User: "app", Port: 3306, Database: "commerce"},
			want: "host",
		},
		{
			name: "bad port",
			cfg:  MySQLConfig{User: "app", Host: "127.0.0.1", Port: 70000, Database: "commerce"},
			want: "port",
		},
		{
			name: "missing database",
			cfg:  MySQLConfig{User: "app", Host: "127.0.0.1", Port: 3306},
			want: "database",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if err == nil {
				t.Fatal("Validate() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %q, want it to mention %q", err, tt.want)
			}
		})
	}
}

func TestRedactedDSNDoesNotExposePassword(t *testing.T) {
	cfg := DefaultMySQLConfig()
	cfg.Password = "secret-password"

	dsn, err := cfg.RedactedDSN()
	if err != nil {
		t.Fatalf("RedactedDSN() error = %v", err)
	}
	if strings.Contains(dsn, "secret-password") {
		t.Fatalf("redacted DSN exposed password: %q", dsn)
	}
	credentials, _, ok := strings.Cut(dsn, "@tcp(")
	if !ok {
		t.Fatalf("redacted DSN = %q, want @tcp segment", dsn)
	}
	_, encodedPassword, ok := strings.Cut(credentials, ":")
	if !ok {
		t.Fatalf("credentials = %q, want user:password", credentials)
	}
	password, err := url.QueryUnescape(encodedPassword)
	if err != nil {
		t.Fatalf("QueryUnescape(%q) error = %v", encodedPassword, err)
	}
	if password != "***" {
		t.Fatalf("redacted password = %q, want ***", password)
	}
}

func TestProductModelSummaryAndTableName(t *testing.T) {
	product := Product{
		SKU:        "SKU-001",
		Name:       "CloudWeGo Hoodie",
		PriceCents: 8999,
		Active:     true,
	}

	if product.TableName() != "products" {
		t.Fatalf("TableName() = %q, want products", product.TableName())
	}

	got := product.Summary()
	want := "SKU-001 CloudWeGo Hoodie active $89.99"
	if got != want {
		t.Fatalf("Summary() = %q, want %q", got, want)
	}
}

func TestBuildConnectionPlan(t *testing.T) {
	plan, err := BuildConnectionPlan(DefaultMySQLConfig(), NewGORMSettings())
	if err != nil {
		t.Fatalf("BuildConnectionPlan() error = %v", err)
	}
	for _, want := range []string{
		"dialect=mysql",
		"table=products",
		"disable_fk_migration=true",
		"singular_table=false",
		"dsn=app:%2A%2A%2A@tcp(127.0.0.1:3306)/commerce",
	} {
		if !strings.Contains(plan, want) {
			t.Fatalf("plan = %q, want it to contain %q", plan, want)
		}
	}
	if strings.Contains(plan, "local-password") {
		t.Fatalf("plan exposed password: %q", plan)
	}

	settings := NewGORMSettings()
	if !settings.DisableForeignKeyConstraintWhenMigrating {
		t.Fatal("DisableForeignKeyConstraintWhenMigrating = false, want true for explicit migration control")
	}
}
