package main

import (
    "bytes"
    "fmt"
    "log"
    "net/url"
    "os"
    "runtime"
    "strings"
    "sync"

    "github.com/gocolly/colly/v2"
    "golang.org/x/net/html"
)

type BadLink struct {
    URL       string
    Status    int
    Err       error
    ParentURL string
    TagHTML   string
}

// printError emits a readable, top-down report for a bad link
func printError(b BadLink) {
    fmt.Println("----- BAD LINK FOUND -----")
    fmt.Printf("URL:         %s\n", b.URL)
    if b.Status != 0 {
        fmt.Printf("Status:      %d\n", b.Status)
    }
    if b.Err != nil {
        fmt.Printf("Error:       %s\n", b.Err)
    }
    fmt.Printf("Parent page: %s\n", b.ParentURL)
    // collapse whitespace in the tag HTML
    tag := strings.Join(strings.Fields(b.TagHTML), " ")
    fmt.Printf("Anchor tag:  %s\n", tag)
    fmt.Println("--------------------------")
}

func main() {
    if len(os.Args) < 2 {
        log.Fatalf("Usage: %s <start-url>", os.Args[0])
    }
    startURL := os.Args[1]

    parsed, err := url.Parse(startURL)
    if err != nil {
        log.Fatalf("Invalid start URL: %v", err)
    }
    domain := parsed.Hostname()

    // Async collector, same-domain only
    c := colly.NewCollector(
        colly.Async(true),
        colly.AllowedDomains(domain),
    )
    c.Limit(&colly.LimitRule{
        DomainGlob:  "*",
        Parallelism: runtime.NumCPU(),
    })

    var mu sync.Mutex
    var badLinks []BadLink

    c.OnHTML("a[href]", func(e *colly.HTMLElement) {
        link := e.Request.AbsoluteURL(e.Attr("href"))
        if link == "" {
            return
        }
        u, err := url.Parse(link)
        if err != nil || u.Hostname() != domain {
            return
        }

        // Render the <a>…</a> outer HTML
        var buf bytes.Buffer
        if len(e.DOM.Nodes) > 0 {
            if err := html.Render(&buf, e.DOM.Nodes[0]); err != nil {
                log.Printf("failed to render <a> node: %v", err)
            }
        }
        tagHTML := strings.TrimSpace(buf.String())

        // Store parent info
        ctx := colly.NewContext()
        ctx.Put("parentURL", e.Request.URL.String())
        ctx.Put("parentTag", tagHTML)

        c.Request("GET", link, nil, ctx, nil)
    })

    c.OnResponse(func(r *colly.Response) {
        if r.StatusCode != 200 {
            parent := r.Ctx.Get("parentURL")
            tag := r.Ctx.Get("parentTag")
            b := BadLink{
                URL:       r.Request.URL.String(),
                Status:    r.StatusCode,
                ParentURL: parent,
                TagHTML:   tag,
            }
            mu.Lock()
            badLinks = append(badLinks, b)
            mu.Unlock()
            printError(b)
        }
    })

    c.OnError(func(r *colly.Response, err error) {
        parent := r.Ctx.Get("parentURL")
        tag := r.Ctx.Get("parentTag")
        b := BadLink{
            URL:       r.Request.URL.String(),
            Err:       err,
            ParentURL: parent,
            TagHTML:   tag,
        }
        mu.Lock()
        badLinks = append(badLinks, b)
        mu.Unlock()
        printError(b)
    })

    log.Printf("Starting crawl on %s …\n", startURL)
    if err := c.Visit(startURL); err != nil {
        log.Fatalf("Failed to start crawl: %v", err)
    }
    c.Wait()

    // Optional summary
    if len(badLinks) == 0 {
        fmt.Println("✅ All links returned HTTP 200!")
    } else {
        fmt.Printf("\nTotal bad links found: %d\n", len(badLinks))
    }
}
