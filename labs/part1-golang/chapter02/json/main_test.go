package main

import "testing"

func TestEncodeProfileOmitsEmptyFields(t *testing.T) {
	got, err := EncodeProfile(Profile{Name: "Ada"})
	if err != nil {
		t.Fatalf("EncodeProfile() error = %v", err)
	}
	want := `{"name":"Ada"}`
	if got != want {
		t.Fatalf("EncodeProfile() = %s, want %s", got, want)
	}
}

func TestDecodeProfileAppliesZeroValuesForMissingFields(t *testing.T) {
	got, err := DecodeProfile([]byte(`{"name":"Ada","unknown":true}`))
	if err != nil {
		t.Fatalf("DecodeProfile() error = %v", err)
	}
	if got.Name != "Ada" || got.Age != 0 || len(got.Tags) != 0 {
		t.Fatalf("DecodeProfile() = %+v, want name Ada with zero-value optional fields", got)
	}
}

func TestDecodeProfileStrictRejectsUnknownFields(t *testing.T) {
	if _, err := DecodeProfileStrict([]byte(`{"name":"Ada","unknown":true}`)); err == nil {
		t.Fatal("DecodeProfileStrict() error = nil, want error")
	}
}

func TestDecodeProfileRejectsInvalidJSON(t *testing.T) {
	if _, err := DecodeProfile([]byte(`{"name":`)); err == nil {
		t.Fatal("DecodeProfile() error = nil, want error")
	}
}

func TestBadEncodeJSONRejectsUnsupportedFuncField(t *testing.T) {
	if _, err := BadEncodeJson(); err == nil {
		t.Fatal("BadEncodeJson() error = nil, want error")
	}
}
