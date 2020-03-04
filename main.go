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
var wg sync.WaitGroup
func main() {

    var concurrency int
    flag.IntVar(&concurrency, "c", 20, "set the concurrency level")

    var verbose bool
    flag.BoolVar(&verbose, "v", false, "display url : cname")

    flag.Parse()

    m := new(dns.Msg)

    urls := make(chan string)

    for i := 0; i < concurrency; i++ {
        wg.Add(1)

        go func() {
          for url := range urls {
            m.SetQuestion(url+".", dns.TypeCNAME)
            m.RecursionDesired = true
            r, _ := dns.Exchange(m, "8.8.4.4:53")
            if r.Answer != nil {
              
              if verbose {
                fmt.Printf("%s : %s\n", url, strings.TrimSuffix(r.Answer[0].(*dns.CNAME).Target,"."))
              } else {
                fmt.Println(strings.TrimSuffix(r.Answer[0].(*dns.CNAME).Target,"."))  
              }
              
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