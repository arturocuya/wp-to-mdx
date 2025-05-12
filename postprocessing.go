package main

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/jmoiron/sqlx"
)

func PostProcessMarkdownLines(markdown string, db *sqlx.DB) string {
	// post-processing for YouTube links...
	splittedMd := strings.Split(markdown, "\n")
	for i, line := range splittedMd {
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

		if strings.HasPrefix(line, "\\[gallery ") {
			ids, err := parseGalleryIDs(line)
			if err != nil {
				log.Printf("Warning: error parsing gallery: %s", err)
				continue
			}

			dbURLs, err := GetImageURLsFromDB(db, ids)

			splittedMd[i] = ""

			for _, url := range dbURLs {
				splittedMd[i] += fmt.Sprintf("<img src=\"%s\"/>\n\n", url)
			}
		}
	}
	markdown = strings.Join(splittedMd, "\n")

	if strings.Contains(markdown, "<YouTube id=") {
		markdown = fmt.Sprintf("import { YouTube } from 'astro-embed';\n\n%s", markdown)
	}

	if strings.Contains(markdown, "<Image") {
		markdown = fmt.Sprintf("import { Image } from 'astro:assets';\n\n%s", markdown)
	}

	return markdown
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
