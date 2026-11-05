package repository

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"net"
	"os"
	"os/exec"
	"time"
)

// Native PostgreSQL remains the default. The optional stdio adapter runs the
// same repository SQL in isolated PostgreSQL WASM when OS IPC is unavailable.
func integrationPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	path := os.Getenv("REEF_PGLITE_RUNNER")
	if path == "" {
		return pgxpool.New(ctx, dsn)
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	config.MaxConns = 1
	config.ConnConfig.DialFunc = func(ctx context.Context, _, _ string) (net.Conn, error) {
		cmd := exec.Command("node", path)
		cmd.Stderr = os.Stderr
		in, err := cmd.StdinPipe()
		if err != nil {
			return nil, err
		}
		out, err := cmd.StdoutPipe()
		if err != nil {
			return nil, err
		}
		if err = cmd.Start(); err != nil {
			return nil, err
		}
		return &pipeConn{Reader: out, Writer: in, cmd: cmd}, nil
	}
	return pgxpool.NewWithConfig(ctx, config)
}

type pipeConn struct {
	io.Reader
	io.Writer
	cmd *exec.Cmd
}

func (c *pipeConn) Close() error {
	_ = c.Writer.(io.Closer).Close()
	_ = c.Reader.(io.Closer).Close()
	_ = c.cmd.Process.Kill()
	return c.cmd.Wait()
}
func (c *pipeConn) LocalAddr() net.Addr              { return pipeAddr("local") }
func (c *pipeConn) RemoteAddr() net.Addr             { return pipeAddr("embedded") }
func (c *pipeConn) SetDeadline(time.Time) error      { return nil }
func (c *pipeConn) SetReadDeadline(time.Time) error  { return nil }
func (c *pipeConn) SetWriteDeadline(time.Time) error { return nil }

type pipeAddr string

func (p pipeAddr) Network() string { return "stdio" }
func (p pipeAddr) String() string  { return string(p) }
