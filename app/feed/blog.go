package feed

import (
	"encoding/xml"
	"fmt"
	"strings"

	"wapuugotchi/feed/app/ai"
)

const wordpressComFeedURL = "https://wordpress.com/blog/feed/"
const blogPattern = "Write a very brief summary in 1-2 sentences. Respond without HTML or Markdown. Text:\n\n%s"

type wordPressComFeed struct {
	Channel wordPressComChannel `xml:"channel"`
}

type wordPressComChannel struct {
	Items []wordPressComItem `xml:"item"`
}

type wordPressComItem struct {
	Title          string   `xml:"title"`
	Link           string   `xml:"link"`
	PubDate        string   `xml:"pubDate"`
	ContentEncoded string   `xml:"encoded"`
	Categories     []string `xml:"category"`
}

func LatestWordPressComBlog(fetch func(url, source string) ([]byte, error)) (Item, error) {
	body, err := fetch(wordpressComFeedURL, "wordpress com")
	if err != nil {
		return Item{}, err
	}

	var feed wordPressComFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return Item{}, err
	}
	if len(feed.Channel.Items) == 0 {
		return Item{}, nil
	}

	item := feed.Channel.Items[0]
	content := buildBlogContent(item.Title, item.ContentEncoded)
	return Item{
		Title:      item.Title,
		Link:       item.Link,
		PubDate:    item.PubDate,
		Content:    content,
		Categories: item.Categories,
	}, nil
}

func buildBlogContent(title, encoded string) string {
	title = strings.TrimSpace(title)
	body := strings.TrimSpace(encoded)
	summary := ""
	if body != "" {
		if result, err := ai.TransformText(blogPattern, body); err == nil {
			summary = strings.TrimSpace(result)
		}
	}
	if title == "" && summary == "" {
		return ""
	}
	if summary == "" {
		return fmt.Sprintf("<p><strong>%s</strong></p>", title)
	}
	return fmt.Sprintf("<p><strong>%s</strong></p><p>%s</p>", title, summary)
}
