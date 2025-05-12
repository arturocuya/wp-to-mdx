package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

func processContent(content []Post, outputDir string, htmlOutputDir string, wpAPIBase string, isPage bool) []string {
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
	if postsOutputDir == "" {
		postsOutputDir = "./output-posts" // default value if not set
	}
	if pagesOutputDir == "" {
		pagesOutputDir = "./output-pages" // default value if not set
	}
	if htmlOutputDir == "" {
		htmlOutputDir = "./output-html" // default value if not set
	}
	if wpAPIBase == "" {
		wpAPIBase = "http://localhost:8082/wp-json/wp/v2" // default value if not set
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

	// Process posts
	posts, err := FetchPosts(db)
	if err != nil {
		log.Fatalf("Failed to fetch posts: %v", err)
	}

	// Process each post
	for i := range posts {
		// Get tags and categories
		tags, err := FetchPostTags(db, posts[i].ID)
		if err != nil {
			log.Printf("Warning: %v", err)
		}
		posts[i].Tags = tags

		categories, err := FetchPostCategories(db, posts[i].ID)
		if err != nil {
			log.Printf("Warning: %v", err)
		}
		posts[i].Categories = categories

		// Get featured image
		featuredImage, err := FetchFeaturedImage(db, posts[i].ID)
		if err != nil {
			log.Printf("Warning: %v", err)
		}
		posts[i].FeaturedImage = featuredImage

		// Merge categories into tags
		posts[i].Tags = append(posts[i].Tags, categories...)

		// Get the full URL from WordPress API
		url, err := GetPostURL(wpAPIBase, posts[i].ID)
		if err != nil {
			log.Printf("Warning: Could not get URL for post %d: %v", posts[i].ID, err)
		} else {
			posts[i].URL = url
		}
	}

	// Process pages
	pages, err := FetchPages(db)
	if err != nil {
		log.Fatalf("Failed to fetch pages: %v", err)
	}

	// Process each page
	for i := range pages {
		// Get tags and categories
		tags, err := FetchPostTags(db, pages[i].ID)
		if err != nil {
			log.Printf("Warning: %v", err)
		}
		pages[i].Tags = tags

		categories, err := FetchPostCategories(db, pages[i].ID)
		if err != nil {
			log.Printf("Warning: %v", err)
		}
		pages[i].Categories = categories

		// Get featured image
		featuredImage, err := FetchFeaturedImage(db, pages[i].ID)
		if err != nil {
			log.Printf("Warning: %v", err)
		}
		pages[i].FeaturedImage = featuredImage

		// Merge categories into tags
		pages[i].Tags = append(pages[i].Tags, categories...)

		// Get the full URL from WordPress API
		url, err := GetPageURL(wpAPIBase, pages[i].ID)
		if err != nil {
			log.Printf("Warning: Could not get URL for page %d: %v", pages[i].ID, err)
		} else {
			pages[i].URL = url
		}
	}

	// Process posts and pages
	postImageURLs := processContent(posts, postsOutputDir, htmlOutputDir, wpAPIBase, false)
	pageImageURLs := processContent(pages, pagesOutputDir, htmlOutputDir, wpAPIBase, true)

	// Combine all image URLs
	allImageURLs := append(postImageURLs, pageImageURLs...)

	// Print all image URLs that need to be downloaded
	fmt.Println("Images to download:")
	for i, src := range allImageURLs {
		fmt.Println(i, src)
	}
}
