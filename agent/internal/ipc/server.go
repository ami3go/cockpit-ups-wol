package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"
)

const MaxRequestBytes = 1 << 20

type Server struct {
	Handler Handler
	SocketPath string
	SocketMode os.FileMode
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	if s.SocketPath == "" { return errors.New("socket path is required") }
	if s.SocketMode == 0 { s.SocketMode = 0o660 }
	if err := os.MkdirAll(filepath.Dir(s.SocketPath), 0o750); err != nil { return err }
	_ = os.Remove(s.SocketPath)
	ln, err := net.Listen("unix", s.SocketPath)
	if err != nil { return err }
	defer func(){ _ = ln.Close(); _ = os.Remove(s.SocketPath) }()
	if err := os.Chmod(s.SocketPath, s.SocketMode); err != nil { return err }
	go func(){ <-ctx.Done(); _ = ln.Close() }()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err()!=nil { return nil }
			return err
		}
		go s.serveConn(ctx, conn)
	}
}

func (s *Server) serveConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30*time.Second))
	r := bufio.NewReader(io.LimitReader(conn, MaxRequestBytes+1))
	line, err := r.ReadBytes('\n')
	if err != nil && !errors.Is(err, io.EOF) { return }
	if len(line) > MaxRequestBytes {
		_ = json.NewEncoder(conn).Encode(failure("","INVALID_REQUEST","request too large")); return
	}
	select { case <-ctx.Done(): return; default: }
	req, err := DecodeRequest(line)
	if err != nil { _ = json.NewEncoder(conn).Encode(failure("","INVALID_REQUEST",err.Error())); return }
	_ = json.NewEncoder(conn).Encode(s.Handler.Handle(req))
}

func Call(ctx context.Context, socketPath string, req Request) (Response,error) {
	var resp Response
	d := net.Dialer{}
	conn, err := d.DialContext(ctx,"unix",socketPath)
	if err != nil { return resp, err }
	defer conn.Close()
	if err := json.NewEncoder(conn).Encode(req); err != nil { return resp, err }
	if err := json.NewDecoder(io.LimitReader(conn, MaxRequestBytes)).Decode(&resp); err != nil { return resp, err }
	if resp.ID != req.ID && resp.ID!="" { return resp, fmt.Errorf("response id mismatch: got %q want %q",resp.ID,req.ID) }
	return resp,nil
}
