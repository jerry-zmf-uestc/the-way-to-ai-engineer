package main

import (
	"errors"
	"fmt"
	"os"
)

// ErrConfigNotFound is returned when the config file cannot be located.
var ErrConfigNotFound = errors.New("config not found")

// LoadConfig reads a config file and wraps low-level errors with %w so that
// callers can keep using errors.Is / errors.As to inspect the failure.
func LoadConfig(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// 同时保留 sentinel 与底层错误，调用方两种判定方式都能用。
			return nil, fmt.Errorf("load config %q: %w", path, errors.Join(ErrConfigNotFound, err))
		}
		return nil, fmt.Errorf("load config %q: %w", path, err)
	}
	return data, nil
}

func main() {
	if _, err := LoadConfig("/no/such/path"); err != nil {
		// 推荐写法：用 errors.Is 判断 sentinel error，而不是字符串比较
		if errors.Is(err, ErrConfigNotFound) {
			fmt.Println("missing:", err)
			return
		}
		fmt.Println("error:", err)
	}
}
