package main

import (
    "bufio"
    "flag"
    "fmt"
    "io"
    "log"
    "os"
    "strings"
    "sync"

    "github.com/miekg/dns"
)

var wg sync.WaitGroup

func main() {

    var concurrency int
    flag.IntVar(&concurrency, "c", 20, "set the concurrency level")

    var verbose bool
    flag.BoolVar(&verbose, "v", false, "display domain : cname")

    flag.Parse()

    // use a buffered channel to prevent blocking
    domains := make(chan string, 100)

    for i := 0; i < concurrency; i++ {
        wg.Add(1)

        go func() {

            for domain := range domains {
                m := new(dns.Msg)
                m.SetQuestion(domain+".", dns.TypeCNAME)
                m.RecursionDesired = true
                r, err := dns.Exchange(m, "8.8.4.4:53")
                if err != nil {
                    log.Fatalln(err)
                }
                if r.Answer != nil {
                    cname := r.Answer[0].(*dns.CNAME).Target
                    if verbose {
                        fmt.Printf("%s : %s\n", domain, strings.TrimSuffix(cname, "."))
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
    for sc.Scan() {
        domains <- sc.Text()
    }

    // tidy up
    close(domains)
    wg.Wait()
}
