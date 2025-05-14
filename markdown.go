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

					imageURLs = append(imageURLs, src)
					markdown := fmt.Sprintf("\n\n<img src=\"%s\" alt=\"%s\" />\n\n", src, alt)
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
