package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

func TestMixedResponseEnvelopeClosesSession(t *testing.T) {
	for _, field := range []string{`"method":"tools/call",`, `"params":{},`} {
		t.Run(field, func(t *testing.T) {
			serverInput, clientInput := io.Pipe()
			clientOutput, serverOutput := io.Pipe()
			defer serverInput.Close()
			defer serverOutput.Close()
			client, err := NewClient(clientInput, clientOutput, "docs", nil)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			finished := make(chan struct{})
			go func() {
				defer close(finished)
				buffer := make([]byte, 4096)
				_, _ = serverInput.Read(buffer)
				_, _ = fmt.Fprintln(serverOutput, `{"jsonrpc":"2.0","id":1,`+field+`"result":{}}`)
			}()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if _, err := client.exchange(ctx, "initialize", map[string]any{}); !errors.Is(err, ErrProtocol) || !client.closed {
				t.Fatal("mixed envelope accepted", err)
			}
			<-finished
		})
	}
}
func TestServerRejectsResponseFieldsInRequest(t *testing.T) {
	for _, field := range []string{`"result":{},`, `"error":{"code":-1,"message":"fake"},`} {
		packet := `{"jsonrpc":"2.0","id":1,"method":"initialize",` + field + `"params":{}}` + "\n"
		var output bytes.Buffer
		if err := Serve(strings.NewReader(packet), &output); !errors.Is(err, ErrProtocol) || output.Len() != 0 {
			t.Fatal("request consumed response fields", err, output.String())
		}
	}
}
