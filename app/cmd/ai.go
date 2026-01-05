package cmd // Paket "cmd": enthält CLI-nahe Logik und Wrapper-Funktionen für Kommandozeilenbefehle.

import "wapuugotchi/feed/app/ai" // Importiert das AI-Paket, das die eigentliche Text-Transformation ausführt.

const defaultPattern = "Text:\n\n%s" // Default-Prompt für die CLI; %s wird durch den übergebenen Text ersetzt.

// TransformTextByAi uses the default prompt for the CLI. // Dokumentationskommentar: beschreibt Zweck der Funktion.
func TransformTextByAi(text string) (string, error) { // Öffentliche Hilfsfunktion: kapselt KI-Aufruf für CLI-Nutzung.
	return ai.TransformText(defaultPattern, text) // Ruft die zentrale KI-Funktion mit Default-Prompt + Text auf und gibt Ergebnis/Fehler direkt zurück.
}
