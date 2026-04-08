package feed // Definiert das Paket "feed"; enthält Logik zum Abrufen/Transformieren von RSS-Feed-Inhalten.

import ( // Import-Block: Abhängigkeiten dieser Datei.
	"encoding/xml" // Wird gebraucht, um RSS-XML (WordPress Releases Feed) in Go-Structs zu unmarshallen.
	"strings"      // Wird verwendet, um Whitespace zu trimmen und leere Inhalte sauber zu erkennen.

	"wapuugotchi/feed/app/ai" // Eigenes KI-Paket: transformiert Rohtext mit einem Prompt in gewünschtes Ausgabeformat.
)

const releasesFeedURL = "https://wordpress.org/news/category/releases/feed/"
// URL des WordPress.org News-Releases RSS-Feeds; hier kommen neue Release-Posts her.

const releasesPattern = "You are given a WordPress release announcement. Extract the version, a one-sentence summary, and 2-4 key highlights written for a WordPress site administrator.\n\nImportant: If the release is a Release Candidate (RC), Beta, or any pre-release, always include the full label in the headline (e.g. \"WordPress 7.0 RC2 is here!\" not \"WordPress 7.0 is here!\").\n\nOutput raw HTML on a single line. No markdown, no code blocks, no extra text. Use literal < and > characters.\n\nFollow this structure exactly:\n<p><strong>WordPress 6.5 is here!</strong></p><p>A major release packed with new features and improvements.</p><ul><li><strong>Block Bindings API:</strong> Connect blocks directly to custom data sources.</li><li><strong>Font Library:</strong> Install and manage fonts from the editor.</li></ul>\n\nNow do the same for this text:\n\n%s"
// Prompt-Template: nutzt ein konkretes Beispiel statt abstrakter Platzhalter-Syntax.
// und du vermutlich wirklich HTML im RSS <description> ausliefern willst, nicht escaped Entities.

type Item struct { // Internes, vereinheitlichtes Item-Format für dein Aggregationssystem (wird von mehreren Quellen genutzt).
	Title      string   // Titel der Nachricht (z.B. "WordPress 6.x released").
	Link       string   // Link zur Originalquelle.
	PubDate    string   // Veröffentlichungsdatum als String (RSS-Format), später anderswo geparsed/normalisiert.
	Content    string   // Inhalt/Description, hier typischerweise HTML (entweder KI-rendered oder Fallback-Text).
	Categories []string // Kategorien/Tags aus dem Feed (optional).
}

type wordPressFeed struct { // Repräsentiert das Root-Level des RSS-Dokuments (vereinfacht auf das, was du brauchst).
	Channel wordPressChannel `xml:"channel"` // Mappt das <channel>-Element auf dieses Feld.
}

type wordPressChannel struct { // Repräsentiert <channel>, in dem die <item>-Elemente liegen.
	Items []wordPressItem `xml:"item"` // Alle einzelnen Feed-Items (Posts) aus dem RSS.
}

type wordPressItem struct { // Repräsentiert ein einzelnes <item> im WordPress Releases Feed.
	Title       string   `xml:"title"`       // Mappt <title> → Titel des Posts.
	Link        string   `xml:"link"`        // Mappt <link> → URL zum Post.
	PubDate     string   `xml:"pubDate"`     // Mappt <pubDate> → Veröffentlichungsdatum (RSS-String).
	Description string   `xml:"description"` // Mappt <description> → Inhalt (oft HTML/CDATAsnippet).
	Categories  []string `xml:"category"`    // Mappt <category> (mehrfach) → Slice von Kategorien/Tags.
}

func LatestReleases(fetch func(url, source string) ([]byte, error)) (Item, error) {
	// Exportierte Funktion: holt den neuesten WordPress Release-Post und gibt ihn als internes Item zurück.
	// fetch wird injiziert (Dependency Injection), damit HTTP-Handling/Retry/Headers zentral bleibt und testbar ist.

	body, err := fetch(releasesFeedURL, "wordpress releases")
	// Ruft den Feed per HTTP ab; "wordpress releases" dient typischerweise für Fehlermeldungen/Logging im fetch.

	if err != nil {
		// Wenn HTTP-Fetch scheitert (Timeout, non-2xx, Netzwerk)…
		return Item{}, err
		// …weiterreichen: hier kann man ohne Body nichts sinnvoll machen.
	}

	var feed wordPressFeed
	// Zielvariable für das XML-Unmarshal: danach steht feed.Channel.Items gefüllt da (oder leer).

	if err := xml.Unmarshal(body, &feed); err != nil {
		// Parst das RSS-XML in die Structs; scheitert bei ungültigem XML oder Strukturabweichungen.
		return Item{}, err
		// Fehler weitergeben: ohne valide Struktur weißt du nicht, was "latest" ist.
	}

	if len(feed.Channel.Items) == 0 {
		// Falls der Feed keine Items enthält (z.B. temporär leer oder parse lieferte nichts)…
		return Item{}, nil
		// …kein Fehler: bedeutet schlicht "kein neuer Content verfügbar".
	}

	item := feed.Channel.Items[0]
	// Nimmt das erste Item als "latest"; setzt voraus, dass der RSS-Feed absteigend sortiert ist (üblich bei RSS).

	content := buildReleasesContent(item.Description)
	// Baut den Content: entweder KI-formatiertes RAW-HTML oder Fallback auf Original-Description.

	return Item{
		Title:      item.Title,      // Übernimmt Titel aus dem Feed.
		Link:       item.Link,       // Übernimmt Link aus dem Feed.
		PubDate:    item.PubDate,    // Übernimmt PubDate-String unverändert (wird später normalisiert).
		Content:    content,         // Setzt erzeugten Content (KI oder Fallback).
		Categories: item.Categories, // Übernimmt Kategorien aus dem Feed.
	}, nil
	// Erfolgreiche Rückgabe: ein "standardisiertes" Item für den Aggregator.
}

func buildReleasesContent(description string) string {
	// Hilfsfunktion: verarbeitet den description-Text (typisch HTML) und versucht per KI ein strikt formatiertes HTML zu erzeugen.

	content := strings.TrimSpace(description)
	// Trim: verhindert, dass Whitespace-only Descriptions als "Content vorhanden" zählen.

	if content == "" {
		// Wenn nach Trim kein Inhalt übrig bleibt…
		return ""
		// …liefer leer zurück: upstream kann dann Entry ggf. droppen oder minimal ausgeben.
	}

	rendered, err := ai.TransformText(releasesPattern, content)
	// Übergibt den Rohtext an die KI mit einem sehr strikten Prompt (RAW HTML, genaues Format, einzeilig).

	if err != nil {
		// Wenn die KI scheitert (Netzwerk, Rate Limit, Parsing, Modellfehler)…
		return content
		// …Fallback: lieber Original-Description als gar nichts, damit der Feed nicht leer wird.
	}

	return rendered
	// Erfolgsfall: KI-generiertes RAW-HTML zurückgeben (entspricht dem gewünschten Layout).
}
