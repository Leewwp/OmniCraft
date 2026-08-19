package clamav

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestClientPingUsesPrivateClamdProtocol(t *testing.T) {
	listener := listenTestClamd(t, func(conn net.Conn) error {
		command, err := readNulCommand(conn)
		if err != nil {
			return err
		}
		if command != "zPING" {
			return errors.New("unexpected command: " + command)
		}
		_, err = io.WriteString(conn, "PONG\n")
		return err
	})

	client := NewClient(listener.Addr().String(), time.Second)
	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
}

func TestNewClientPreservesConfiguredTimeout(t *testing.T) {
	client := NewClient("clamav:3310", 0)
	if client.timeout != 0 {
		t.Fatalf("timeout = %s, want configured zero value", client.timeout)
	}
}

func TestClientVersionParsesEngineAndSignatureVersions(t *testing.T) {
	listener := listenTestClamd(t, func(conn net.Conn) error {
		command, err := readNulCommand(conn)
		if err != nil {
			return err
		}
		if command != "zVERSION" {
			return errors.New("unexpected command: " + command)
		}
		_, err = io.WriteString(conn, "ClamAV 1.4.3/27608/2026-08-19\n")
		return err
	})

	version, err := NewClient(listener.Addr().String(), time.Second).Version(context.Background())
	if err != nil {
		t.Fatalf("Version() error = %v", err)
	}
	if version.Engine != "ClamAV 1.4.3" || version.Signatures != "27608" {
		t.Fatalf("version = %+v", version)
	}
}

func TestClientScanStreamsChunksAndParsesDetection(t *testing.T) {
	listener := listenTestClamd(t, func(conn net.Conn) error {
		command, err := readNulCommand(conn)
		if err != nil {
			return err
		}
		if command != "zINSTREAM" {
			return errors.New("unexpected command: " + command)
		}
		var payload strings.Builder
		for {
			var size uint32
			if err := binary.Read(conn, binary.BigEndian, &size); err != nil {
				return err
			}
			if size == 0 {
				break
			}
			chunk := make([]byte, size)
			if _, err := io.ReadFull(conn, chunk); err != nil {
				return err
			}
			payload.Write(chunk)
		}
		if payload.String() != "hello clamav" {
			return errors.New("stream payload mismatch")
		}
		_, err = io.WriteString(conn, "stream: Eicar-Test-Signature FOUND\n")
		return err
	})

	client := NewClient(listener.Addr().String(), time.Second)
	result, err := client.Scan(context.Background(), strings.NewReader("hello clamav"))
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if result.Status != StatusBlocked || result.DetectionName != "Eicar-Test-Signature" {
		t.Fatalf("result = %+v, want blocked Eicar-Test-Signature", result)
	}
}

func TestClientRejectsUnboundedResponse(t *testing.T) {
	listener := listenTestClamd(t, func(conn net.Conn) error {
		if _, err := readNulCommand(conn); err != nil {
			return err
		}
		for {
			var size uint32
			if err := binary.Read(conn, binary.BigEndian, &size); err != nil {
				return err
			}
			if size == 0 {
				break
			}
			if _, err := io.CopyN(io.Discard, conn, int64(size)); err != nil {
				return err
			}
		}
		_, err := io.WriteString(conn, strings.Repeat("x", maxResponseBytes+1))
		return err
	})

	client := NewClient(listener.Addr().String(), time.Second)
	if _, err := client.Scan(context.Background(), strings.NewReader("payload")); !errors.Is(err, ErrResponseTooLong) {
		t.Fatalf("Scan() error = %v, want ErrResponseTooLong", err)
	}
}

func TestClientMapsClamdDisconnectToProtocolError(t *testing.T) {
	listener := listenTestClamd(t, func(conn net.Conn) error {
		_, err := readNulCommand(conn)
		return err
	})

	if err := NewClient(listener.Addr().String(), time.Second).Ping(context.Background()); !errors.Is(err, ErrProtocol) {
		t.Fatalf("Ping() error = %v, want ErrProtocol after clamd disconnect", err)
	}
}

func TestClientScanHonorsContextTimeout(t *testing.T) {
	listener := listenTestClamd(t, func(conn net.Conn) error {
		if _, err := readNulCommand(conn); err != nil {
			return err
		}
		for {
			var size uint32
			if err := binary.Read(conn, binary.BigEndian, &size); err != nil {
				return err
			}
			if size == 0 {
				break
			}
			if _, err := io.CopyN(io.Discard, conn, int64(size)); err != nil {
				return err
			}
		}
		select {}
	})

	client := NewClient(listener.Addr().String(), time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := client.Scan(ctx, strings.NewReader("payload")); !errors.Is(err, ErrScanTimeout) {
		t.Fatalf("Scan() error = %v, want ErrScanTimeout", err)
	}
}

func listenTestClamd(t *testing.T, handler func(net.Conn) error) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		if err := handler(conn); err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			t.Errorf("fake clamd: %v", err)
		}
	}()
	return listener
}

func readNulCommand(conn net.Conn) (string, error) {
	var command strings.Builder
	var one [1]byte
	for {
		if _, err := io.ReadFull(conn, one[:]); err != nil {
			return command.String(), err
		}
		if one[0] == 0 {
			return command.String(), nil
		}
		command.WriteByte(one[0])
	}
}
