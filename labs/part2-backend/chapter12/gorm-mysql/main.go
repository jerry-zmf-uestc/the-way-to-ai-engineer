package main

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Product struct {
	ID         uint       `gorm:"primaryKey"`
	SKU        string     `gorm:"size:64;uniqueIndex;not null"`
	Name       string     `gorm:"size:128;not null"`
	PriceCents int64      `gorm:"not null;check:price_cents >= 0"`
	Active     bool       `gorm:"not null;default:true"`
	CreatedAt  time.Time  `gorm:"not null"`
	UpdatedAt  time.Time  `gorm:"not null"`
	DeletedAt  *time.Time `gorm:"index"`
}

func (Product) TableName() string {
	return "products"
}

func (p Product) Summary() string {
	state := "inactive"
	if p.Active {
		state = "active"
	}
	return fmt.Sprintf("%s %s %s $%.2f", p.SKU, p.Name, state, float64(p.PriceCents)/100)
}

type MySQLConfig struct {
	User     string
	Password string
	Host     string
	Port     int
	Database string
	Charset  string
	Location *time.Location
}

func DefaultMySQLConfig() MySQLConfig {
	return MySQLConfig{
		User:     "app",
		Password: "local-password",
		Host:     "127.0.0.1",
		Port:     3306,
		Database: "commerce",
		Charset:  "utf8mb4",
		Location: time.Local,
	}
}

func (c MySQLConfig) Validate() error {
	if strings.TrimSpace(c.User) == "" {
		return fmt.Errorf("mysql user is required")
	}
	if strings.TrimSpace(c.Host) == "" {
		return fmt.Errorf("mysql host is required")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("mysql port must be between 1 and 65535")
	}
	if strings.TrimSpace(c.Database) == "" {
		return fmt.Errorf("mysql database is required")
	}
	return nil
}

func (c MySQLConfig) DSN() (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}

	charset := c.Charset
	if charset == "" {
		charset = "utf8mb4"
	}
	loc := c.Location
	if loc == nil {
		loc = time.Local
	}

	params := url.Values{}
	params.Set("charset", charset)
	params.Set("parseTime", "true")
	params.Set("loc", loc.String())

	credentials := url.QueryEscape(c.User)
	if c.Password != "" {
		credentials += ":" + url.QueryEscape(c.Password)
	}
	return fmt.Sprintf(
		"%s@tcp(%s)/%s?%s",
		credentials,
		net.JoinHostPort(c.Host, strconv.Itoa(c.Port)),
		url.QueryEscape(c.Database),
		params.Encode(),
	), nil
}

func (c MySQLConfig) RedactedDSN() (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	redacted := c
	if redacted.Password != "" {
		redacted.Password = "***"
	}
	return redacted.DSN()
}

type GORMSettings struct {
	Dialect                                  string
	DisableForeignKeyConstraintWhenMigrating bool
	SingularTable                            bool
}

func NewGORMSettings() GORMSettings {
	return GORMSettings{
		Dialect:                                  "mysql",
		DisableForeignKeyConstraintWhenMigrating: true,
		SingularTable:                            false,
	}
}

func (s GORMSettings) Validate() error {
	if s.Dialect != "mysql" {
		return fmt.Errorf("gorm dialect must be mysql")
	}
	return nil
}

func BuildConnectionPlan(c MySQLConfig, s GORMSettings) (string, error) {
	if err := s.Validate(); err != nil {
		return "", err
	}
	dsn, err := c.RedactedDSN()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"dialect=%s table=%s disable_fk_migration=%t singular_table=%t dsn=%s",
		s.Dialect,
		Product{}.TableName(),
		s.DisableForeignKeyConstraintWhenMigrating,
		s.SingularTable,
		dsn,
	), nil
}

func main() {
	cfg := DefaultMySQLConfig()
	plan, err := BuildConnectionPlan(cfg, NewGORMSettings())
	if err != nil {
		fmt.Printf("invalid connection plan: %v\n", err)
		return
	}

	product := Product{
		SKU:        "SKU-001",
		Name:       "CloudWeGo Hoodie",
		PriceCents: 8999,
		Active:     true,
	}

	fmt.Printf("gorm model=%s %s\n", product.Summary(), plan)
}
