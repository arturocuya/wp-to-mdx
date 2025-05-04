package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	html2md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
)

// Post represents a WordPress post with title, publish date, update date, HTML content, and related taxonomies.
type Post struct {
	ID            int      `db:"ID"`
	Title         string   `db:"title"`
	PublishedDate string   `db:"published_date"`
	UpdatedDate   string   `db:"updated_date"`
	Content       string   `db:"content"`
	URL           string   // Will be populated from WordPress API
	Tags          []string // Will be populated separately
	Categories    []string // Will be populated separately
	IsFeatured    bool     // Default is false
	FeaturedImage string   // Will be populated from WordPress API
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
	wpAPIBase := os.Getenv("WP_API_BASE")
	if postsOutputDir == "" {
		postsOutputDir = "./output-posts" // default value if not set
	}
	if wpAPIBase == "" {
		wpAPIBase = "http://localhost:8082/wp-json/wp/v2" // default value if not set
	}

	// Create posts output directory if it doesn't exist
	if err := os.MkdirAll(postsOutputDir, 0755); err != nil {
		log.Fatalf("Failed to create posts output directory: %v", err)
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

	// Query to fetch posts with title, publish date, update date and HTML content
	query := `
        SELECT
          ID,
          post_title   AS title,
          post_date    AS published_date,
          post_modified AS updated_date,
          post_content AS content
        FROM wp_posts
        WHERE
          post_type   = 'post'
          AND post_status = 'publish'
        ORDER BY post_date DESC limit 10;
    `

	// Slice to hold unmarshaled posts
	var posts []Post

	// Execute query and map results into the slice
	if err := db.Select(&posts, query); err != nil {
		log.Fatalf("Query execution error: %v", err)
	}

	// For each post, fetch its tags and categories
	for i := range posts {
		// Get tags for the post
		var tags []string
		tagQuery := `
			SELECT t.name
			FROM wp_terms t
			INNER JOIN wp_term_taxonomy tt ON t.term_id = tt.term_id
			INNER JOIN wp_term_relationships tr ON tt.term_taxonomy_id = tr.term_taxonomy_id
			WHERE tr.object_id = ?
			AND tt.taxonomy = 'post_tag';
		`
		if err := db.Select(&tags, tagQuery, posts[i].ID); err != nil {
			log.Printf("Error fetching tags for post %d: %v", posts[i].ID, err)
		}
		posts[i].Tags = tags

		// Get categories for the post
		var categories []string
		categoryQuery := `
			SELECT t.name
			FROM wp_terms t
			INNER JOIN wp_term_taxonomy tt ON t.term_id = tt.term_id
			INNER JOIN wp_term_relationships tr ON tt.term_taxonomy_id = tr.term_taxonomy_id
			WHERE tr.object_id = ?
			AND tt.taxonomy = 'category';
		`
		if err := db.Select(&categories, categoryQuery, posts[i].ID); err != nil {
			log.Printf("Error fetching categories for post %d: %v", posts[i].ID, err)
		}
		posts[i].Categories = categories

		// Get featured image for the post
		var featuredImageID int
		featuredImageQuery := `
			SELECT meta_value
			FROM wp_postmeta
			WHERE post_id = ?
			AND meta_key = '_thumbnail_id';
		`
		if err := db.Get(&featuredImageID, featuredImageQuery, posts[i].ID); err != nil {
			log.Printf("Error fetching featured image ID for post %d: %v", posts[i].ID, err)
		} else if featuredImageID > 0 {
			// Get the image URL from wp_posts
			var imageURL string
			imageURLQuery := `
				SELECT guid
				FROM wp_posts
				WHERE ID = ?;
			`
			if err := db.Get(&imageURL, imageURLQuery, featuredImageID); err != nil {
				log.Printf("Error fetching featured image URL for post %d: %v", posts[i].ID, err)
			} else {
				posts[i].FeaturedImage = imageURL
			}
		}

		// Merge categories into tags for the frontmatter schema
		// This is because the schema only has a tags field, not categories
		posts[i].Tags = append(posts[i].Tags, categories...)

		// Get the full URL from WordPress API
		url, err := getPostURL(wpAPIBase, posts[i].ID)
		if err != nil {
			log.Printf("Warning: Could not get URL for post %d: %v", posts[i].ID, err)
		} else {
			posts[i].URL = url
		}

		fmt.Println("url", url)
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

	// Process each post
	for _, post := range posts {
		// Convert the HTML string to Markdown
		markdown, err := converter.ConvertString(post.Content)
		if err != nil {
			log.Printf("Conversion error for post %s: %v", post.Title, err)
			continue
		}

		post.Content = markdown

		contentLen := min(len(post.Content), 20)

		// Get the full URL from WordPress API just before creating the file
		fullURL, err := getPostURL(wpAPIBase, post.ID)
		if err != nil {
			log.Printf("Warning: Could not get URL for post %d: %v", post.ID, err)
			continue
		}

		// Extract the path from the URL
		u, err := url.Parse(fullURL)
		if err != nil {
			log.Printf("Warning: Could not parse URL %s: %v", fullURL, err)
			continue
		}
		path := strings.TrimPrefix(u.Path, "/")
		if path == "" {
			path = "index"
		}
		// Remove any trailing slash from the path
		path = strings.TrimSuffix(path, "/")
		filePath := fmt.Sprintf("%s/%s.md", postsOutputDir, path)

		// Create the directory path if it doesn't exist
		dirPath := filepath.Dir(filePath)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			log.Printf("Failed to create directory %s: %v", dirPath, err)
			continue
		}

		fmt.Printf(
			"Title: %s\nDate: %s\nTags: %s\nURL: %s\nFile: %s\nFeatured Image: %s\nContent snippet: %.60s...\n\n",
			post.Title,
			post.PublishedDate,
			strings.Join(post.Tags, ", "),
			fullURL,
			filePath,
			post.FeaturedImage,
			post.Content[:contentLen],
		)

		// Add featured image to imageURLs if it exists
		if post.FeaturedImage != "" {
			imageURLs = append(imageURLs, post.FeaturedImage)
		}

		// Parse the publish date from the database format
		publishDate, err := parseWordPressDate(post.PublishedDate)
		if err != nil {
			log.Printf("Warning: Could not parse publish date '%s': %v", post.PublishedDate, err)
			publishDate = time.Now() // fallback to current time
		}

		// Parse the update date from the database format
		var updatedDateFrontmatter string
		updatedDate, err := parseWordPressDate(post.UpdatedDate)
		if err != nil {
			log.Printf("Warning: Could not parse update date '%s': %v", post.UpdatedDate, err)
			// If we can't parse the updated date, we'll omit it from the frontmatter
		} else {
			updatedDateFrontmatter = fmt.Sprintf("updatedDate: %s\n", strconv.Quote(updatedDate.Format("2006-01-02")))
		}

		// Format tags as a JSON array for the frontmatter
		tagsJSON := "[]"
		if len(post.Tags) > 0 {
			quotedTags := make([]string, len(post.Tags))
			for i, tag := range post.Tags {
				quotedTags[i] = strconv.Quote(tag)
			}
			tagsJSON = fmt.Sprintf("[%s]", strings.Join(quotedTags, ", "))
		}

		// Create markdown content with frontmatter according to the schema
		featuredImageFrontmatter := ""
		if post.FeaturedImage != "" {
			featuredImageFrontmatter = fmt.Sprintf("featuredImage: %s\n", strconv.Quote(post.FeaturedImage))
		}
		markdownWithFrontmatter := fmt.Sprintf("---\ntitle: %s\nexcerpt: \"\"\npublishDate: %s\n%sisFeatured: false\ntags: %s\n%sseo: {}\n---\n\n%s",
			strconv.Quote(post.Title),
			strconv.Quote(publishDate.Format("2006-01-02")),
			updatedDateFrontmatter,
			tagsJSON,
			featuredImageFrontmatter,
			post.Content,
		)

		if err := os.WriteFile(filePath, []byte(markdownWithFrontmatter), 0644); err != nil {
			log.Printf("WriteFile error for %s: %v", filePath, err)
			continue
		}
	}

	fmt.Println("Images to download:")
	for i, src := range imageURLs {
		fmt.Println(i, src)
	}
}

// parseWordPressDate attempts to parse a WordPress date string using multiple formats
func parseWordPressDate(dateStr string) (time.Time, error) {
	// List of possible date formats in WordPress
	dateFormats := []string{
		"2006-01-02 15:04:05",           // MySQL datetime format
		time.RFC3339,                    // "2006-01-02T15:04:05Z07:00"
		"2006-01-02T15:04:05-07:00",     // WordPress often uses this format
		"2006-01-02T15:04:05.000-07:00", // With milliseconds
		"2006-01-02T15:04:05.000Z",      // UTC with milliseconds
	}

	// Try each format until one works
	for _, format := range dateFormats {
		if date, err := time.Parse(format, dateStr); err == nil {
			return date, nil
		}
	}

	// None of the formats worked
	return time.Time{}, fmt.Errorf("could not parse date using any known WordPress formats: %s", dateStr)
}

// sanitizeFilename removes characters that might cause problems in filenames
func sanitizeFilename(filename string) string {
	// Replace problematic characters with underscores
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return replacer.Replace(filename)
}

// getPostURL fetches the full URL of a post using the WordPress REST API
func getPostURL(apiBase string, postID int) (string, error) {
	client := &http.Client{}
	url := fmt.Sprintf("%s/posts/%d", apiBase, postID)

	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch post URL: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		Link string `json:"link"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %v", err)
	}

	return result.Link, nil
}
