package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

type check struct {
	Domain     string
	Nameserver string
}

// a list of dns servers to randomly choose from
var dnsServers = []string{
	"1.1.1.1:53",
	"1.0.0.1:53",
	"8.8.8.8:53",
	"8.8.4.4:53",
	"9.9.9.9:53",
}

func main() {
	var concurrency int
	flag.IntVar(&concurrency, "c", 20, "set the concurrency level")

	var verbose bool
	flag.BoolVar(&verbose, "v", false, "display domain : cname")

	flag.Parse()

	if concurrency < 1 {
		fmt.Fprintf(os.Stderr, "[!] -c must be at least 1 (got %d)\n", concurrency)
		os.Exit(2)
	}

	udp := &dns.Client{Timeout: 5 * time.Second}
	tcp := &dns.Client{Net: "tcp", Timeout: 5 * time.Second}

	// use a buffered channel to prevent blocking
	checks := make(chan check, 100)

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range checks {
				cname, err := lookupCNAME(udp, tcp, c.Domain, c.Nameserver)
				if err != nil {
					// one retry against a different resolver before giving up
					cname, err = lookupCNAME(udp, tcp, c.Domain, otherServer(c.Nameserver))
				}
				if err != nil {
					fmt.Fprintf(os.Stderr, "%s: %s\n", c.Domain, err)
					continue
				}
				if cname == "" {
					continue
				}
				if verbose {
					fmt.Printf("%s : %s\n", c.Domain, cname)
				} else {
					fmt.Println(cname)
				}
			}
		}()
	}

	// read user input either piped to the program
	// or from a single argument
	var input io.Reader = os.Stdin
	if arg := flag.Arg(0); arg != "" {
		input = strings.NewReader(arg)
	}

	// send input to the channel
	// and randomly choose a server from the list
	// to help prevent timeouts
	sc := bufio.NewScanner(input)
	for sc.Scan() {
		domain, ok := domainFromLine(sc.Text())
		if !ok {
			continue
		}
		checks <- check{domain, dnsServers[rand.Intn(len(dnsServers))]}
	}

	// tidy up
	close(checks)
	if err := sc.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "[!] failed to read input: %s\n", err)
	}
	wg.Wait()
}

// domainFromLine normalises one input line to a bare domain: trimmed, trailing
// dot removed, and the host pulled out of a URL if one was given. Blank lines
// and comments are skipped. Previously a blank line was sent as a query for ".".
func domainFromLine(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", false
	}
	if strings.Contains(line, "://") {
		u, err := url.Parse(line)
		if err != nil || u.Hostname() == "" {
			return "", false
		}
		line = u.Hostname()
	}
	line = strings.TrimSuffix(line, ".")
	if line == "" {
		return "", false
	}
	return line, true
}

// lookupCNAME asks server for domain's CNAME. A truncated UDP reply is retried
// over TCP. It returns "" with no error when the name has no CNAME (including
// NXDOMAIN), and an error for transport failures or SERVFAIL/REFUSED so the
// caller can retry elsewhere. Answers are type-checked: the old code asserted
// Answer[0] was a CNAME and panicked on anything else.
func lookupCNAME(udp, tcp *dns.Client, domain, server string) (string, error) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(domain), dns.TypeCNAME)
	m.RecursionDesired = true

	r, _, err := udp.Exchange(m, server)
	if err != nil {
		return "", err
	}
	if r.Truncated {
		r, _, err = tcp.Exchange(m, server)
		if err != nil {
			return "", err
		}
	}
	switch r.Rcode {
	case dns.RcodeSuccess, dns.RcodeNameError:
	default:
		return "", fmt.Errorf("%s from %s", dns.RcodeToString[r.Rcode], server)
	}

	qname := m.Question[0].Name
	// prefer the CNAME owned by the queried name, then any CNAME in the answer
	for _, rr := range r.Answer {
		if c, ok := rr.(*dns.CNAME); ok && strings.EqualFold(c.Hdr.Name, qname) {
			return strings.TrimSuffix(c.Target, "."), nil
		}
	}
	for _, rr := range r.Answer {
		if c, ok := rr.(*dns.CNAME); ok {
			return strings.TrimSuffix(c.Target, "."), nil
		}
	}
	return "", nil
}

// otherServer picks a resolver from the pool other than the one that just failed.
func otherServer(failed string) string {
	for i := 0; i < 10; i++ {
		if s := dnsServers[rand.Intn(len(dnsServers))]; s != failed {
			return s
		}
	}
	return dnsServers[0]
}
