package main

import (
    "bufio"
    "flag"
    "fmt"
    "io"
    "log"
    "math/rand"
    "os"
    "strings"
    "sync"
    "time"

    "github.com/miekg/dns"
)

var wg sync.WaitGroup

type check struct {
    Domain     string
    Nameserver string
}

func main() {

    var concurrency int
    flag.IntVar(&concurrency, "c", 20, "set the concurrency level")

    var verbose bool
    flag.BoolVar(&verbose, "v", false, "display domain : cname")

    flag.Parse()

    // a list of dns servers to randomly choose from
    dns_servers := []string{
        "1.1.1.1:53",
        "8.8.8.8:53",
        "8.8.4.4:53",
        "9.9.9.9:53",
    }

    // seed to randomly select dns server
    rand.Seed(time.Now().UnixNano())

    // use a buffered channel to prevent blocking
    checks := make(chan check, 100)

    for i := 0; i < concurrency; i++ {
        wg.Add(1)

        go func() {

            for check := range checks {

                m := new(dns.Msg)
                m.SetQuestion(check.Domain+".", dns.TypeCNAME)
                m.RecursionDesired = true
                r, err := dns.Exchange(m, check.Nameserver)
                if err != nil {
                    log.Fatalln(err)
                }
                if r.Answer != nil {
                    cname := r.Answer[0].(*dns.CNAME).Target
                    if verbose {
                        fmt.Printf("%s : %s\n", check.Domain, strings.TrimSuffix(cname, "."))
                    } else {
                        fmt.Println(strings.TrimSuffix(cname, "."))
                    }

                }
            }
            wg.Done()
        }()
    }

    // read user input either piped to the program
    // or from a single argument
    var input_domains io.Reader
    input_domains = os.Stdin

    arg_domain := flag.Arg(0)
    if arg_domain != "" {
        input_domains = strings.NewReader(arg_domain)
    }

    sc := bufio.NewScanner(input_domains)

    // send input to the channel
    // and randomly choose a server from the list
    // to help prevent timeouts
    for sc.Scan() {
        server := dns_servers[rand.Intn(len(dns_servers))]
        checks <- check{sc.Text(), server}
    }

    // tidy up
    close(checks)
    wg.Wait()
}
