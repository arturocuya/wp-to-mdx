package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/jmoiron/sqlx"
)

func PostProcessMarkdownLines(markdown string, db *sqlx.DB) (string, []string) {
	// Get base URL from environment
	baseURL := os.Getenv("WP_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8082"
	}
	// Compile once
	audioRe := regexp.MustCompile(`\[audio\s+mp3="([^"]+)"\]\s*\[/audio\]`)
	videoRe := regexp.MustCompile(`\[video\s+width="(\d+)"\s+height="(\d+)"\s+mp4="([^"]+)"\]\s*\[/video\]`)

	// post-processing for YouTube links...
	var mediaURLs []string
	splittedMd := strings.Split(markdown, "\n")
	for i, line := range splittedMd {
		line = strings.TrimSpace(line)
		line = strings.ReplaceAll(line, `\[`, `[`)
		line = strings.ReplaceAll(line, `\]`, `]`)

		parts := strings.SplitN(line, " ", 2)
		link := parts[0]
		rest := ""
		if len(parts) > 1 {
			rest = " " + parts[1]
		}

		link = strings.ReplaceAll(link, "https://www.youtube.com/watch?v=", "https://youtu.be/")
		link = strings.ReplaceAll(link, "https://www.youtube.com/", "https://youtu.be/")
		link = strings.ReplaceAll(link, "https://youtube.com/", "https://youtu.be/")

		if strings.HasPrefix(link, "https://youtu.be") {
			splittedMd[i] = fmt.Sprintf("<YouTube id=\"%s\" />%s", link, rest)
		}

		// gallery shortcode?
		galleryRe := regexp.MustCompile(`\[gallery\s+.*?ids="[^"]+".*?\]`)
		if galleryRe.MatchString(line) {
			ids, err := parseGalleryIDs(line)
			if err != nil {
				log.Printf("Warning: error parsing gallery: %s", err)
				continue
			}

			dbURLs, _ := GetImageURLsFromDB(db, ids)

			splittedMd[i] = ""

			for _, url := range dbURLs {
				// Strip base URL to make path relative
				relativePath := strings.TrimPrefix(url, baseURL)
				splittedMd[i] += fmt.Sprintf("<img src=\"%s\"/>\n\n", relativePath)
				mediaURLs = append(mediaURLs, url) // Keep full URL for download
			}
		}

		// audio shortcode?
		if m := audioRe.FindStringSubmatch(line); m != nil {
			src := m[1]
			// Strip base URL to make path relative
			relativePath := strings.TrimPrefix(src, baseURL)
			splittedMd[i] = fmt.Sprintf(
				`<audio controls>
    <source src="%s" type="audio/mpeg"/>
    Your browser does not support the audio element.
</audio>`, relativePath,
			)
			mediaURLs = append(mediaURLs, src) // Keep full URL for download
			fmt.Println("processed audio shortcode")
			continue
		}

		// video shortcode?
		if m := videoRe.FindStringSubmatch(line); m != nil {
			width, height, src := m[1], m[2], m[3]
			// Strip base URL to make path relative
			relativePath := strings.TrimPrefix(src, baseURL)
			splittedMd[i] = fmt.Sprintf(
				`<video controls width="%s" height="%s">
    <source src="%s" type="video/mp4"/>
    Your browser does not support the video tag.
</video>`, width, height, relativePath,
			)
			mediaURLs = append(mediaURLs, src) // Keep full URL for download
			fmt.Println("processed video shortcode")
		}
	}
	markdown = strings.Join(splittedMd, "\n")

	if strings.Contains(markdown, "<YouTube id=") {
		markdown = fmt.Sprintf("import { YouTube } from 'astro-embed';\n\n%s", markdown)
	}

	if strings.Contains(markdown, "<Image") {
		markdown = fmt.Sprintf("import { Image } from 'astro:assets';\n\n%s", markdown)
	}

	return markdown, mediaURLs
}

// parseGalleryIDs extracts all numeric IDs from a string like:
// [gallery columns="1" size="full" ids="3528,3529,3530,…"]
func parseGalleryIDs(content string) ([]int, error) {
	re := regexp.MustCompile(`ids="([^"]+)"`)
	match := re.FindStringSubmatch(content)
	if len(match) < 2 {
		return nil, fmt.Errorf("no ids attribute found in gallery shortcode")
	}
	parts := strings.Split(match[1], ",")
	var ids []int
	for _, p := range parts {
		id, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return nil, fmt.Errorf("invalid id %q: %w", p, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
