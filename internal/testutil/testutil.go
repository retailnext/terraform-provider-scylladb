// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package testutil

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// NewTestContainer starts a ScyllaDB container and returns the host:port string.
// The container is automatically cleaned up when the test finishes.
func NewTestContainer(t *testing.T) string {
	ctx := context.Background()

	// Get the config
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	scyllaConfig, err := filepath.Abs(filepath.Join(dir, "testdata", "scylla.yaml"))
	require.NoError(t, err)
	scyllaDevContainer, err := testcontainers.Run(
		ctx, "scylladb/scylla:2025.4.1",
		testcontainers.WithCmdArgs("--smp", "1", "--overprovisioned", "1"),
		testcontainers.WithExposedPorts("9042/tcp"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("9042/tcp"),
			// wait.ForLog("Ready to accept connections"),
		),
		testcontainers.WithFiles(testcontainers.ContainerFile{
			HostFilePath:      scyllaConfig,
			ContainerFilePath: "/etc/scylla/scylla.yaml",
			FileMode:          0o777,
		}),
		// Commented out log consumer to reduce noise during tests
		// testcontainers.WithLogConsumerConfig(&testcontainers.LogConsumerConfig{
		// 	Opts:      []testcontainers.LogProductionOption{testcontainers.WithLogProductionTimeout(10 * time.Second)},
		// 	Consumers: []testcontainers.LogConsumer{&StdoutLogConsumer{}},
		// }),
	)
	if err != nil {
		t.Fatalf("failed to start the scylla container: %s", err)
	}

	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(scyllaDevContainer); err != nil {
			t.Fatalf("failed to terminate the scylla container: %s", err)
		}
	})

	host, err := scyllaDevContainer.PortEndpoint(ctx, "9042", "")
	if err != nil {
		t.Fatalf("failed to get the scylla container endpoint: %s", err)
	}
	return host
}

// StdoutLogConsumer is a LogConsumer that prints the log to stdout.
type StdoutLogConsumer struct{}

// Accept prints the log to stdout.
func (lc *StdoutLogConsumer) Accept(l testcontainers.Log) {
	fmt.Print(string(l.Content))
}
