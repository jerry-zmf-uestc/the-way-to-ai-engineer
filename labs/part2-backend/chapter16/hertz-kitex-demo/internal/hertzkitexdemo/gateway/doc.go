// Package gateway hosts the Hertz HTTP gateway that fronts the Kitex RPC
// chain. It wires middleware (inject trace-id / recover / logging), maps
// sentinel errors to HTTP status codes, and calls UserService via a Kitex
// client that carries timeout + retry policies from internal/config.
//
// Real implementation lands in task 2.6.
package gateway
