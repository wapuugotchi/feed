package cmd // Paketname: gruppiert diesen Code als Teil des "cmd"-Pakets (typisch für CLI/Commands).

import ( // Import-Block: alles, was dieser File aus der Standardlib + eigenen Modulen braucht.
	"crypto/md5"    // Für stabile Hash-IDs (Entry-ID) aus Text; wichtig fürs Deduplizieren.
	"encoding/json" // JSON lesen/schreiben (site.json, entries.json).
	"encoding/xml"  // RSS-XML generieren (feed.xml).
	"fmt"           // Formatierte Ausgabe + Fehlertexte.
	"io"            // io.Copy/io.Discard + io.ReadAll: Response-Body handhaben.
	"net/http"      // HTTP-Client zum Abrufen der Feeds.
	"os"            // Dateisystem + Stdout/Stderr + Exit.
	"path/filepath" // OS-sichere Pfad-Konstruktion.
	"sort"          // Sortieren der Entries nach Datum.
	"strings"       // Trimmen/Normalisieren von Strings, wichtig bei Input aus Feeds.
	"time"          // Zeitparser + Formate + Timeouts + Backoff.

	"wapuugotchi/feed/app/env"
	"wapuugotchi/feed/app/feed" // Dein internes Paket: liefert "Latest..."-Fetcher und feed.Item Typ.
) // Ende Import-Block.

type Site struct { // Konfiguration/Metadaten deines eigenen RSS-Feeds.
	Title       string `json:"title"`       // Feed-Titel; JSON-Tag: Schlüssel heißt "title".
	Link        string `json:"link"`        // Feed-Link; wichtig für RSS-Consumers.
	Description string `json:"description"` // Feed-Beschreibung; RSS Pflicht/üblich.
} // Ende struct Site.

type Entry struct { // Persistierte Entry-Struktur (entries.json) für deinen Aggregator.
	ID         string   `json:"id"`                   // Eindeutige ID; benutzt zur Deduplizierung.
	Source     string   `json:"source,omitempty"`     // Quelle des Eintrags (z.B. wordpress-releases oder article).
	Title      string   `json:"title"`                // Titel der Entry.
	Link       string   `json:"link"`                 // URL zum Original.
	Content    string   `json:"content"`              // Inhalt/Description im RSS.
	Iframe     string   `json:"iframe,omitempty"`     // Optionales Embed; im RSS aktuell nicht genutzt.
	CreatedAt  string   `json:"created_at"`           // ISO/RFC3339 Zeitstempel als String (leicht zu speichern).
	Categories []string `json:"categories,omitempty"` // Optional: Kategorien/Tags; omitempty spart JSON wenn leer.
} // Ende struct Entry.

type RSS struct { // Root-Objekt für RSS 2.0 XML.
	XMLName xml.Name `xml:"rss"`          // Setzt Root-Tag <rss>.
	Version string   `xml:"version,attr"` // RSS-Version als Attribut: version="2.0".
	Channel Channel  `xml:"channel"`      // Enthält <channel>...</channel>.
} // Ende struct RSS.

type Channel struct { // RSS Channel: Metadaten + Items.
	Title         string `xml:"title"`                   // <title> im RSS.
	Link          string `xml:"link"`                    // <link> im RSS.
	Description   string `xml:"description"`             // <description> im RSS.
	LastBuildDate string `xml:"lastBuildDate,omitempty"` // Optionaler Build-Zeitpunkt; omitempty => weglassen wenn leer.
	Items         []Item `xml:"item"`                    // Liste der <item> Elemente.
} // Ende struct Channel.

type Item struct { // RSS Item: einzelne Nachricht/Eintrag.
	ID          string   `xml:"id"`                 // Nicht standard-RSS Feld (typisch wäre guid); bei dir <id>.
	Title       string   `xml:"title"`              // <title>
	Link        string   `xml:"link"`               // <link>
	PubDate     string   `xml:"pubDate"`            // <pubDate> im RFC1123(Z) Format.
	Description string   `xml:"description"`        // <description> (bei dir Content).
	Iframe      string   `xml:"iframe,omitempty"`   // Optionales <iframe>-Feld (custom XML).
	Categories  []string `xml:"category,omitempty"` // <category> mehrfach möglich; weglassen wenn leer.
} // Ende struct Item.

type Paths struct { // Kleine Struktur: bündelt zusammengehörige Dateipfade.
	site     string // Pfad zu site.json.
	entries  string // Pfad zu entries.json.
	articles string // Pfad zu Artikeldateien (manuelle Inhalte).
	feed     string // Pfad zur Ausgabe feed.xml.
} // Ende struct paths.

const ( // Konstanten: zentrale HTTP Header-Defaults.
	userAgent        = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36" // Tarnung/Kompatibilität; manche Server blocken Default-Go-Agent.
	acceptHeader     = "application/rss+xml, application/xml;q=0.9, text/xml;q=0.8, */*;q=0.7"                                           // Akzeptierte Response-Formate; hilft bei Content Negotiation.
	releasesProvider = "wordpress-releases"
	articlesSource   = "article"
) // Ende const.

func RunFeedUpdate() error { // Hauptfunktion: lädt Daten, holt neue Items, schreibt files, baut feed.xml.
	paths, err := getPaths() // Ermittelt Pfade für site.json, entries.json, feed.xml relativ zum CWD.
	if err != nil {          // Wenn getPaths scheitert (z.B. kein CWD), abbrechen.
		return err // Fehler nach außen geben.
	} // Ende error-check.

	site := loadSite(paths.site)          // Lädt Site-Metadaten; liefert Defaults wenn Datei fehlt.
	entries := loadEntries(paths.entries) // Lädt bisher bekannte Einträge (für Dedupe + Historie).

	updated := false                       // Flag: ob neue Entries hinzugekommen sind.
	for _, provider := range providers() { // Iteriert über alle Feed-Quellen (provider).
		added, err := addLatest(provider, &entries) // Holt "latest item" pro Provider und fügt es ggf. hinzu.
		if err != nil {                             // Wenn dieser Provider fehlschlägt…
			fmt.Fprintln(os.Stderr, err) // …Fehler loggen, aber nicht den gesamten Run abbrechen.
			continue                     // Weiter mit nächstem Provider.
		} // Ende provider-error.
		if added { // Wenn tatsächlich ein neuer Entry hinzugefügt wurde…
			updated = true // …merken, dass wir speichern + XML rebuilden müssen.
		} // Ende added-check.
	} // Ende provider-loop.
	if updated {
		saveEntries(paths.entries, entries) // Persistiert aktualisierte entries.json.
		fmt.Println("provider update detected")
	} else {
		fmt.Println("no provider update detected")
	}

	manualArticles := loadArticleEntries(paths.articles)
	allEntries := mergeEntries(entries, manualArticles)

	if err := buildFeed(site, allEntries, paths.feed); err != nil { // Baut feed.xml neu (RSS).
		return err // Fehler beim Schreiben/Encoding nach außen geben.
	} // Ende buildFeed error-check.

	fmt.Println("feed rebuilt") // Ausgabe: Feed wurde neu erstellt.
	return nil                  // Erfolg.
} // Ende RunFeedUpdate.

type feedProvider struct { // Abstraktion einer Quelle: Name + Fetch-Funktion.
	Name  string                                                                  // Name wird u.a. in ID-Hash einbezogen (stabil pro Quelle).
	Fetch func(fetch func(url, source string) ([]byte, error)) (feed.Item, error) // Fetcher nimmt eine fetch-Funktion (Dependency Injection) und liefert ein feed.Item.
} // Ende struct feedProvider.

func providers() []feedProvider { // Liefert die Liste der Quellen, die abgefragt werden sollen.
	return []feedProvider{ // Slice-Literal: Reihenfolge ist die Abfrage-Reihenfolge.
		{Name: releasesProvider, Fetch: feed.LatestReleases}, // Quelle 1: WordPress Releases.
		// 		{Name: "wordpress-tv", Fetch: feed.LatestWordPressTV},       // Quelle 2: WordPress TV.
		// 		{Name: "wordpress-com", Fetch: feed.LatestWordPressComBlog}, // Quelle 3: WordPress.com Blog.
	} // Ende Slice.
} // Ende providers.

func getPaths() (Paths, error) { // Ermittelt, wo Dateien liegen sollen (relativ zum Working Directory).
	root, err := os.Getwd() // Holt das aktuelle Arbeitsverzeichnis.
	if err != nil {         // Falls das nicht geht (selten, aber möglich)…
		return Paths{}, err // …leere paths + Fehler zurück.
	} // Ende error-check.
	dataDir := filepath.Join(root, "data") // Baut data/ Pfad OS-sicher zusammen.
	return Paths{                          // Gibt alle Pfade zurück.
		site:     filepath.Join(dataDir, "site.json"),    // data/site.json
		entries:  filepath.Join(dataDir, "entries.json"), // data/entries.json
		articles: filepath.Join(root, "articles"),        // articles/ (manuell gepflegte Beiträge)
		feed:     filepath.Join(root, "feed.xml"),        // feed.xml im Projektroot.
	}, nil // Kein Fehler.
} // Ende getPaths.

func loadSite(path string) Site { // Lädt Site-Infos mit sinnvollem Default.
	site := Site{Title: "Wapuugotchi RSS"} // Default-Wert; wichtig falls site.json fehlt/leer ist.
	if fillSiteFromEnv(&site) {
		return site
	}
	readJSON(path, &site) // Versucht zu überschreiben; bei Fehlern macht readJSON einfach nichts.
	return site           // Gibt Site zurück (Default oder geladen).
} // Ende loadSite.

func loadEntries(path string) []Entry { // Lädt gespeicherte Entries.
	entries := []Entry{}     // Default: leerer Slice (kein nil).
	readJSON(path, &entries) // Bei fehlender Datei bleibt es leer.
	return entries           // Return.
} // Ende loadEntries.

func saveEntries(path string, entries []Entry) { // Speichert Entries nach JSON.
	writeJSON(path, entries) // Zentralisierte JSON-Ausgabe + Fehlerbehandlung (exit).
} // Ende saveEntries.

func fillSiteFromEnv(site *Site) bool { // Lädt alle env variablen
	title := env.ReadEnv("FEED_TITLE")
	link := env.ReadEnv("FEED_LINK")
	description := env.ReadEnv("FEED_DESCRIPTION")
	site.Title = title
	site.Link = link
	site.Description = description
	if title == "" && link == "" && description == "" { // Prüft, ob *alle* Werte leer sind - Falls ja, signalisiert die Funktion: "Es wurde keine sinnvolle Konfiguration gefunden"
		return false
	}
	return true // Alle Werte sind gesetzt

} // Ende fillSiteFromEnv.

func addLatest(provider feedProvider, entries *[]Entry) (bool, error) { // Holt neuesten Item eines Providers und fügt ihn ggf. hinzu.
	item, err := provider.Fetch(fetchFeed) // Provider-Fetcher aufrufen; bekommt fetchFeed als HTTP-Funktion.
	if err != nil {                        // Wenn Fetch scheitert…
		return false, err // …nichts hinzugefügt + Fehler.
	} // Ende error-check.
	if strings.TrimSpace(item.Title) == "" { // Wenn Item ohne Titel kommt…
		return false, nil // …ignorieren: vermutlich ungültig/leer.
	} // Ende title-check.

	item.Categories = cleanCategories(item.Categories) // Kategorien trimmen + leere entfernen.
	id := pickEntryID(provider.Name, item)             // Stabile ID aus Provider + PubDate/Link generieren.
	newEntry := Entry{
		ID:         id,
		Source:     provider.Name,
		Title:      item.Title,
		Link:       item.Link,
		Content:    item.Content,
		CreatedAt:  pickEntryTime(item),
		Categories: item.Categories,
	}

	if provider.Name == releasesProvider {
		return replaceWithLatestRelease(entries, newEntry), nil
	}

	if idExists(*entries, id) { // Prüfen, ob diese ID schon vorhanden ist.
		return false, nil // Wenn ja: kein Update.
	} // Ende exists-check.

	*entries = append(*entries, newEntry) // Neuen Entry an den Slice anhängen (über Pointer mutieren).
	return true, nil                      // Es wurde etwas hinzugefügt.
} // Ende addLatest.

func replaceWithLatestRelease(entries *[]Entry, latest Entry) bool {
	if len(*entries) == 1 && isReleaseEntry((*entries)[0]) && (*entries)[0].ID == latest.ID {
		return false
	}
	*entries = []Entry{latest}
	return true
}

func isReleaseEntry(entry Entry) bool {
	if strings.TrimSpace(entry.Source) == releasesProvider {
		return true
	}
	for _, category := range entry.Categories {
		switch strings.ToLower(strings.TrimSpace(category)) {
		case "release", "releases":
			return true
		}
	}
	return false
}

func mergeEntries(base, extra []Entry) []Entry {
	if len(extra) == 0 {
		return base
	}
	merged := append([]Entry{}, base...)
	knownIDs := make(map[string]struct{}, len(merged))
	for _, entry := range merged {
		knownIDs[entry.ID] = struct{}{}
	}
	for _, entry := range extra {
		if strings.TrimSpace(entry.Title) == "" {
			continue
		}
		if strings.TrimSpace(entry.CreatedAt) == "" {
			continue
		}
		if _, exists := knownIDs[entry.ID]; exists {
			continue
		}
		knownIDs[entry.ID] = struct{}{}
		merged = append(merged, entry)
	}
	return merged
}

func loadArticleEntries(dir string) []Entry {
	list, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	entries := make([]Entry, 0, len(list))
	for _, file := range list {
		if file.IsDir() || !strings.HasSuffix(strings.ToLower(file.Name()), ".json") {
			continue
		}

		path := filepath.Join(dir, file.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var entry Entry
		if err := json.Unmarshal(data, &entry); err != nil {
			continue
		}

		entry.Title = strings.TrimSpace(entry.Title)
		entry.Link = strings.TrimSpace(entry.Link)
		entry.Content = strings.TrimSpace(entry.Content)
		entry.Iframe = strings.TrimSpace(entry.Iframe)
		entry.CreatedAt = strings.TrimSpace(entry.CreatedAt)
		entry.Categories = cleanCategories(entry.Categories)
		entry.Source = articlesSource

		if entry.Title == "" || entry.CreatedAt == "" {
			continue
		}
		if _, err := parseTime(entry.CreatedAt); err != nil {
			continue
		}
		if strings.TrimSpace(entry.ID) == "" {
			entry.ID = hashString("article|" + file.Name() + "|" + entry.Link + "|" + entry.CreatedAt)
		}

		entries = append(entries, entry)
	}
	return entries
}

func fetchFeed(url, source string) ([]byte, error) { // HTTP Fetch helper mit Retry auf 429.
	client := &http.Client{Timeout: 15 * time.Second} // Client mit Timeout; schützt vor Hängern.

	var body []byte                            // Hier landet der Response-Body.
	for attempt := 0; attempt < 2; attempt++ { // Max 2 Versuche: 1 normal + 1 Retry bei 429.
		req, err := http.NewRequest(http.MethodGet, url, nil) // Request bauen.
		if err != nil {                                       // Wenn URL kaputt o.ä.
			return nil, err // Direkt zurück.
		} // Ende error-check.
		req.Header.Set("User-Agent", userAgent) // Setzt User-Agent.
		req.Header.Set("Accept", acceptHeader)  // Setzt Accept Header.

		resp, err := client.Do(req) // Request ausführen.
		if err != nil {             // Netzwerkfehler, DNS, Timeout, etc.
			return nil, err // Zurückgeben.
		} // Ende error-check.

		if resp.StatusCode == http.StatusTooManyRequests && attempt == 0 { // Wenn 429 und wir sind beim ersten Versuch…
			_, _ = io.Copy(io.Discard, resp.Body) // Body leeren, damit Keep-Alive sauber ist (best practice).
			resp.Body.Close()                     // Body schließen (wichtig: Ressourcen frei).
			time.Sleep(2 * time.Second)           // Kurzer Backoff bevor Retry.
			continue                              // Nächster Versuch.
		} // Ende 429-Handling.

		if resp.StatusCode < 200 || resp.StatusCode >= 300 { // Alles außerhalb 2xx als Fehler behandeln.
			resp.Body.Close()                                                // Body schließen, sonst Leak.
			return nil, fmt.Errorf("%s api status: %s", source, resp.Status) // Fehler mit Quelle + Status.
		} // Ende status-check.

		body, err = io.ReadAll(resp.Body) // Body vollständig lesen.
		resp.Body.Close()                 // Immer schließen, auch bei Erfolg.
		if err != nil {                   // Falls ReadAll fehlschlägt…
			return nil, err // …Fehler zurück.
		} // Ende read error-check.
		break // Erfolgreich gelesen: Retry-Schleife verlassen.
	} // Ende retry-loop.

	return body, nil // Gibt Response-Bytes zurück.
} // Ende fetchFeed.

func cleanCategories(values []string) []string { // Entfernt Whitespace + leere Kategorien.
	result := make([]string, 0, len(values)) // Prealloc: spart Reallocs, max so groß wie input.
	for _, value := range values {           // Über alle Kategorien iterieren.
		value = strings.TrimSpace(value) // Whitespace entfernen.
		if value == "" {                 // Leere Einträge rausfiltern.
			continue // Skip.
		} // Ende empty-check.
		result = append(result, value) // Saubere Kategorie hinzufügen.
	} // Ende loop.
	return result // Ergebnis zurück.
} // Ende cleanCategories.

func buildFeed(site Site, entries []Entry, outputPath string) error { // Baut feed.xml aus Site + Entries.
	sort.Slice(entries, func(i, j int) bool { // Sortiert Entries absteigend nach CreatedAt-String.
		return entries[i].CreatedAt > entries[j].CreatedAt // Stringvergleich funktioniert bei RFC3339 (lexikographisch = chronologisch).
	}) // Ende sort.

	channel := Channel{ // Channel-Metadaten setzen.
		Title:       site.Title,       // Feed Titel.
		Link:        site.Link,        // Feed Link.
		Description: site.Description, // Feed Beschreibung.
	} // Ende channel init.

	if len(entries) > 0 { // Wenn mindestens ein Entry existiert…
		last, err := parseTime(entries[0].CreatedAt) // Nimmt den neuesten (nach Sort) und parsed RFC3339.
		if err == nil {                              // Wenn parse klappt…
			channel.LastBuildDate = last.UTC().Format(time.RFC1123Z) // lastBuildDate in RSS-übliches Format.
		} // Ende parse success.
	} // Ende entries-check.

	for _, entry := range entries { // Alle Entries in RSS-Items umwandeln.
		createdAt, err := parseTime(entry.CreatedAt) // CreatedAt parsen.
		if err != nil {                              // Wenn kaputt…
			continue // Entry überspringen (besser als kompletten Feed kaputt machen).
		} // Ende parse error.
		channel.Items = append(channel.Items, Item{ // Item hinzufügen.
			Title:       entry.Title,                           // Titel.
			Link:        entry.Link,                            // Link.
			ID:          entry.ID,                              // ID (bei dir <id>).
			PubDate:     createdAt.UTC().Format(time.RFC1123Z), // pubDate in RFC1123Z.
			Description: entry.Content,                         // description = content.
			Iframe:      strings.TrimSpace(entry.Iframe),       // Optionales iframe-Feld.
			Categories:  entry.Categories,                      // Kategorien.
		}) // Ende append.
	} // Ende loop.

	rss := RSS{ // RSS Root erstellen.
		Version: "2.0",   // RSS Version setzen.
		Channel: channel, // Channel einhängen.
	} // Ende rss init.

	file, err := os.Create(outputPath) // Zieldatei erstellen/überschreiben.
	if err != nil {                    // Wenn das nicht geht (Permission, Pfad)…
		return err // …Fehler zurück.
	} // Ende error-check.
	defer file.Close() // Sicherstellen, dass Datei am Ende geschlossen wird.

	if _, err := file.WriteString(xml.Header); err != nil { // XML Header schreiben (<?xml version="1.0"...>).
		return err // Fehler zurück.
	} // Ende header write.

	enc := xml.NewEncoder(file) // XML-Encoder, der direkt in die Datei schreibt.
	enc.Indent("", "  ")        // Pretty Print: Einrückung für Lesbarkeit.
	return enc.Encode(rss)      // RSS struct als XML schreiben; gibt ggf. error zurück.
} // Ende buildFeed.

func parseTime(value string) (time.Time, error) { // Erwartet RFC3339 timestamps (CreatedAt).
	return time.Parse(time.RFC3339, strings.TrimSpace(value)) // Trimmt und parsed.
} // Ende parseTime.

func pickEntryID(provider string, item feed.Item) string { // Generiert ID stabil anhand PubDate/Link.
	base := strings.TrimSpace(item.PubDate) // Primär: PubDate als Basis (stabil bei Feeds).
	if base == "" {                         // Wenn PubDate fehlt…
		base = strings.TrimSpace(item.Link) // …nutze Link als Basis.
	} // Ende fallback.
	if base == "" { // Wenn auch Link fehlt…
		base = fmt.Sprintf("%s-%d", provider, time.Now().UnixNano()) // …notfalls Zufallsbasis: Provider + Zeit.
	} // Ende final fallback.
	return hashString(provider + "|" + base) // Hash reduziert Länge + normalisiert; Provider trennt gleiche Daten zwischen Quellen.
} // Ende pickEntryID.

func pickEntryTime(item feed.Item) string { // Ermittelt CreatedAt aus PubDate, fallback now.
	parsed, err := parsePubDate(item.PubDate) // Versucht, PubDate in Time zu parsen.
	if err != nil {                           // Wenn das nicht klappt…
		return time.Now().UTC().Format(time.RFC3339) // …nutze "jetzt" (besser als leer).
	} // Ende error-check.
	return parsed.UTC().Format(time.RFC3339) // Normalisiert als RFC3339 String (UTC).
} // Ende pickEntryTime.

func idExists(entries []Entry, id string) bool { // Prüft, ob ID schon in entries.json existiert.
	for _, entry := range entries { // Iteriert linear über alle Entries.
		if entry.ID == id { // Match?
			return true // Existiert schon.
		} // Ende match-check.
	} // Ende loop.
	return false // Nicht gefunden.
} // Ende idExists.

func parsePubDate(value string) (time.Time, error) { // Parst PubDate aus RSS/HTTP-Feeds.
	value = strings.TrimSpace(value) // Whitespace entfernen.
	if value == "" {                 // Wenn leer…
		return time.Time{}, fmt.Errorf("empty pubDate") // …Fehler, damit Caller fallbacken kann.
	} // Ende empty-check.
	if parsed, err := time.Parse(time.RFC1123Z, value); err == nil { // Erst RFC1123Z versuchen (mit Offset).
		return parsed, nil // Erfolg.
	} // Ende first try.
	return time.Parse(time.RFC1123, value) // Sonst RFC1123 ohne explizites Offset versuchen.
} // Ende parsePubDate.

func hashString(value string) string { // Macht aus beliebigem Text einen stabilen Hex-Hash.
	value = strings.TrimSpace(value) // Normalisieren: verhindert Hash-Varianten durch Whitespace.
	if value == "" {                 // Falls wirklich leer…
		return fmt.Sprintf("hash-%d", time.Now().UnixNano()) // …fallback, damit nie leer zurückkommt.
	} // Ende empty-check.
	sum := md5.Sum([]byte(value)) // MD5 Hash (nicht für Security, nur für IDs/Keys ok).
	return fmt.Sprintf("%x", sum) // Hex-String zurückgeben.
} // Ende hashString.

func readJSON(path string, target any) { // Liest JSON-Datei in target, aber "silent fail".
	data, err := os.ReadFile(path) // Datei lesen.
	if err != nil {                // Wenn Datei fehlt/kein Zugriff…
		return // …einfach nichts tun (Defaults bleiben).
	} // Ende error-check.
	_ = json.Unmarshal(data, target) // JSON parsen; Fehler wird ignoriert (bewusst: robust, aber still).
} // Ende readJSON.

func writeJSON(path string, value any) { // Schreibt JSON-Datei, bei Fehlern hartes Exit.
	file, err := os.Create(path) // Datei erstellen/überschreiben.
	if err != nil {              // Wenn das fehlschlägt…
		fmt.Fprintln(os.Stderr, err) // …Fehler ausgeben.
		os.Exit(1)                   // …Programm beenden (weil Persistenz kritisch ist).
	} // Ende error-check.
	defer file.Close() // Datei sicher schließen.

	enc := json.NewEncoder(file)              // JSON Encoder auf Datei.
	enc.SetIndent("", "  ")                   // Pretty JSON für bessere Diffbarkeit/Lesbarkeit.
	enc.SetEscapeHTML(false)                  // Verhindert z.B. "<" zu "\u003c" (hilfreich für Content/Links).
	if err := enc.Encode(value); err != nil { // JSON schreiben.
		fmt.Fprintln(os.Stderr, err) // Fehler ausgeben.
		os.Exit(1)                   // Harte Beendigung (konsistenter State ist wichtig).
	} // Ende encode error-check.
} // Ende writeJSON.
