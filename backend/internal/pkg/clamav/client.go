// Package clamav contains the private, minimal clamd protocol adapter used by
// the archive scan worker. It intentionally exposes no network protocol
// details to handlers or clients.
package clamav

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

const (
	streamChunkSize  = 32 * 1024
	maxResponseBytes = 4096
)

var (
	ErrUnavailable     = errors.New("clamd unavailable")
	ErrProtocol        = errors.New("clamd protocol error")
	ErrResponseTooLong = errors.New("clamd response too long")
	ErrScanTimeout     = errors.New("clamd scan timeout")
	ErrScanFailed      = errors.New("clamd scan failed")
)

type Status string

const (
	StatusClean   Status = "clean"
	StatusBlocked Status = "blocked"
)

type Result struct {
	Status        Status
	DetectionName string
}

type Version struct {
	Engine     string
	Signatures string
}

type Client struct {
	address string
	timeout time.Duration
}

func NewClient(address string, timeout time.Duration) *Client {
	return &Client{address: strings.TrimSpace(address), timeout: timeout}
}

func (c *Client) Ping(ctx context.Context) error {
	return c.withConnection(ctx, func(conn net.Conn) error {
		if err := writeCommand(conn, "zPING"); err != nil {
			return err
		}
		response, err := readResponse(conn)
		if err != nil {
			return err
		}
		if strings.TrimSpace(response) != "PONG" {
			return ErrProtocol
		}
		return nil
	})
}

func (c *Client) Version(ctx context.Context) (Version, error) {
	var version Version
	err := c.withConnection(ctx, func(conn net.Conn) error {
		if err := writeCommand(conn, "zVERSION"); err != nil {
			return err
		}
		response, err := readResponse(conn)
		if err != nil {
			return err
		}
		fields := strings.Fields(response)
		if len(fields) < 2 {
			return ErrProtocol
		}
		version.Engine = fields[0] + " " + fields[1]
		if parts := strings.Split(fields[1], "/"); len(parts) > 1 {
			version.Engine = fields[0] + " " + parts[0]
			version.Signatures = parts[1]
		}
		return nil
	})
	return version, err
}

func (c *Client) Scan(ctx context.Context, source io.Reader) (Result, error) {
	if source == nil {
		return Result{}, ErrProtocol
	}
	var result Result
	err := c.withConnection(ctx, func(conn net.Conn) error {
		if err := writeCommand(conn, "zINSTREAM"); err != nil {
			return err
		}
		buffer := make([]byte, streamChunkSize)
		for {
			read, readErr := source.Read(buffer)
			if read > 0 {
				if err := writeChunk(conn, buffer[:read]); err != nil {
					return err
				}
			}
			if readErr != nil {
				if errors.Is(readErr, io.EOF) {
					break
				}
				return fmt.Errorf("read scan object: %w", readErr)
			}
		}
		if err := writeChunk(conn, nil); err != nil {
			return err
		}
		response, err := readResponse(conn)
		if err != nil {
			return err
		}
		parsed, err := parseScanResponse(response)
		if err != nil {
			return err
		}
		result = parsed
		return nil
	})
	return result, err
}

func (c *Client) withConnection(ctx context.Context, fn func(net.Conn) error) (operationErr error) {
	if c == nil || c.address == "" {
		return ErrUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", c.address)
	if err != nil {
		if ctx.Err() != nil {
			return ErrScanTimeout
		}
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer conn.Close()
	deadline := time.Now().Add(c.timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := conn.SetDeadline(deadline); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			return ErrUnavailable
		}
		return ErrUnavailable
	}
	done := make(chan struct{})
	closeResult := make(chan error, 1)
	go func() {
		select {
		case <-ctx.Done():
			closeResult <- conn.Close()
		case <-done:
			closeResult <- nil
		}
	}()
	err = fn(conn)
	close(done)
	if closeErr := <-closeResult; closeErr != nil && ctx.Err() == nil && err == nil {
		return ErrUnavailable
	}
	if closeErr := conn.Close(); closeErr != nil && ctx.Err() == nil && err == nil {
		return ErrUnavailable
	}
	if ctx.Err() != nil {
		return ErrScanTimeout
	}
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return ErrScanTimeout
	}
	if err != nil {
		return err
	}
	return nil
}

func writeCommand(conn net.Conn, command string) error {
	_, err := writeFull(conn, append([]byte(command), 0))
	if err != nil {
		return fmt.Errorf("%w: write command", ErrUnavailable)
	}
	return nil
}

func writeChunk(conn net.Conn, chunk []byte) error {
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(chunk)))
	if _, err := writeFull(conn, header[:]); err != nil {
		return fmt.Errorf("%w: write chunk header", ErrUnavailable)
	}
	if len(chunk) == 0 {
		return nil
	}
	if _, err := writeFull(conn, chunk); err != nil {
		return fmt.Errorf("%w: write chunk", ErrUnavailable)
	}
	return nil
}

func writeFull(writer io.Writer, payload []byte) (int, error) {
	written, err := io.Copy(writer, bytes.NewReader(payload))
	return int(written), err
}

func readResponse(conn net.Conn) (string, error) {
	response := make([]byte, 0, 128)
	buffer := make([]byte, 256)
	for len(response) <= maxResponseBytes {
		read, err := conn.Read(buffer)
		if read > 0 {
			response = append(response, buffer[:read]...)
			if len(response) > maxResponseBytes {
				return "", ErrResponseTooLong
			}
			if index := strings.IndexByte(string(response), '\n'); index >= 0 {
				return strings.TrimSpace(string(response[:index])), nil
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return "", ErrProtocol
			}
			return "", err
		}
	}
	return "", ErrResponseTooLong
}

func parseScanResponse(response string) (Result, error) {
	response = strings.TrimSpace(response)
	if !strings.HasPrefix(response, "stream: ") {
		return Result{}, ErrProtocol
	}
	payload := strings.TrimSpace(strings.TrimPrefix(response, "stream: "))
	if payload == "OK" {
		return Result{Status: StatusClean}, nil
	}
	if strings.HasSuffix(payload, " FOUND") {
		return Result{Status: StatusBlocked, DetectionName: strings.TrimSpace(strings.TrimSuffix(payload, " FOUND"))}, nil
	}
	if strings.HasSuffix(payload, " ERROR") || payload == "ERROR" {
		return Result{}, ErrScanFailed
	}
	return Result{}, ErrProtocol
}
