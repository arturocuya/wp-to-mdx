package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	html2md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
)

// Post represents a WordPress post with title, publish date and HTML content.
type Post struct {
	Title         string `db:"title"`
	PublishedDate string `db:"published_date"`
	Content       string `db:"content"`
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
	outputDir := os.Getenv("OUTPUT_DIR")
	if outputDir == "" {
		outputDir = "./output-md" // default value if not set
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Build DSN (Data Source Name)
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		user, password, host, port, dbName,
	)

	// Connect to MySQL using sqlx
	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Query to fetch title, publish date and HTML content
	query := `
        SELECT
          post_title   AS title,
          post_date    AS published_date,
          post_content AS content
        FROM wp_posts
        WHERE
          post_type   = 'post'
          AND post_status = 'publish'
        ORDER BY post_date DESC;
    `

	// Slice to hold unmarshaled posts
	var posts []Post

	// Execute query and map results into the slice
	if err := db.Select(&posts, query); err != nil {
		log.Fatalf("Query execution error: %v", err)
	}

	// Create a new converter with default options
	converter := html2md.NewConverter("", true, nil)

	var imageURLs []string

	// Add custom rule for images inside paragraph tags
	converter.AddRules(
		html2md.Rule{
			Filter: []string{"p", "span", "h1", "h2", "h3", "h4", "h5", "h6"},
			Replacement: func(content string, selec *goquery.Selection, opt *html2md.Options) *string {
				// Check if the paragraph contains only an image
				if selec.Children().Length() == 1 && selec.Children().Is("img") {
					img := selec.Children().First()
					src, _ := img.Attr("src")
					alt, _ := img.Attr("alt")

					imageURLs = append(imageURLs, src)

					// Construct markdown image with attributes
					markdown := fmt.Sprintf("\n\n<Image src=\"%s\" alt=\"%s\" />\n\n", src, alt)
					return &markdown
				}
				return nil // Let the default rule handle other cases
			},
		},
	)

	// Add rule for iframes
	converter.AddRules(
		html2md.Rule{
			Filter: []string{"iframe"},
			Replacement: func(content string, selec *goquery.Selection, opt *html2md.Options) *string {
				src, exists := selec.Attr("src")
				if !exists {
					return nil
				}
				markdown := fmt.Sprintf("\n\n[View embedded content](%s)\n\n", src)
				return &markdown
			},
		},
	)

	// Print a snippet of each post
	for _, post := range posts {
		if err := os.WriteFile(fmt.Sprintf("./output-html/%s.html", post.Title), []byte(post.Content), 0644); err != nil {
			// log.Fatalf("WriteFile error: %v", err)
			continue
		}

		// Convert the HTML string to Markdown
		markdown, err := converter.ConvertString(post.Content)
		if err != nil {
			log.Fatalf("Conversion error: %v", err)
		}

		post.Content = markdown

		contentLen := min(len(post.Content), 20)

		fmt.Printf(
			"Title: %s\nDate: %s\nContent snippet: %.60s...\n\n",
			post.Title,
			post.PublishedDate,
			post.Content[:contentLen],
		)

		// Parse the date from the database format
		publishDate, err := time.Parse(time.RFC3339, post.PublishedDate)
		if err != nil {
			log.Printf("Warning: Could not parse date '%s': %v", post.PublishedDate, err)
			publishDate = time.Now() // fallback to current time
		}

		// Create markdown content with frontmatter
		markdownWithFrontmatter := fmt.Sprintf("---\ntitle: %s\nexcerpt: \"\"\npublishDate: %s\nisFeatured: false\ntags: []\nseo: {}\n---\n\n%s",
			strconv.Quote(post.Title),
			strconv.Quote(publishDate.Format("2006-01-02")),
			post.Content,
		)

		if err := os.WriteFile(fmt.Sprintf("%s/%s.md", outputDir, post.Title), []byte(markdownWithFrontmatter), 0644); err != nil {
			// log.Fatalf("WriteFile error: %v", err)
			continue
		}
	}

	fmt.Println("images to download")
	for i, src := range imageURLs {
		fmt.Println(i, src)
	}
}
