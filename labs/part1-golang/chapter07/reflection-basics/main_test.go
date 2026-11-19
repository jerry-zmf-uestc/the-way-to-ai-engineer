package main

import (
	"errors"
	"fmt"
	"testing"
)

func TestDescribeType(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{name: "builtin float", in: 3.14, want: "float64(float64)"},
		{name: "named struct", in: User{}, want: "User(struct)"},
		{name: "unnamed slice", in: []int{}, want: "[]int(slice)"},
		{name: "pointer", in: &User{}, want: "*main.User(ptr)"},
		{name: "nil interface", in: nil, want: "<nil>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := describeType(tt.in); got != tt.want {
				t.Errorf("describeType(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFieldTags(t *testing.T) {
	t.Run("json tags of struct", func(t *testing.T) {
		got, err := fieldTags(User{}, "json")
		if err != nil {
			t.Fatalf("fieldTags() error = %v, want nil", err)
		}
		want := []string{"Name=name", "Age=age"} // role is unexported and skipped
		if len(got) != len(want) {
			t.Fatalf("fieldTags() = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("fieldTags()[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("accepts pointer to struct", func(t *testing.T) {
		got, err := fieldTags(&User{}, "vd")
		if err != nil {
			t.Fatalf("fieldTags(ptr) error = %v, want nil", err)
		}
		if len(got) == 0 || got[0] != "Name=len($)>0" {
			t.Errorf("fieldTags(ptr) = %v, want first entry Name=len($)>0", got)
		}
	})

	t.Run("rejects non-struct", func(t *testing.T) {
		_, err := fieldTags(42, "json")
		if !errors.Is(err, ErrNotStruct) {
			t.Errorf("fieldTags(int) error = %v, want ErrNotStruct", err)
		}
	})
}

func TestSetString(t *testing.T) {
	t.Run("sets exported string field", func(t *testing.T) {
		u := User{Name: "old"}
		if err := setString(&u, "Name", "new"); err != nil {
			t.Fatalf("setString() error = %v, want nil", err)
		}
		if u.Name != "new" {
			t.Errorf("u.Name = %q, want new", u.Name)
		}
	})

	tests := []struct {
		name    string
		ptr     any
		field   string
		wantErr error
	}{
		{name: "non-pointer", ptr: User{}, field: "Name", wantErr: ErrNotPointer},
		{name: "nil pointer", ptr: (*User)(nil), field: "Name", wantErr: ErrNotPointer},
		{name: "missing field", ptr: &User{}, field: "Missing", wantErr: ErrFieldMissing},
		{name: "non-string field", ptr: &User{}, field: "Age", wantErr: ErrFieldType},
		{name: "unexported field", ptr: &User{}, field: "role", wantErr: ErrNotSettable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := setString(tt.ptr, tt.field, "x")
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("setString() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestCallMethod(t *testing.T) {
	t.Run("calls Add", func(t *testing.T) {
		got, err := callMethod(Calculator{}, "Add", 3, 4)
		if err != nil {
			t.Fatalf("callMethod() error = %v, want nil", err)
		}
		if len(got) != 1 {
			t.Fatalf("callMethod() returned %d values, want 1", len(got))
		}
		sum, ok := got[0].(int)
		if !ok || sum != 7 {
			t.Errorf("callMethod() = %v, want 7", got[0])
		}
	})

	t.Run("missing method", func(t *testing.T) {
		_, err := callMethod(Calculator{}, "Sub", 1, 2)
		if !errors.Is(err, ErrMethodMissing) {
			t.Errorf("callMethod() error = %v, want ErrMethodMissing", err)
		}
	})

	t.Run("arg count mismatch", func(t *testing.T) {
		_, err := callMethod(Calculator{}, "Add", 1)
		if !errors.Is(err, ErrArgCount) {
			t.Errorf("callMethod() error = %v, want ErrArgCount", err)
		}
	})
}

func Example_describeType() {
	fmt.Println(describeType(3.14))
	fmt.Println(describeType(User{}))
	fmt.Println(describeType([]int{}))
	// Output:
	// float64(float64)
	// User(struct)
	// []int(slice)
}
