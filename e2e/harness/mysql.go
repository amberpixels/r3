package harness

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// MySQL is a running throwaway MySQL container.
type MySQL struct {
	Container testcontainers.Container
	Host      string
	Port      string
}

// StartMySQL boots mysql:8 and waits until it accepts connections. The
// entrypoint restarts the server during init (the temp init server runs with
// networking disabled) and Docker's port proxy accepts connections early, so the
// final "ready" log line is the signal.
func StartMySQL(ctx context.Context) (*MySQL, error) {
	req := testcontainers.ContainerRequest{
		Image:        "mysql:8",
		ExposedPorts: []string{"3306/tcp"},
		Env: map[string]string{
			"MYSQL_ROOT_PASSWORD": Password,
			"MYSQL_USER":          User,
			"MYSQL_PASSWORD":      Password,
			"MYSQL_DATABASE":      Database,
		},
		WaitingFor: wait.ForAll(
			wait.ForLog("port: 3306  MySQL Community Server"),
			wait.ForListeningPort("3306/tcp"),
		).WithDeadline(120 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("start mysql container: %w", err)
	}

	host, port, err := hostPort(ctx, container, "3306")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, err
	}

	return &MySQL{Container: container, Host: host, Port: port}, nil
}

// DSN is the go-sql-driver/mysql connection string.
func (m *MySQL) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&multiStatements=true",
		User, Password, m.Host, m.Port, Database)
}

// Terminate stops the container, ignoring the error a test can do nothing about.
func (m *MySQL) Terminate() {
	if m != nil && m.Container != nil {
		_ = m.Container.Terminate(context.Background())
	}
}

// ErrMySQLUnreachable is returned when MySQL never answers a ping in time.
var ErrMySQLUnreachable = errors.New("failed to ping MySQL after retries")

// WaitReady pings until MySQL answers. The readiness log fires slightly before
// the server takes connections, so a bounded retry sits between the two.
func WaitReady(ctx context.Context, ping func(context.Context) error) error {
	var err error
	for range 30 {
		if err = ping(ctx); err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("%w: %w", ErrMySQLUnreachable, err)
}
