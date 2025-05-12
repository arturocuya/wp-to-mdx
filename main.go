package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
)

func ProcessContent(content []Post, outputDir string, htmlOutputDir string, wpAPIBase string, isPage bool, db *sqlx.DB) []string {
	var allImageURLs []string

	for _, item := range content {
		// Get the full URL from WordPress API just before creating the file
		var fullURL string
		var urlErr error
		if isPage {
			fullURL, urlErr = GetPageURL(wpAPIBase, item.ID)
		} else {
			fullURL, urlErr = GetPostURL(wpAPIBase, item.ID)
		}
		if urlErr != nil {
			log.Printf("Warning: Could not get URL for %d: %v", item.ID, urlErr)
			continue
		}

		// Extract the path from the URL
		u, parseErr := url.Parse(fullURL)
		if parseErr != nil {
			log.Printf("Warning: Could not parse URL %s: %v", fullURL, parseErr)
			continue
		}
		path := strings.TrimPrefix(u.Path, "/")
		if path == "" {
			path = "index"
		}
		// Remove any trailing slash from the path
		path = strings.TrimSuffix(path, "/")

		// Create HTML file path
		htmlFilePath := fmt.Sprintf("%s/%s.html", htmlOutputDir, path)

		// Create the directory path if it doesn't exist
		dirPath := filepath.Dir(htmlFilePath)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			log.Printf("Failed to create directory %s: %v", dirPath, err)
			continue
		}

		// Write HTML file
		if err := os.WriteFile(htmlFilePath, []byte(item.Content), 0644); err != nil {
			log.Printf("WriteFile error for HTML %s: %v", htmlFilePath, err)
			continue
		}

		// Convert HTML to Markdown
		markdown, imageURLs, err := ConvertHTMLToMarkdown(item.Content)
		if err != nil {
			log.Printf("Warning: Failed to convert %d to markdown: %v", item.ID, err)
			continue
		}

		markdown = PostProcessMarkdownLines(markdown, db)

		item.Content = markdown
		allImageURLs = append(allImageURLs, imageURLs...)

		// Add featured image to imageURLs if it exists
		if item.FeaturedImage != "" {
			allImageURLs = append(allImageURLs, item.FeaturedImage)
		}

		// Create markdown file path
		filePath := fmt.Sprintf("%s/%s.mdx", outputDir, path)

		// Parse dates
		publishDate, dateErr := ParseWordPressDate(item.PublishedDate)
		if dateErr != nil {
			log.Printf("Warning: Could not parse publish date '%s': %v", item.PublishedDate, dateErr)
			publishDate = time.Now() // fallback to current time
		}

		updatedDate, updateErr := ParseWordPressDate(item.UpdatedDate)
		if updateErr != nil {
			log.Printf("Warning: Could not parse update date '%s': %v", item.UpdatedDate, updateErr)
			// If we can't parse the updated date, we'll omit it from the frontmatter
		}

		// Generate frontmatter
		frontmatter := GenerateFrontmatter(item, publishDate, updatedDate)

		// Create markdown content with frontmatter
		markdownWithFrontmatter := frontmatter + item.Content

		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("MkdirAll error for %s: %v", dir, err)
			continue
		}

		// Write the markdown file
		if err := os.WriteFile(filePath, []byte(markdownWithFrontmatter), 0644); err != nil {
			log.Printf("WriteFile error for %s: %v", filePath, err)
			continue
		} else {
			log.Printf("Wrote file: %s", filePath)
		}

		// Print item information
		contentLen := min(len(item.Content), 20)
		fmt.Printf(
			"Title: %s\nDate: %s\nTags: %s\nURL: %s\nHTML File: %s\nMarkdown File: %s\nFeatured Image: %s\nContent snippet: %.60s...\n\n",
			item.Title,
			item.PublishedDate,
			strings.Join(item.Tags, ", "),
			fullURL,
			htmlFilePath,
			filePath,
			item.FeaturedImage,
			item.Content[:contentLen],
		)
	}

	return allImageURLs
}

func main() {
	// Load variables from .env file into the environment
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found; using environment variables")
	}

	// Read connection parameters from environment
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	postsOutputDir := os.Getenv("POSTS_OUTPUT_DIR")
	pagesOutputDir := os.Getenv("PAGES_OUTPUT_DIR")
	htmlOutputDir := os.Getenv("OUTPUT_HTML_DIR")
	wpAPIBase := os.Getenv("WP_API_BASE")

	// Default values if not set
	if postsOutputDir == "" {
		postsOutputDir = "./output-posts"
	}
	if pagesOutputDir == "" {
		pagesOutputDir = "./output-pages"
	}
	if htmlOutputDir == "" {
		htmlOutputDir = "./output-html"
	}
	if wpAPIBase == "" {
		wpAPIBase = "http://localhost:8082/wp-json/wp/v2"
	}

	// Create output directories if they don't exist
	for _, dir := range []string{postsOutputDir, pagesOutputDir, htmlOutputDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("Failed to create output directory %s: %v", dir, err)
		}
	}

	// Connect to database
	db, err := ConnectDB(host, port, user, password, dbName)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Fetch posts and pages
	posts, err := FetchPosts(db)
	if err != nil {
		log.Fatalf("Failed to fetch posts: %v", err)
	}
	pages, err := FetchPages(db)
	if err != nil {
		log.Fatalf("Failed to fetch pages: %v", err)
	}

	// Set up concurrency limiting
	nCPU := runtime.NumCPU()
	sem := make(chan struct{}, nCPU)
	var wg sync.WaitGroup

	// Channel to collect image-URL slices from each goroutine
	imageCh := make(chan []string, len(posts)+len(pages))

	// Process each post end-to-end in parallel
	for i := range posts {
		p := &posts[i]
		wg.Add(1)
		sem <- struct{}{}

		go func(p *Post) {
			defer wg.Done()
			defer func() { <-sem }()

			// Enrich metadata
			if tags, err := FetchPostTags(db, p.ID); err != nil {
				log.Printf("Warning fetching tags for post %d: %v", p.ID, err)
			} else {
				p.Tags = tags
			}
			if cats, err := FetchPostCategories(db, p.ID); err != nil {
				log.Printf("Warning fetching categories for post %d: %v", p.ID, err)
			} else {
				p.Categories = cats
			}
			if img, err := FetchFeaturedImage(db, p.ID); err != nil {
				log.Printf("Warning fetching featured image for post %d: %v", p.ID, err)
			} else {
				p.FeaturedImage = img
			}
			// Merge categories into tags
			p.Tags = append(p.Tags, p.Categories...)
			if url, err := GetPostURL(wpAPIBase, p.ID); err != nil {
				log.Printf("Warning getting URL for post %d: %v", p.ID, err)
			} else {
				p.URL = url
			}

			// Process content and collect images for this post
			urls := ProcessContent([]Post{*p}, postsOutputDir, htmlOutputDir, wpAPIBase, false, db)
			imageCh <- urls
		}(p)
	}

	// Process each page end-to-end in parallel
	for i := range pages {
		p := &pages[i]
		wg.Add(1)
		sem <- struct{}{}

		go func(p *Post) {
			defer wg.Done()
			defer func() { <-sem }()

			// Enrich metadata
			if tags, err := FetchPostTags(db, p.ID); err != nil {
				log.Printf("Warning fetching tags for page %d: %v", p.ID, err)
			} else {
				p.Tags = tags
			}
			if cats, err := FetchPostCategories(db, p.ID); err != nil {
				log.Printf("Warning fetching categories for page %d: %v", p.ID, err)
			} else {
				p.Categories = cats
			}
			if img, err := FetchFeaturedImage(db, p.ID); err != nil {
				log.Printf("Warning fetching featured image for page %d: %v", p.ID, err)
			} else {
				p.FeaturedImage = img
			}
			// Merge categories into tags
			p.Tags = append(p.Tags, p.Categories...)
			if url, err := GetPageURL(wpAPIBase, p.ID); err != nil {
				log.Printf("Warning getting URL for page %d: %v", p.ID, err)
			} else {
				p.URL = url
			}

			// Process content and collect images for this page
			urls := ProcessContent([]Post{*p}, pagesOutputDir, htmlOutputDir, wpAPIBase, true, db)
			imageCh <- urls
		}(p)
	}

	// Wait for all to finish, then close channel
	wg.Wait()
	close(imageCh)

	// Combine and print all image URLs
	var allImageURLs []string
	for urls := range imageCh {
		allImageURLs = append(allImageURLs, urls...)
	}

	fmt.Println("Images to download:")
	for i, src := range allImageURLs {
		fmt.Println(i, src)
	}
}
