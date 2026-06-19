package cache

import (
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestCacheMiss(t *testing.T) {
	c := New(time.Minute)

	resp, ok, err := c.Get(queryMessage(t, 100, "example.com.", dns.TypeA))
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if ok {
		t.Fatal("expected cache miss")
	}
	if resp != nil {
		t.Fatalf("expected nil response on miss, got %d bytes", len(resp))
	}
}

func TestCacheHitRewritesRequestID(t *testing.T) {
	now := time.Unix(100, 0)
	c := New(time.Minute)
	c.now = func() time.Time { return now }

	req := queryMessage(t, 100, "example.com.", dns.TypeA)
	if err := c.Add(req, responseMessage(t, req, "192.0.2.10")); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	got, ok, err := c.Get(queryMessage(t, 200, "example.com.", dns.TypeA))
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected cache hit")
	}

	msg := unpackMessage(t, got)
	if msg.Id != 200 {
		t.Fatalf("expected response ID 200, got %d", msg.Id)
	}
	if len(msg.Answer) != 1 {
		t.Fatalf("expected one answer, got %d", len(msg.Answer))
	}
}

func TestCacheExpiresRecord(t *testing.T) {
	now := time.Unix(100, 0)
	c := New(time.Second)
	c.now = func() time.Time { return now }

	req := queryMessage(t, 100, "example.com.", dns.TypeA)
	if err := c.Add(req, responseMessage(t, req, "192.0.2.10")); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	now = now.Add(2 * time.Second)

	resp, ok, err := c.Get(queryMessage(t, 200, "example.com.", dns.TypeA))
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if ok {
		t.Fatal("expected expired record to miss")
	}
	if resp != nil {
		t.Fatalf("expected nil response after expiration, got %d bytes", len(resp))
	}
	if len(c.db) != 0 {
		t.Fatalf("expected expired record to be pruned, got %d records", len(c.db))
	}
}

func queryMessage(t *testing.T, id uint16, name string, qtype uint16) []byte {
	t.Helper()

	msg := new(dns.Msg)
	msg.SetQuestion(name, qtype)
	msg.Id = id

	packed, err := msg.Pack()
	if err != nil {
		t.Fatalf("pack query: %v", err)
	}
	return packed
}

func responseMessage(t *testing.T, reqBytes []byte, ip string) []byte {
	t.Helper()

	req := unpackMessage(t, reqBytes)
	resp := new(dns.Msg)
	resp.SetReply(req)
	resp.Answer = append(resp.Answer, &dns.A{
		Hdr: dns.RR_Header{
			Name:   req.Question[0].Name,
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    300,
		},
		A: net.ParseIP(ip).To4(),
	})

	packed, err := resp.Pack()
	if err != nil {
		t.Fatalf("pack response: %v", err)
	}
	return packed
}

func unpackMessage(t *testing.T, data []byte) *dns.Msg {
	t.Helper()

	msg := new(dns.Msg)
	if err := msg.Unpack(data); err != nil {
		t.Fatalf("unpack message: %v", err)
	}
	return msg
}
