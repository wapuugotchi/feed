package feed // Definiert das Paket "feed"; hier liegt die WordPress-TV-Feed-Logik.

import ( // Import-Block: Abhängigkeiten dieser Datei.
	"encoding/xml" // RSS-XML wird damit in Go-Structs unmarshalled.
	"fmt"          // Wird für HTML-String-Zusammenbau (Sprintf) genutzt.
	"regexp"       // Wird genutzt, um HTML-Teile (iframe/a) per Regex zu finden/ersetzen.
	"strings"      // Trimmen, Suchen, Ersetzen; robustes String-Handling.
)

const wordpressTVFeedURL = "https://wordpress.tv/feed/" // URL des WordPress.tv RSS-Feeds (Quelle für neueste Videos).

var ( // Globale, vorcompilierte Regexe: einmalig bauen (effizient) und mehrfach verwenden.
	iframePattern       = regexp.MustCompile(`(?is)<iframe\b[^>]*>.*?</iframe>`)
	// Findet den ersten kompletten <iframe ...>...</iframe>-Block (case-insensitive + dot matches newline).

	iframeWidthPattern  = regexp.MustCompile(`(?i)\swidth\s*=\s*(?:"[^"]*"|'[^']*'|[^'"\s>]+)`)
	// Findet width=... im iframe-Open-Tag, egal ob in "..." '...' oder unquoted (case-insensitive).

	iframeHeightPattern = regexp.MustCompile(`(?i)\sheight\s*=\s*(?:"[^"]*"|'[^']*'|[^'"\s>]+)`)
	// Findet height=... im iframe-Open-Tag; gleiche Logik wie width.

	iframeAllowPattern  = regexp.MustCompile(`(?i)\sallow\s*=\s*(?:"[^"]*"|'[^']*'|[^'"\s>]+)`)
	// Findet allow=... im iframe-Open-Tag; wichtig, um gewünschte Permissions zu setzen.

	anchorBlockPattern  = regexp.MustCompile(`(?is)<a\b[^>]*>.*?</a>`)
	// Findet komplette <a ...>...</a>-Blöcke (inkl. Inhalt) und kann sie komplett entfernen.

	anchorTagPattern    = regexp.MustCompile(`(?is)</?a\b[^>]*>`)
	// Findet nur die <a ...> und </a> Tags (ohne Inhalt), um "nur Tags" zu strippen.
)

type wordPressTVFeed struct { // Root-Struktur für das RSS-XML (minimaler Ausschnitt).
	Channel wordPressTVChannel `xml:"channel"` // Mappt <channel>...</channel> aus dem RSS.
}

type wordPressTVChannel struct { // Channel enthält die Items.
	Items []wordPressTVItem `xml:"item"` // Mappt alle <item>-Elemente in einen Slice.
}

type wordPressTVItem struct { // Struktur eines einzelnen WordPress.tv RSS-Items.
	Title          string   `xml:"title"`       // Titel des Videos/Posts.
	Link           string   `xml:"link"`        // Link zur WordPress.tv Seite.
	PubDate        string   `xml:"pubDate"`     // Veröffentlichungsdatum als RSS-String.
	Description    string   `xml:"description"` // Kurzbeschreibung (oft HTML, oft mit Links).
	ContentEncoded string   `xml:"encoded"`     // Vollcontent (häufig inkl. iframe embed).
	Categories     []string `xml:"category"`    // Kategorien/Tags.
}

func LatestWordPressTV(fetch func(url, source string) ([]byte, error)) (Item, error) {
	// Exportierte Funktion: holt den neuesten WordPress.tv Eintrag und mappt ihn ins interne Item-Format.
	// fetch wird injiziert, damit HTTP-Details zentral bleiben und Tests leicht sind.

	body, err := fetch(wordpressTVFeedURL, "wordpress tv")
	// Holt den RSS-Feed (Bytes). "wordpress tv" dient als Source-Label für Fehlertexte/Logging.

	if err != nil {
		// Wenn Fetch fehlschlägt (Netzwerk, Timeout, non-2xx)…
		return Item{}, err
		// …gibt leeres Item + Fehler zurück.
	}

	var feed wordPressTVFeed
	// Zielvariable für das XML-Unmarshal.

	if err := xml.Unmarshal(body, &feed); err != nil {
		// XML parsen; Fehler bei invalidem XML oder abweichender Struktur.
		return Item{}, err
	}

	if len(feed.Channel.Items) == 0 {
		// Feed hat keine Items → nichts zu liefern.
		return Item{}, nil
	}

	item := feed.Channel.Items[0]
	// Nimmt das erste Item als "latest" (Annahme: Feed ist absteigend sortiert, typisch für RSS).

	content := buildWordPressTVContent(item.Title, item.Description, item.ContentEncoded)
	// Baut den HTML-Content: Header (Titel/Beschreibung) + normalisiertes iframe + Entfernen von <a>-Tags.

	return Item{
		Title:      item.Title,      // Titel übernehmen.
		Link:       item.Link,       // Link übernehmen.
		PubDate:    item.PubDate,    // PubDate übernehmen (wird später normalisiert).
		Content:    content,         // Finaler HTML-Content.
		Categories: item.Categories, // Kategorien übernehmen.
	}, nil
	// Erfolgreich: standardisiertes Item zurück.
}

func buildWordPressTVContent(title, description, encoded string) string {
	// Baut finalen HTML-Content für RSS aus Title/Description/Encoded (Embed).

	title = strings.TrimSpace(title)
	// Titel normalisieren: kein führender/trailing Whitespace.

	description = stripAnchorTags(strings.TrimSpace(description))
	// Beschreibung trimmen und Links entfernen: verhindert externe/vertrackte Anchor-Tags im RSS.

	encoded = strings.TrimSpace(encoded)
	// Encoded-Inhalt trimmen: leer/whitespace-only wird sauber erkannt.

	header := ""
	// Header ist der Teil vor dem Video-Embed (Titel + Beschreibung als HTML).

	if title != "" {
		// Wenn ein Titel vorhanden ist…
		header = fmt.Sprintf("<p><strong>%s</strong></p>", title)
		// …baue einen fetten Titel-Absatz.
	}

	if description != "" {
		// Wenn es eine Beschreibung gibt…
		header += fmt.Sprintf("<p>%s</p>", description)
		// …hänge sie als eigenen Absatz an (mit bereits gestrippter Link-HTML).
	}

	if encoded == "" {
		// Wenn kein Encoded Content existiert (kein Embed)…
		return header
		// …gib nur den Header zurück.
	}

	normalized := normalizeFirstIframe(encoded)
	// Normalisiert das erste iframe im Encoded Content (width/height/allow).

	return header + stripAnchorTags(normalized)
	// Liefert Header + (iframe-normalisierter) Inhalt zurück und entfernt nochmals Anchor-Tags aus dem Encoded.
}

func normalizeFirstIframe(content string) string {
	// Sucht den ersten iframe-Block und ersetzt ihn durch eine normalisierte Version.

	match := iframePattern.FindString(content)
	// Holt den ersten String, der dem iframePattern entspricht (kompletter iframe-Block).

	if strings.TrimSpace(match) == "" {
		// Wenn kein iframe gefunden wurde…
		return content
		// …gib den Inhalt unverändert zurück.
	}

	normalized := normalizeIframe(match)
	// Normalisiert genau diesen iframe-Block (Attribute setzen/ersetzen).

	return strings.Replace(content, match, normalized, 1)
	// Ersetzt nur das erste Vorkommen (count=1), damit nur der erste iframe angepasst wird.
}

func normalizeIframe(value string) string {
	// Normalisiert Attribute im iframe-Open-Tag, ohne den Rest (Inhalt + closing tag) zu zerstören.

	if value == "" {
		// Defensive: leere Eingabe.
		return ""
		// Nichts zu normalisieren.
	}

	tagEnd := strings.Index(value, ">")
	// Findet das Ende des Opening-Tags "<iframe ...>": Position des ersten ">".

	if tagEnd == -1 {
		// Wenn kein ">" gefunden wird, ist das Tag vermutlich kaputt/unkomplett…
		return value
		// …dann lieber unverändert lassen.
	}

	openTag := value[:tagEnd]
	// Schneidet den Opening-Tag ohne ">" aus (z.B. "<iframe src=... width=...").

	rest := value[tagEnd:]
	// Rest enthält ">" + evtl. Inhalt + "</iframe>" und wird später wieder angehängt.

	if !strings.Contains(openTag, "<iframe") {
		// Defensive: falls der Split etwas Unerwartetes ergibt…
		return value
		// …nicht anfassen.
	}

	openTag = setAttr(openTag, "width", "100%", iframeWidthPattern)
	// Setzt/ersetzt width auf 100% → responsives Embed statt fixer Pixelbreite.

	openTag = setAttr(openTag, "height", "auto", iframeHeightPattern)
	// Setzt/ersetzt height auf auto → passt sich besser an Layout/Container an.

	openTag = setAttr(openTag, "allow", "autoplay; fullscreen; encrypted-media", iframeAllowPattern)
	// Setzt/ersetzt allow → sorgt dafür, dass Player-Funktionen (autoplay/fullscreen etc.) funktionieren.

	return openTag + rest
	// Rekonstruiert den iframe-Block mit modifiziertem Opening-Tag.
}

func setAttr(openTag, name, value string, pattern *regexp.Regexp) string {
	// Setzt ein Attribut im Opening-Tag: ersetzt es, wenn vorhanden; sonst fügt es hinzu.

	attr := fmt.Sprintf(` %s="%s"`, name, value)
	// Baut das gewünschte Attribut als String, inkl. führendem Leerzeichen (damit es sauber anfügbar ist).

	if pattern.MatchString(openTag) {
		// Wenn das Attribut schon existiert…
		return pattern.ReplaceAllString(openTag, attr)
		// …ersetze den bestehenden width/height/allow-Teil durch den gewünschten Wert.
	}

	return strings.TrimSpace(openTag) + attr
	// Sonst: trimmt openTag (gegen doppelte Spaces/Trailing Space) und hängt Attribut an.
	// Hinweis: hier fehlt bewusst das ">" — das wird später in normalizeIframe über "rest" wieder ergänzt.
}

func stripAnchorTags(content string) string {
	// Entfernt Anchor-Tags aus HTML: erst ganze <a>...</a>-Blöcke, dann verbleibende <a> oder </a>.

	if content == "" {
		// Defensive: leer rein → leer raus.
		return content
	}

	withoutBlocks := anchorBlockPattern.ReplaceAllString(content, "")
	// Entfernt komplette <a>...</a>-Blöcke inkl. Inhalt; hart, aber verhindert "verlinkte" Werbetexte o.ä.

	return anchorTagPattern.ReplaceAllString(withoutBlocks, "")
	// Entfernt verbliebene <a ...> und </a> Tags, falls sie nicht als kompletter Block erfasst wurden.
}
