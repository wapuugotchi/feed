package feed // Paket "feed": enthält Funktionen, die externe Feeds abrufen und in dein internes Item-Format umwandeln.

import ( // Import-Block: Abhängigkeiten dieser Datei.
	"encoding/xml" // Wird gebraucht, um den RSS-Feed (XML) in Go-Structs zu parsen (Unmarshal).
	"fmt"          // Wird genutzt, um HTML-Strings via Sprintf zu bauen (Titel + Summary).
	"strings"      // Wird genutzt, um Whitespace zu trimmen und leere Inhalte zuverlässig zu erkennen.

	"wapuugotchi/feed/app/ai" // Eigenes Paket: ruft KI-Provider auf, um Text zu transformieren/zusammenzufassen.
)

const wordpressComFeedURL = "https://wordpress.com/blog/feed/" // Konstante URL: Quelle für den WordPress.com Blog RSS-Feed.
const blogPattern = "Write a very brief summary in 1-2 sentences. Respond without HTML or Markdown. Text:\n\n%s" // Prompt-Template: erzwingt kurze Plain-Text-Zusammenfassung ohne Formatierung.

type wordPressComFeed struct { // Root-Struktur für RSS-XML (minimal: nur channel wird benötigt).
	Channel wordPressComChannel `xml:"channel"` // Mappt das <channel>-Element auf dieses Feld.
}

type wordPressComChannel struct { // Struktur für den RSS-Channel-Block.
	Items []wordPressComItem `xml:"item"` // Mappt alle <item>-Elemente (Posts) in einen Slice.
}

type wordPressComItem struct { // Struktur für ein einzelnes RSS-Item aus dem WordPress.com Blog Feed.
	Title          string   `xml:"title"`    // Titel des Blogposts.
	Link           string   `xml:"link"`     // Link zum Blogpost.
	PubDate        string   `xml:"pubDate"`  // Veröffentlichungsdatum (RSS-String, wird später normalisiert).
	ContentEncoded string   `xml:"encoded"`  // Voller Inhalt (oft HTML), im Feed als "encoded" geliefert.
	Categories     []string `xml:"category"` // Kategorien/Tags des Posts.
}

func LatestWordPressComBlog(fetch func(url, source string) ([]byte, error)) (Item, error) { // Exportierte Funktion: liefert das neueste Blog-Item im internen Format.
	body, err := fetch(wordpressComFeedURL, "wordpress com") // Ruft Feed per HTTP ab; source-Label dient Fehlerkontext/Logging.
	if err != nil { // Wenn Fetch fehlschlägt (Timeout, Status, Netzwerk)…
		return Item{}, err // …leeres Item + Fehler zurückgeben.
	}

	var feed wordPressComFeed // Zielvariable für XML-Parsing.
	if err := xml.Unmarshal(body, &feed); err != nil { // Unmarshal XML → Structs; Fehler bei invalidem XML oder Strukturänderungen.
		return Item{}, err // Fehler weitergeben, weil ohne Parse kein Item extrahierbar ist.
	}
	if len(feed.Channel.Items) == 0 { // Wenn der Feed keine Items enthält…
		return Item{}, nil // …kein Fehler, aber nichts zu liefern.
	}

	item := feed.Channel.Items[0] // Nimmt das erste Item als "latest" (Annahme: Feed ist absteigend sortiert, üblich bei RSS).
	content := buildBlogContent(item.Title, item.ContentEncoded) // Baut HTML-Description: Titel + KI-Zusammenfassung des Inhalts.
	return Item{ // Mappt WordPress.com Item auf dein internes Item-Struct.
		Title:      item.Title,      // Titel übernehmen.
		Link:       item.Link,       // Link übernehmen.
		PubDate:    item.PubDate,    // PubDate übernehmen (wird später geparsed/normalisiert).
		Content:    content,         // Generierter Content (HTML).
		Categories: item.Categories, // Kategorien übernehmen.
	}, nil // Erfolgreich zurückgeben.
}

func buildBlogContent(title, encoded string) string { // Hilfsfunktion: baut den HTML-Content aus Titel und (KI-)Summary.
	title = strings.TrimSpace(title) // Titel trimmen, damit " " nicht als echter Titel zählt.
	body := strings.TrimSpace(encoded) // Body trimmen, um leere/Whitespace-only Inhalte zu erkennen.
	summary := "" // Default: keine Zusammenfassung.
	if body != "" { // Nur wenn Body vorhanden ist, lohnt sich der KI-Call.
		if result, err := ai.TransformText(blogPattern, body); err == nil { // KI transformiert Body nach blogPattern; Fehler wird bewusst ignoriert.
			summary = strings.TrimSpace(result) // Ergebnis trimmen; verhindert führende/trailing Newlines/Spaces.
		}
	}
	if title == "" && summary == "" { // Wenn weder Titel noch Summary vorhanden sind…
		return "" // …liefere leeren Content (Caller kann Entry ggf. droppen/ignorieren).
	}
	if summary == "" { // Wenn keine Summary erzeugt wurde (z.B. KI-Fehler oder Body leer), aber Titel existiert…
		return fmt.Sprintf("<p><strong>%s</strong></p>", title) // …liefere wenigstens den Titel als HTML.
	}
	return fmt.Sprintf("<p><strong>%s</strong></p><p>%s</p>", title, summary) // Standardfall: Titel fett + Summary als Absatz.
}
