package main

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type Profile struct {
	Name string   `json:"name"`
	Age  int      `json:"age,omitempty"`
	Tags []string `json:"tags,omitempty"`
}

func EncodeProfile(profile Profile) (string, error) {
	data, err := json.Marshal(profile)
	if err != nil {
		return "", fmt.Errorf("marshal profile: %w", err)
	}
	return string(data), nil
}

func DecodeProfile(data []byte) (Profile, error) {
	var profile Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return Profile{}, fmt.Errorf("unmarshal profile: %w", err)
	}
	return profile, nil
}

func DecodeProfileStrict(data []byte) (Profile, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var profile Profile
	if err := decoder.Decode(&profile); err != nil {
		return Profile{}, fmt.Errorf("decode profile strictly: %w", err)
	}
	return profile, nil
}

type BadCase struct {
	F func()
}

func BadEncodeJson() ([]byte, error) {
	bad := BadCase{
		F: func() {},
	}
	return json.Marshal(bad)
}

func main() {
	encoded, err := EncodeProfile(Profile{Name: "Ada", Tags: []string{"go", "json"}})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(encoded)

	profile, err := DecodeProfile([]byte(`{"name":"Ada","unknown":true}`))
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("%+v\n", profile)

	badEncoded, err := BadEncodeJson()
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(string(badEncoded))
}
