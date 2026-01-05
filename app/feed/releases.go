package feed

import (
	"encoding/xml"
	"strings"

	"wapuugotchi/feed/app/ai"
)

const releasesFeedURL = "https://wordpress.org/news/category/releases/feed/"
const releasesPattern = "Extract key highlights from the text below. Output RAW HTML only. Do NOT escape HTML characters. Do NOT output JSON. Use literal < > characters, not unicode (e.g. < not \\u003c). Output must be a single line with no line breaks. Format EXACTLY: <p><strong>WordPress ###VERSION### is here!</strong></p><p>###Description###</p><ul><li><strong>###TITLE_HIGHLIGHT_1:###</strong> TEXT_HIGHLIGHT_1</li><li><strong>###TITLE_HIGHLIGHT_2:###</strong> TEXT_HIGHLIGHT_2</li><li><strong>###TITLE_HIGHLIGHT_n:###</strong> TEXT_HIGHLIGHT_n</li></ul> Description must be one short sentence (max 60 characters), high-level, and must not repeat the headline. Text:\n\n%s"

type Item struct {
	Title      string
	Link       string
	PubDate    string
	Content    string
	Categories []string
}

type wordPressFeed struct {
	Channel wordPressChannel `xml:"channel"`
}

type wordPressChannel struct {
	Items []wordPressItem `xml:"item"`
}

type wordPressItem struct {
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	PubDate     string   `xml:"pubDate"`
	Description string   `xml:"description"`
	Categories  []string `xml:"category"`
}

func LatestReleases(fetch func(url, source string) ([]byte, error)) (Item, error) {
	body, err := fetch(releasesFeedURL, "wordpress releases")
	if err != nil {
		return Item{}, err
	}

	var feed wordPressFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return Item{}, err
	}
	if len(feed.Channel.Items) == 0 {
		return Item{}, nil
	}

	item := feed.Channel.Items[0]
	content := buildReleasesContent(item.Description)
	return Item{
		Title:      item.Title,
		Link:       item.Link,
		PubDate:    item.PubDate,
		Content:    content,
		Categories: item.Categories,
	}, nil
}

func buildReleasesContent(description string) string {
	content := strings.TrimSpace(description)
	if content == "" {
		return ""
	}
	rendered, err := ai.TransformText(releasesPattern, content)
	if err != nil {
		return content
	}
	return rendered
}
