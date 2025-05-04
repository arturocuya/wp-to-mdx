package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	html2md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
)

// ConvertHTMLToMarkdown converts HTML content to Markdown format
func ConvertHTMLToMarkdown(html string) (string, []string, error) {
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

	markdown, err := converter.ConvertString(html)
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
