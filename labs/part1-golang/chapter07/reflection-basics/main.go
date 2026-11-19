// Command reflection-basics demonstrates the four core capabilities of Go
// reflection: type introspection, struct field/tag traversal, mutating values
// through a pointer, and dynamic method invocation.
package main

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// Sentinel errors let callers use errors.Is instead of string comparison.
var (
	ErrNotStruct     = errors.New("reflection: value is not a struct")
	ErrNotPointer    = errors.New("reflection: value is not a non-nil pointer")
	ErrFieldMissing  = errors.New("reflection: field not found")
	ErrFieldType     = errors.New("reflection: field is not a string")
	ErrNotSettable   = errors.New("reflection: field is not settable")
	ErrMethodMissing = errors.New("reflection: method not found")
	ErrArgCount      = errors.New("reflection: argument count mismatch")
)

// User demonstrates struct tag traversal and field mutation. The unexported
// role field shows that reflection cannot set unexported fields.
type User struct {
	Name string `json:"name" vd:"len($)>0"`
	Age  int    `json:"age"`
	role string
}

// Calculator demonstrates dynamic method invocation by name.
type Calculator struct{}

// Add returns the sum of a and b.
func (Calculator) Add(a, b int) int { return a + b }

// describeType returns a "TypeName(Kind)" description of v. Type is the concrete
// named type while Kind is the underlying category, e.g. a defined type over int
// has Kind int. A nil interface reports "<nil>".
func describeType(v any) string {
	t := reflect.TypeOf(v)
	if t == nil {
		return "<nil>"
	}
	name := t.Name()
	if name == "" {
		// Composite types such as []int have no name; fall back to String().
		name = t.String()
	}
	return fmt.Sprintf("%s(%s)", name, t.Kind())
}

// fieldTags walks the exported fields of struct v (or a pointer to a struct) and
// returns "Field=tagValue" entries for the given struct tag key. It returns
// ErrNotStruct when v is not a struct.
func fieldTags(v any, tagKey string) ([]string, error) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil, fmt.Errorf("field tags for %s: %w", rv.Kind(), ErrNotStruct)
	}

	t := rv.Type()
	tags := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		tags = append(tags, fmt.Sprintf("%s=%s", field.Name, field.Tag.Get(tagKey)))
	}
	return tags, nil
}

// setString sets the named string field of the struct pointed to by ptr. ptr
// must be a non-nil pointer to a struct; the field must exist, be a string, and
// be settable (exported). It reports a wrapped sentinel error otherwise.
func setString(ptr any, field, value string) error {
	rv := reflect.ValueOf(ptr)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("set %q: %w", field, ErrNotPointer)
	}

	elem := rv.Elem()
	if elem.Kind() != reflect.Struct {
		return fmt.Errorf("set %q: %w", field, ErrNotStruct)
	}

	fv := elem.FieldByName(field)
	if !fv.IsValid() {
		return fmt.Errorf("set %q: %w", field, ErrFieldMissing)
	}
	if fv.Kind() != reflect.String {
		return fmt.Errorf("set %q: %w", field, ErrFieldType)
	}
	if !fv.CanSet() {
		// Unexported fields are addressable but not settable.
		return fmt.Errorf("set %q: %w", field, ErrNotSettable)
	}

	fv.SetString(value)
	return nil
}

// callMethod invokes the named method on receiver with args and returns the
// results as a slice. It reports ErrMethodMissing or ErrArgCount on mismatch.
func callMethod(receiver any, method string, args ...any) ([]any, error) {
	m := reflect.ValueOf(receiver).MethodByName(method)
	if !m.IsValid() {
		return nil, fmt.Errorf("call %q: %w", method, ErrMethodMissing)
	}
	if m.Type().NumIn() != len(args) {
		return nil, fmt.Errorf("call %q: want %d args, got %d: %w",
			method, m.Type().NumIn(), len(args), ErrArgCount)
	}

	in := make([]reflect.Value, len(args))
	for i, arg := range args {
		in[i] = reflect.ValueOf(arg)
	}

	out := m.Call(in)
	results := make([]any, len(out))
	for i, rv := range out {
		results[i] = rv.Interface()
	}
	return results, nil
}

func main() {
	// 1. Type vs Kind.
	fmt.Println(describeType(3.14))
	fmt.Println(describeType(User{}))
	fmt.Println(describeType([]int{}))

	// 2. Struct tag traversal.
	tags, err := fieldTags(User{}, "json")
	if err != nil {
		fmt.Println("fieldTags error:", err)
		return
	}
	fmt.Printf("json tags: [%s]\n", strings.Join(tags, " "))

	// 3. Mutate through a pointer.
	u := User{Name: "old", Age: 20}
	if err := setString(&u, "Name", "new"); err != nil {
		fmt.Println("setString error:", err)
		return
	}
	fmt.Printf("after set: name=%s age=%d\n", u.Name, u.Age)

	// 4. Dynamic method call.
	results, err := callMethod(Calculator{}, "Add", 3, 4)
	if err != nil {
		fmt.Println("callMethod error:", err)
		return
	}
	fmt.Printf("Add(3,4) = %v\n", results[0])
}
