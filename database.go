package main

import (
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
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

// ConnectDB establishes a connection to the MySQL database
func ConnectDB(host, port, user, password, dbName string) (*sqlx.DB, error) {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		user, password, host, port, dbName,
	)
	return sqlx.Connect("mysql", dsn)
}

// FetchPosts retrieves all published posts from the database
func FetchPosts(db *sqlx.DB) ([]Post, error) {
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
        ORDER BY post_date DESC;
    `

	var posts []Post
	if err := db.Select(&posts, query); err != nil {
		return nil, fmt.Errorf("query execution error: %v", err)
	}

	return posts, nil
}

// FetchPostTags retrieves all tags for a post
func FetchPostTags(db *sqlx.DB, postID int) ([]string, error) {
	var tags []string
	query := `
		SELECT t.name
		FROM wp_terms t
		INNER JOIN wp_term_taxonomy tt ON t.term_id = tt.term_id
		INNER JOIN wp_term_relationships tr ON tt.term_taxonomy_id = tr.term_taxonomy_id
		WHERE tr.object_id = ?
		AND tt.taxonomy = 'post_tag';
	`
	if err := db.Select(&tags, query, postID); err != nil {
		return nil, fmt.Errorf("error fetching tags for post %d: %v", postID, err)
	}
	return tags, nil
}

// FetchPostCategories retrieves all categories for a post
func FetchPostCategories(db *sqlx.DB, postID int) ([]string, error) {
	var categories []string
	query := `
		SELECT t.name
		FROM wp_terms t
		INNER JOIN wp_term_taxonomy tt ON t.term_id = tt.term_id
		INNER JOIN wp_term_relationships tr ON tt.term_taxonomy_id = tr.term_taxonomy_id
		WHERE tr.object_id = ?
		AND tt.taxonomy = 'category';
	`
	if err := db.Select(&categories, query, postID); err != nil {
		return nil, fmt.Errorf("error fetching categories for post %d: %v", postID, err)
	}
	return categories, nil
}

// FetchFeaturedImage retrieves the featured image URL for a post
func FetchFeaturedImage(db *sqlx.DB, postID int) (string, error) {
	var featuredImageID int
	query := `
		SELECT meta_value
		FROM wp_postmeta
		WHERE post_id = ?
		AND meta_key = '_thumbnail_id';
	`
	if err := db.Get(&featuredImageID, query, postID); err != nil {
		return "", fmt.Errorf("error fetching featured image ID for post %d: %v", postID, err)
	}

	if featuredImageID > 0 {
		var imageURL string
		query := `
			SELECT guid
			FROM wp_posts
			WHERE ID = ?;
		`
		if err := db.Get(&imageURL, query, featuredImageID); err != nil {
			return "", fmt.Errorf("error fetching featured image URL for post %d: %v", postID, err)
		}
		return imageURL, nil
	}

	return "", nil
}

// FetchPages retrieves all published pages from the WordPress database
func FetchPages(db *sqlx.DB) ([]Post, error) {
	query := `
        SELECT
          ID,
          post_title   AS title,
          post_date    AS published_date,
          post_modified AS updated_date,
          post_content AS content
        FROM wp_posts
        WHERE
          post_type   = 'page'
          AND post_status = 'publish'
        ORDER BY post_date DESC;
    `

	var pages []Post
	if err := db.Select(&pages, query); err != nil {
		return nil, fmt.Errorf("failed to fetch pages: %v", err)
	}

	return pages, nil
}
