package main

import (
    "fmt"
    "flag"
    "io"
    "strings"
    "os"
    "bufio"
    "sync"
    "github.com/miekg/dns"
)

func main() {

    var concurrency int
    flag.IntVar(&concurrency, "c", 20, "set the concurrency level")

    flag.Parse()

    config, _ := dns.ClientConfigFromFile("/etc/resolv.conf")
    c := new(dns.Client)
    m := new(dns.Msg)

    urls := make(chan string)

    // spin up a bunch of workers
    var wg sync.WaitGroup
    for i := 0; i < concurrency; i++ {
        wg.Add(1)

        go func() {
            for url := range urls {
                m.SetQuestion(url+".", dns.TypeCNAME)
                m.RecursionDesired = true
                r, _, err := c.Exchange(m, config.Servers[0]+":"+config.Port)
                if err != nil {
                    continue
                } else {
                    fmt.Println(r.Answer[0].(*dns.CNAME).Target)    
                }  
            }
            wg.Done()
        }()
    }

    var input_urls io.Reader
    input_urls = os.Stdin

    arg_url := flag.Arg(0)
    if arg_url != "" {
        input_urls = strings.NewReader(arg_url)
    }

    sc := bufio.NewScanner(input_urls)
    
    for sc.Scan() {
        urls <- sc.Text()
    }

    close(urls)
    wg.Wait()
}