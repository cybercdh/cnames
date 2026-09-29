package main

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestDomainFromLine(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"www.example.com", "www.example.com", true},
		{"  www.example.com.  ", "www.example.com", true},
		{"https://www.example.com/path", "www.example.com", true},
		{"", "", false},
		{"   ", "", false},
		{"# note", "", false},
		{".", "", false},
	}
	for _, c := range cases {
		got, ok := domainFromLine(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("domainFromLine(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

// serve starts a local UDP+TCP DNS server whose handler is h and returns its address.
func serve(t *testing.T, h dns.HandlerFunc) string {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := pc.LocalAddr().String()
	l, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	us := &dns.Server{PacketConn: pc, Handler: h}
	ts := &dns.Server{Listener: l, Handler: h}
	go us.ActivateAndServe()
	go ts.ActivateAndServe()
	t.Cleanup(func() { us.Shutdown(); ts.Shutdown() })
	return addr
}

func clients() (*dns.Client, *dns.Client) {
	return &dns.Client{Timeout: 2 * time.Second}, &dns.Client{Net: "tcp", Timeout: 2 * time.Second}
}

func TestLookupCNAME_NonCNAMEAnswerDoesNotPanic(t *testing.T) {
	addr := serve(t, func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		// an A record where the old code blindly asserted *dns.CNAME
		a := &dns.A{Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.IPv4(192, 0, 2, 1)}
		c := &dns.CNAME{Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 60}, Target: "target.example."}
		m.Answer = []dns.RR{a, c}
		w.WriteMsg(m)
	})
	udp, tcp := clients()
	got, err := lookupCNAME(udp, tcp, "www.example.com", addr)
	if err != nil || got != "target.example" {
		t.Fatalf("got (%q, %v), want (target.example, nil)", got, err)
	}
}

func TestLookupCNAME_NoCNAMEAndNXDOMAIN(t *testing.T) {
	addr := serve(t, func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		if strings.HasPrefix(r.Question[0].Name, "missing.") {
			m.Rcode = dns.RcodeNameError
		}
		w.WriteMsg(m)
	})
	udp, tcp := clients()
	for _, d := range []string{"plain.example.com", "missing.example.com"} {
		got, err := lookupCNAME(udp, tcp, d, addr)
		if err != nil || got != "" {
			t.Errorf("%s: got (%q, %v), want (\"\", nil)", d, got, err)
		}
	}
}

func TestLookupCNAME_ServfailIsError(t *testing.T) {
	addr := serve(t, func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Rcode = dns.RcodeServerFailure
		w.WriteMsg(m)
	})
	udp, tcp := clients()
	if _, err := lookupCNAME(udp, tcp, "x.example.com", addr); err == nil {
		t.Fatal("expected an error for SERVFAIL")
	}
}

func TestLookupCNAME_TruncatedRetriesOverTCP(t *testing.T) {
	addr := serve(t, func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		if _, isUDP := w.RemoteAddr().(*net.UDPAddr); isUDP {
			m.Truncated = true
			w.WriteMsg(m)
			return
		}
		m.Answer = []dns.RR{&dns.CNAME{Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 60}, Target: "via-tcp.example."}}
		w.WriteMsg(m)
	})
	udp, tcp := clients()
	got, err := lookupCNAME(udp, tcp, "big.example.com", addr)
	if err != nil || got != "via-tcp.example" {
		t.Fatalf("got (%q, %v), want (via-tcp.example, nil)", got, err)
	}
}
