package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// GetPostURL fetches the full URL of a post using the WordPress REST API
func GetPostURL(apiBase string, postID int) (string, error) {
	client := &http.Client{}
	url := fmt.Sprintf("%s/posts/%d", apiBase, postID)
	log.Printf("Fetching post URL from: %s", url)

	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch post URL: %v", err)
	}
	defer resp.Body.Close()

	log.Printf("Post API response status: %d", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		body, _ := json.Marshal(resp.Body)
		log.Printf("Post API error response body: %s", string(body))
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		Link string `json:"link"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %v", err)
	}

	log.Printf("Successfully fetched post URL: %s", result.Link)
	return result.Link, nil
}

// GetPageURL fetches the full URL of a page using the WordPress REST API
func GetPageURL(apiBase string, pageID int) (string, error) {
	client := &http.Client{}
	url := fmt.Sprintf("%s/pages/%d", apiBase, pageID)
	log.Printf("Fetching page URL from: %s", url)

	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch page URL: %v", err)
	}
	defer resp.Body.Close()

	log.Printf("Page API response status: %d", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		body, _ := json.Marshal(resp.Body)
		log.Printf("Page API error response body: %s", string(body))
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		Link string `json:"link"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %v", err)
	}

	log.Printf("Successfully fetched page URL: %s", result.Link)
	return result.Link, nil
}
