package main

import (
    "bytes"
    "flag"
    "fmt"
    "io"
    "log"
    "net/http"
    "net/url"
    "os"
    "path/filepath"
    "runtime"
    "strings"
    "sync"

    "github.com/gocolly/colly/v2"
    "github.com/joho/godotenv"
    "golang.org/x/net/html"
)

type BadLink struct {
    URL       string
    Status    int
    Err       error
    ParentURL string
    TagHTML   string
    Fixed     bool
}

// downloadFile downloads a file from a URL and saves it to the given path
func downloadFile(url, filePath string) error {
    resp, err := http.Get(url)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        return fmt.Errorf("server returned status %d", resp.StatusCode)
    }

    if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
        return err
    }

    out, err := os.Create(filePath)
    if err != nil {
        return err
    }
    defer out.Close()

    _, err = io.Copy(out, resp.Body)
    return err
}

// isWpContentURL checks if the URL is a wp-content media URL
func isWpContentURL(urlStr string) bool {
    u, err := url.Parse(urlStr)
    if err != nil {
        return false
    }
    return strings.Contains(u.Path, "/wp-content/")
}

// getWpContentPath extracts the wp-content path from a URL
func getWpContentPath(urlStr string) string {
    u, err := url.Parse(urlStr)
    if err != nil {
        return ""
    }
    path := u.Path
    idx := strings.Index(path, "/wp-content/")
    if idx == -1 {
        return ""
    }
    return path[idx+1:] // Remove leading slash
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
    // Load .env file if it exists
    if err := godotenv.Load(); err != nil {
        log.Printf("Warning: .env file not found or could not be loaded: %v", err)
    }

    var fixMedia bool
    flag.BoolVar(&fixMedia, "fix-media", false, "Download and fix missing wp-content media files")
    flag.Parse()

    if flag.NArg() < 1 {
        log.Fatalf("Usage: %s [--fix-media] <start-url>", os.Args[0])
    }
    startURL := flag.Arg(0)

    var mediaOutputDir, wpBaseURL string
    if fixMedia {
        mediaOutputDir = os.Getenv("MEDIA_OUTPUT_DIR")
        if mediaOutputDir == "" {
            log.Fatalf("MEDIA_OUTPUT_DIR environment variable is required when using --fix-media")
        }
        
        wpBaseURL = os.Getenv("WP_BASE_URL")
        if wpBaseURL == "" {
            log.Fatalf("WP_BASE_URL environment variable is required when using --fix-media")
        }
        
        // Ensure wpBaseURL ends with /
        if !strings.HasSuffix(wpBaseURL, "/") {
            wpBaseURL += "/"
        }
    }

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
                Fixed:     false,
            }

            // Try to fix media files if --fix-media flag is enabled
            if fixMedia && isWpContentURL(b.URL) {
                wpPath := getWpContentPath(b.URL)
                if wpPath != "" {
                    // Construct download URL using WP_BASE_URL + wp-content path
                    downloadURL := wpBaseURL + wpPath
                    targetPath := filepath.Join(mediaOutputDir, wpPath)
                    if err := downloadFile(downloadURL, targetPath); err == nil {
                        b.Fixed = true
                        fmt.Printf("✅ Downloaded: %s -> %s\n", downloadURL, targetPath)
                    }
                }
            }

            mu.Lock()
            badLinks = append(badLinks, b)
            mu.Unlock()
            
            if !fixMedia {
                printError(b)
            }
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
            Fixed:     false,
        }

        // Try to fix media files if --fix-media flag is enabled
        if fixMedia && isWpContentURL(b.URL) {
            wpPath := getWpContentPath(b.URL)
            if wpPath != "" {
                // Construct download URL using WP_BASE_URL + wp-content path
                downloadURL := wpBaseURL + wpPath
                targetPath := filepath.Join(mediaOutputDir, wpPath)
                if downloadErr := downloadFile(downloadURL, targetPath); downloadErr == nil {
                    b.Fixed = true
                    fmt.Printf("✅ Downloaded: %s -> %s\n", downloadURL, targetPath)
                }
            }
        }

        mu.Lock()
        badLinks = append(badLinks, b)
        mu.Unlock()
        
        if !fixMedia {
            printError(b)
        }
    })

    log.Printf("Starting crawl on %s …\n", startURL)
    if err := c.Visit(startURL); err != nil {
        log.Fatalf("Failed to start crawl: %v", err)
    }
    c.Wait()

    // Summary
    if len(badLinks) == 0 {
        fmt.Println("✅ All links returned HTTP 200!")
    } else if fixMedia {
        // Separate recovered vs unrecoverable links
        var recovered, unrecoverable []BadLink
        for _, link := range badLinks {
            if link.Fixed {
                recovered = append(recovered, link)
            } else {
                unrecoverable = append(unrecoverable, link)
            }
        }

        fmt.Printf("\n=== MEDIA RECOVERY SUMMARY ===\n")
        fmt.Printf("✅ Recovered links: %d\n", len(recovered))
        for _, link := range recovered {
            fmt.Printf("  - %s\n", link.URL)
        }

        fmt.Printf("\n❌ Unrecoverable links: %d\n", len(unrecoverable))
        for _, link := range unrecoverable {
            printError(link)
        }
        
        fmt.Printf("\nTotal: %d bad links (%d recovered, %d unrecoverable)\n", 
            len(badLinks), len(recovered), len(unrecoverable))
    } else {
        fmt.Printf("\nTotal bad links found: %d\n", len(badLinks))
    }
}
