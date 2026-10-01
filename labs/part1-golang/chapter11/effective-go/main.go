package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var errEmptyQuantity = errors.New("quantity is empty")
var errNonPositiveQuantity = errors.New("quantity must be positive")

type order struct {
	ID       string
	Quantity int
}

type retryPolicy struct {
	Attempts int
}

func (p retryPolicy) normalizedAttempts() int {
	if p.Attempts > 0 {
		return p.Attempts
	}
	return 1
}

func parseQuantity(input string) (int, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return 0, errEmptyQuantity
	}

	quantity, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse quantity %q: %w", input, err)
	}
	if quantity <= 0 {
		return 0, fmt.Errorf("%w: %d", errNonPositiveQuantity, quantity)
	}
	return quantity, nil
}

func summarizeOrders(orders []order) string {
	switch len(orders) {
	case 0:
		return "no orders"
	case 1:
		return "1 order"
	default:
		return fmt.Sprintf("%d orders", len(orders))
	}
}

func main() {
	policy := retryPolicy{}
	quantity, err := parseQuantity("3")
	if err != nil {
		fmt.Println("parse failed:", err)
		return
	}

	orders := []order{{ID: "A-100", Quantity: quantity}}
	fmt.Println("attempts:", policy.normalizedAttempts())
	fmt.Println("summary:", summarizeOrders(orders))
}
