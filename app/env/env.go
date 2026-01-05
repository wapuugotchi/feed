package env // Paket "env": kapselt Logik zum Lesen von Environment-Variablen und Laden einer .env Datei.

import ( // Import-Block: Standardbibliothek für OS-, Pfad- und String-Operationen.
	"os"           // Zugriff auf Environment (LookupEnv/Setenv), Dateisystem (ReadFile/Stat), CWD (Getwd).
	"path/filepath" // OS-sichere Pfadoperationen (Join, Dir).
	"strings"      // TrimSpace, Split, HasPrefix: Parsen und Normalisieren von Strings.
)

func ReadEnv(keys ...string) string { // Liest die erste gesetzte, nicht-leere Env-Variable aus einer Liste von Keys.
	for _, key := range keys { // Iteriert über alle übergebenen Schlüssel in der angegebenen Reihenfolge.
		if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" { // Prüft: existiert UND nicht nur Whitespace?
			return strings.TrimSpace(val) // Gibt den getrimmten Wert des ersten passenden Keys zurück.
		}
	}
	return "" // Wenn keiner der Keys gesetzt ist oder nur leer/Whitespace: leerer String.
}

func LoadDotEnv() error { // Lädt eine .env Datei und setzt Variablen ins Prozess-ENV (nur wenn noch nicht gesetzt).
	root := FindRepoRoot() // Ermittelt das Repo-Root (anhand von go.mod), damit .env immer dort gesucht wird.
	envPath := filepath.Join(root, ".env") // Baut den vollständigen Pfad zur .env Datei.

	data, err := os.ReadFile(envPath) // Liest den kompletten Inhalt der .env Datei.
	if err != nil { // Wenn Lesen fehlschlägt…
		if os.IsNotExist(err) { // …und die Datei existiert nicht…
			return nil // …ist das kein Fehler: .env ist optional.
		}
		return err // Anderer Fehler (z.B. Permission): weiterreichen.
	}

	for _, line := range strings.Split(string(data), "\n") { // Iteriert zeilenweise über den Inhalt der .env Datei.
		line = strings.TrimSpace(line) // Entfernt Whitespace am Anfang/Ende der Zeile.
		if line == "" || strings.HasPrefix(line, "#") { // Leere Zeilen oder Kommentare…
			continue // …ignorieren.
		}
		parts := strings.SplitN(line, "=", 2) // Trennt KEY=VALUE, aber nur beim ersten '='.
		if len(parts) != 2 { // Wenn kein gültiges KEY=VALUE Format…
			continue // …ignorieren.
		}
		key := strings.TrimSpace(parts[0]) // Key normalisieren (Whitespace entfernen).
		val := strings.Trim(strings.TrimSpace(parts[1]), `"'`) // Value trimmen und optionale Quotes (' oder ") entfernen.
		if key == "" || val == "" { // Wenn Key oder Value leer sind…
			continue // …ignorieren, um kein kaputtes ENV zu setzen.
		}
		if _, exists := os.LookupEnv(key); !exists { // Nur setzen, wenn die Variable noch nicht existiert…
			if err := os.Setenv(key, val); err != nil { // Setzt die Env-Variable im aktuellen Prozess.
				return err // Wenn Setenv fehlschlägt: Fehler zurückgeben.
			}
		}
	}

	return nil // Erfolgsfall: .env geladen oder nichts zu tun.
}

func FindRepoRoot() string { // Findet das Root-Verzeichnis des Repos anhand einer go.mod Datei.
	wd, err := os.Getwd() // Holt das aktuelle Arbeitsverzeichnis.
	if err != nil { // Wenn das fehlschlägt…
		return "." // …Fallback: aktuelles Verzeichnis.
	}
	dir := wd // Startpunkt der Suche ist das aktuelle Verzeichnis.
	for { // Endlosschleife: läuft Verzeichnisbaum nach oben.
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil { // Prüft, ob in diesem Verzeichnis eine go.mod existiert.
			return dir // Wenn ja: das ifst das Repo-Root.
		}
		parent := filepath.Dir(dir) // Ermittelt das Parent-Verzeichnis.
		if parent == dir { // Wenn Parent == dir, sind wir am Filesystem-Root angekommen.
			return wd // Kein go.mod gefunden → Fallback auf ursprüngliches Working Directory.
		}
		dir = parent // Einen Level nach oben gehen und erneut prüfen.
	}
}
