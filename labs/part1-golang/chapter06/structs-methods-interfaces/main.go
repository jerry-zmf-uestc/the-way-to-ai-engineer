package main

import (
	"errors"
	"fmt"
	"reflect"
)

type Account struct {
	Owner   string
	Balance int
}

func (a *Account) Deposit(amount int) error {
	if amount <= 0 {
		return errors.New("deposit amount must be positive")
	}
	a.Balance += amount
	return nil
}

func (a *Account) Withdraw(amount int) error {
	if amount <= 0 {
		return errors.New("withdraw amount must be positive")
	}
	if amount > a.Balance {
		return fmt.Errorf("withdraw %d: balance %d", amount, a.Balance)
	}
	a.Balance -= amount
	return nil
}

func (a Account) Summary() string {
	return fmt.Sprintf("%s has %d credits", a.Owner, a.Balance)
}

type Formatter interface {
	Format() string
}

type AccountSummary struct {
	Account Account
}

func (s AccountSummary) Format() string {
	return s.Account.Summary()
}

func Format(formatter Formatter) string {
	return formatter.Format()
}

func DescribeValue(v any) string {
	if v == nil {
		return "nil interface"
	}
	return "non-nil interface holding " + reflect.TypeOf(v).String()
}

func main() {
	account := Account{Owner: "Ada"}
	if err := account.Deposit(120); err != nil {
		fmt.Println("error:", err)
		return
	}
	if err := account.Withdraw(45); err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println(Format(AccountSummary{Account: account}))
	fmt.Println(DescribeValue(nil))
	var typedNil *Account
	fmt.Println(DescribeValue(typedNil))
}
