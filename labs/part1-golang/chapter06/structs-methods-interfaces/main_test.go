package main

import "testing"

func TestAccountMethodsUpdateBalance(t *testing.T) {
	account := Account{Owner: "Ada"}

	if err := account.Deposit(120); err != nil {
		t.Fatalf("Deposit() error = %v", err)
	}
	if err := account.Withdraw(45); err != nil {
		t.Fatalf("Withdraw() error = %v", err)
	}
	if got, want := account.Balance, 75; got != want {
		t.Fatalf("Balance = %d, want %d", got, want)
	}
}

func TestAccountMethodsRejectInvalidAmounts(t *testing.T) {
	account := Account{Owner: "Ada", Balance: 50}

	if err := account.Deposit(0); err == nil {
		t.Fatal("Deposit(0) error = nil, want error")
	}
	if err := account.Withdraw(75); err == nil {
		t.Fatal("Withdraw(75) error = nil, want error")
	}
}

func TestFormatterInterface(t *testing.T) {
	account := Account{Owner: "Ada", Balance: 75}

	got := Format(AccountSummary{Account: account})
	want := "Ada has 75 credits"
	if got != want {
		t.Fatalf("Format() = %q, want %q", got, want)
	}
}

func TestNilInterfaceDescription(t *testing.T) {
	if got := DescribeValue(nil); got != "nil interface" {
		t.Fatalf("DescribeValue(nil) = %q, want nil interface", got)
	}

	var account *Account
	if got := DescribeValue(account); got != "non-nil interface holding *main.Account" {
		t.Fatalf("DescribeValue(account) = %q", got)
	}
}
