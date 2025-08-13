package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	html2md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
)

var client = &http.Client{}

// ConvertHTMLToMarkdown converts HTML content to Markdown format
func ConvertHTMLToMarkdown(inputHtml string) (string, []string, error) {
	// turn &lt; into &amp;lt;  so the parser produces a text node containing "&lt;"
	inputHtml = strings.ReplaceAll(inputHtml, "&lt;", "&amp;lt;")
	inputHtml = strings.ReplaceAll(inputHtml, "&gt;", "&amp;gt;")

	converter := html2md.NewConverter("", true, nil)
	var imageURLs []string

	// Load base URL from environment
	baseURL := os.Getenv("WP_BASE_URL")

	// Rule to strip baseURL from all <a> hrefs
	converter.AddRules(
		html2md.Rule{
			Filter: []string{"a"},
			Replacement: func(_ string, selec *goquery.Selection, _ *html2md.Options) *string {
				href, ok := selec.Attr("href")
				if !ok {
					return nil
				}

				finalURL := href
				// only follow redirects for links under our own site
				if strings.HasPrefix(href, baseURL) {
					if resp, err := client.Get(href); err == nil {
						defer resp.Body.Close()
						finalURL = resp.Request.URL.String()
					}
				}

				// convert to a site-relative path
				newHref := strings.ReplaceAll(finalURL, baseURL, "/")
				text := selec.Text()
				md := fmt.Sprintf("[%s](%s)", text, newHref)
				return &md
			},
		},
	)
	// Add custom rule for images inside paragraph tags
	converter.AddRules(
		html2md.Rule{
			Filter: []string{"p", "span", "h1", "h2", "h3", "h4", "h5", "h6"},
			Replacement: func(content string, selec *goquery.Selection, opt *html2md.Options) *string {
				if selec.Children().Length() == 1 && selec.Children().Is("img") {
					img := selec.Children().First()
					src, _ := img.Attr("src")
					alt, _ := img.Attr("alt")

					// Keep full URL for downloads
					imageURLs = append(imageURLs, src)

					// Strip base URL for display
					relativePath := strings.TrimPrefix(src, baseURL)
					markdown := fmt.Sprintf("\n\n<img src=\"%s\" alt=\"%s\" />\n\n", relativePath, alt)
					return &markdown
				}
				return nil
			},
		},
	)

	// Add custom rule for figure tags with single <a> child
	converter.AddRules(
		html2md.Rule{
			Filter: []string{"figure"},
			Replacement: func(content string, selec *goquery.Selection, opt *html2md.Options) *string {
				// Check for audio element in figure
				if selec.Children().Length() == 1 && selec.Children().Is("audio") {
					audio := selec.Children().First()
					src, ok := audio.Attr("src")
					if ok {
						// Keep full URL for downloads
						imageURLs = append(imageURLs, src)

						// Strip base URL for display
						relativePath := strings.TrimPrefix(src, baseURL)
						markdown := fmt.Sprintf("\n\n<audio controls src=\"/%s\"></audio>\n\n", relativePath)
						return &markdown
					}
				}

				// Check for YouTube video in figure with div and figcaption
				if selec.Children().Length() == 2 {
					div := selec.Children().First()
					figcaption := selec.Children().Last()

					if div.Is("div") && figcaption.Is("figcaption") {
						divText := strings.TrimSpace(div.Text())
						captionText := strings.TrimSpace(figcaption.Text())

						// Check if div contains YouTube URL
						if strings.Contains(divText, "youtu.be") || strings.Contains(divText, "youtube.com") {
							// Extract video ID from URL
							videoID := ""
							if strings.Contains(divText, "youtu.be/") {
								parts := strings.Split(divText, "youtu.be/")
								if len(parts) > 1 {
									videoID = strings.Split(parts[1], "?")[0]
									videoID = strings.Split(videoID, "&")[0]
								}
							} else if strings.Contains(divText, "youtube.com/watch?v=") {
								parts := strings.Split(divText, "v=")
								if len(parts) > 1 {
									videoID = strings.Split(parts[1], "&")[0]
								}
							}

							if videoID != "" {
								markdown := fmt.Sprintf("\n\n<YouTube id=\"%s\" />\n\n%s\n\n", videoID, captionText)
								return &markdown
							}
						}
					}
				}

				if selec.Children().Length() == 1 && selec.Children().Is("a") {
					a := selec.Children().First()
					href, _ := a.Attr("href")

					// Keep full URL for downloads
					imageURLs = append(imageURLs, href)

					// Strip base URL for display
					relativePath := strings.TrimPrefix(href, baseURL)

					// Use link text as alt text if available
					altText := a.Text()
					if altText == "" {
						altText = "Image"
					}

					markdown := fmt.Sprintf("\n\n<img src=\"/%s\" alt=\"%s\" />\n\n", relativePath, altText)
					return &markdown
				}
				return nil
			},
		},
	)

	// Add rule for iframes
	converter.AddRules(
		html2md.Rule{
			Filter: []string{"iframe"},
			Replacement: func(content string, selec *goquery.Selection, opt *html2md.Options) *string {
				src, ok := selec.Attr("src")
				if !ok {
					return nil
				}
				if strings.Contains(src, "youtube.com") || strings.Contains(src, "youtu.be") {
					src = strings.ReplaceAll(src, "https://www.youtube.com/embed/", "https://youtu.be/")
					md := fmt.Sprintf("\n\n<YouTube id=\"%s\" />\n\n", src)
					return &md
				}
				md := fmt.Sprintf("\n\n[View embedded content](%s)\n\n", src)
				return &md
			},
		},
	)

	markdown, err := converter.ConvertString(inputHtml)
	if err != nil {
		return "", nil, fmt.Errorf("conversion error: %v", err)
	}

	// Handle [youtube]URL[/youtube] shortcode format
	markdown = processYouTubeShortcodes(markdown)

	return markdown, imageURLs, nil
}

// GenerateFrontmatter creates the frontmatter for a markdown file
func GenerateFrontmatter(post Post, publishDate, updatedDate time.Time) string {
	// Format tags as a JSON array for the frontmatter
	tagsJSON := "[]"
	if len(post.Tags) > 0 {
		quotedTags := make([]string, len(post.Tags))
		for i, tag := range post.Tags {
			quotedTags[i] = strconv.Quote(tag)
		}
		tagsJSON = fmt.Sprintf("[%s]", strings.Join(quotedTags, ", "))
	}

	// Add updated date to frontmatter if available
	updatedDateFrontmatter := ""
	if !updatedDate.IsZero() {
		updatedDateFrontmatter = fmt.Sprintf("updatedDate: %s\n", strconv.Quote(updatedDate.Format("2006-01-02")))
	}

	// Add featured image to frontmatter if available
	featuredImageFrontmatter := ""
	if post.FeaturedImage != "" {
		featuredImageFrontmatter = fmt.Sprintf("featuredImage: %s\n", strconv.Quote(post.FeaturedImage))
	}

	return fmt.Sprintf("---\ntitle: %s\nexcerpt: \"\"\npublishDate: %s\n%sisFeatured: false\ntags: %s\n%sseo: {}\n---\n\n",
		strconv.Quote(post.Title),
		strconv.Quote(publishDate.Format("2006-01-02")),
		updatedDateFrontmatter,
		tagsJSON,
		featuredImageFrontmatter,
	)
}

// processYouTubeShortcodes converts [youtube]URL[/youtube] shortcodes to YouTube components
func processYouTubeShortcodes(content string) string {
	result := content

	for {
		// Find the start of a YouTube shortcode
		startTag := "\\[youtube\\]"
		endTag := "\\[/youtube\\]"

		startIndex := strings.Index(result, startTag)
		if startIndex == -1 {
			break // No more shortcodes found
		}

		// Find the corresponding end tag
		endIndex := strings.Index(result[startIndex:], endTag)
		if endIndex == -1 {
			break // No matching end tag found
		}

		// Adjust endIndex to be relative to the full string
		endIndex += startIndex

		// Extract the URL between the tags
		url := result[startIndex+len(startTag) : endIndex]

		// Extract video ID from YouTube URL
		videoID := extractYouTubeVideoID(url)

		replacement := ""
		if videoID != "" {
			replacement = fmt.Sprintf("\n\n<YouTube id=\"https://youtu.be/%s\" />\n\n", videoID)
		} else {
			// If unable to extract video ID, keep original shortcode
			replacement = result[startIndex : endIndex+len(endTag)]
		}

		// Replace the shortcode with the YouTube component
		result = result[:startIndex] + replacement + result[endIndex+len(endTag):]
	}

	return result
}

// extractYouTubeVideoID extracts the video ID from various YouTube URL formats
func extractYouTubeVideoID(url string) string {
	// Handle youtu.be format
	if strings.Contains(url, "youtu.be/") {
		parts := strings.Split(url, "youtu.be/")
		if len(parts) > 1 {
			videoID := strings.Split(parts[1], "?")[0]
			videoID = strings.Split(videoID, "&")[0]
			return videoID
		}
	}

	// Handle youtube.com/watch?v= format
	if strings.Contains(url, "youtube.com/watch?v=") {
		parts := strings.Split(url, "v=")
		if len(parts) > 1 {
			videoID := strings.Split(parts[1], "&")[0]
			return videoID
		}
	}

	return ""
}
