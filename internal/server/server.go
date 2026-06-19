package server

import (
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"time"

	"github.com/isa0-gh/resolv/internal/service"
	"github.com/miekg/dns"
)

type Server struct {
	repo *service.ServiceRepo
}

func New(repo *service.ServiceRepo) *Server {
	return &Server{repo: repo}
}

func (s *Server) HandleUDPConn(data []byte, addr *net.UDPAddr, conn *net.UDPConn) {
	resp, err := s.processQuery(data)
	if err != nil {
		slog.Error("ERROR processing query over UDP", "error", err)
		return
	}

	_, err = conn.WriteToUDP(resp, addr)
	if err != nil {
		slog.Error("ERROR writing to udp", "error", err)
		return
	}
}

func (s *Server) HandleTCPConn(conn net.Conn) {
	defer conn.Close()

	for {
		var length uint16
		if err := binary.Read(conn, binary.BigEndian, &length); err != nil {
			return // Connection closed or timeout
		}

		data := make([]byte, length)
		if _, err := io.ReadFull(conn, data); err != nil {
			return
		}

		resp, err := s.processQuery(data)
		if err != nil {
			slog.Error("ERROR processing query over TCP", "error", err)
			return
		}

		respLen := uint16(len(resp))
		if err := binary.Write(conn, binary.BigEndian, respLen); err != nil {
			return
		}

		if _, err := conn.Write(resp); err != nil {
			return
		}
	}
}

func (s *Server) processQuery(data []byte) ([]byte, error) {
	if localResp, ok := s.repo.Local.Match(data); ok {
		return localResp, nil
	}

	cached, ok, err := s.repo.Cache.Get(data)
	if err == nil && ok {
		return cached, nil
	}

	resp, err := s.repo.Resolver.Resolve(data)
	if err != nil {
		slog.Error("ERROR resolving dns message", "error", err)
		if sfMsg := servfailResponse(data); sfMsg != nil {
			return sfMsg, nil
		}
		return nil, err
	}

	if err := s.repo.Cache.Add(data, resp); err != nil {
		slog.Error("CACHE ERROR", "error", err)
	}

	return resp, nil
}

func servfailResponse(query []byte) []byte {
	req := new(dns.Msg)
	if err := req.Unpack(query); err != nil {
		return nil
	}
	resp := new(dns.Msg)
	resp.SetRcode(req, dns.RcodeServerFailure)
	packed, err := resp.Pack()
	if err != nil {
		return nil
	}
	return packed
}

func (s *Server) Run() error {
	// Start UDP server
	udpAddr, err := net.ResolveUDPAddr("udp", s.repo.Config.BindAddress)
	if err != nil {
		return err
	}
	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	defer udpConn.Close()

	// Start TCP server
	tcpListener, err := net.Listen("tcp", s.repo.Config.BindAddress)
	if err != nil {
		return err
	}
	defer tcpListener.Close()

	slog.Info("resolv started.", "resolver", s.repo.Config.Resolver, "listen", s.repo.Config.BindAddress, "config", s.repo.Config)

	s.repo.Cache.StartFlusher(time.Duration(s.repo.Config.TTL) * time.Second)

	errChan := make(chan error, 2)

	go func() {
		buf := make([]byte, 4096)
		for {
			size, clientAddr, err := udpConn.ReadFromUDP(buf)
			if err != nil {
				errChan <- err
				return
			}
			request := make([]byte, size)
			copy(request, buf[:size])
			go s.HandleUDPConn(request, clientAddr, udpConn)
		}
	}()

	go func() {
		for {
			conn, err := tcpListener.Accept()
			if err != nil {
				errChan <- err
				return
			}
			go s.HandleTCPConn(conn)
		}
	}()

	return <-errChan
}
