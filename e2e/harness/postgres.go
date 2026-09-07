package harness

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Postgres is a running throwaway PostgreSQL container. It hands out the address
// in the three shapes the drivers want and leaves connecting to the caller,
// since each driver opens it differently (database/sql, gorm, bun, go-pg).
type Postgres struct {
	Container testcontainers.Container
	Host      string
	Port      string
}

// StartPostgres boots postgres:18-alpine and waits until it actually accepts
// connections. The entrypoint boots a throwaway server for initdb before the
// real one, and Docker's port proxy accepts connections before the DB is up, so
// the second "ready" log line is the signal, not the open port.
func StartPostgres(ctx context.Context) (*Postgres, error) {
	req := testcontainers.ContainerRequest{
		Image:        "postgres:18-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     User,
			"POSTGRES_PASSWORD": Password,
			"POSTGRES_DB":       Database,
		},
		WaitingFor: wait.ForAll(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
			wait.ForListeningPort("5432/tcp"),
		).WithDeadline(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("start postgres container: %w", err)
	}

	host, port, err := hostPort(ctx, container, "5432")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, err
	}

	return &Postgres{Container: container, Host: host, Port: port}, nil
}

// Addr is "host:port", what go-pg's Options takes.
func (p *Postgres) Addr() string { return net.JoinHostPort(p.Host, p.Port) }

// KeywordDSN is the "host=... port=..." form lib/pq and gorm's postgres driver take.
func (p *Postgres) KeywordDSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		p.Host, p.Port, User, Password, Database)
}

// URLDSN is the URL form pgx prefers.
func (p *Postgres) URLDSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", User, Password, p.Addr(), Database)
}

// Terminate stops the container, ignoring the error a test can do nothing about.
func (p *Postgres) Terminate() {
	if p != nil && p.Container != nil {
		_ = p.Container.Terminate(context.Background())
	}
}

// hostPort resolves a started container's mapped host and port.
func hostPort(ctx context.Context, c testcontainers.Container, port string) (string, string, error) {
	host, err := c.Host(ctx)
	if err != nil {
		return "", "", fmt.Errorf("container host: %w", err)
	}
	mapped, err := c.MappedPort(ctx, port)
	if err != nil {
		return "", "", fmt.Errorf("container port %s: %w", port, err)
	}
	return host, mapped.Port(), nil
}
