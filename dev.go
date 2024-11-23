//go:build never
// +build never

package noprune

import (
	_ "github.com/bool64/dev"           // Include CI/Dev scripts to project.
	_ "github.com/dohernandez/dev-grpc" // Include development grpc helpers to project.
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway"
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2"
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
)
