package ai // Paket "ai": zentrale Schnittstelle für KI-Provider + .env/ENV Handling.

import ( // Import-Block: Standardbibliothek + Env-Helper.
	"fmt"     // Wird für formatierte Fehler (unknown provider) und Prompt-Formatierung (Sprintf) genutzt.
	"strings" // Trim/Contains/Split: robustes Parsen/Normalisieren von Strings und .env Zeilen.

	"wapuugotchi/feed/app/env"
)

func TransformText(pattern, text string) (string, error) { // Öffentliche API: nimmt Prompt-Pattern + Text und liefert transformierten Output.
	prompt := buildPrompt(pattern, text)                          // Baut aus Pattern+Text den finalen Prompt, der an den Provider geht.
	provider := strings.ToLower(strings.TrimSpace(getProvider())) // Liest AI_PROVIDER, normalisiert (trim + lowercase) für stabile Switch-Logik.
	switch provider {                                             // Wählt je nach Provider-Name die Implementierung.
	case "", "huggingface": // Default: leer oder explizit "huggingface" → Hugging Face verwenden.
		return transformWithHuggingFace(prompt) // Delegiert an HF-Implementierung (HTTP Chat Completions).
	default: // Jede andere Eingabe gilt als nicht unterstützt.
		return "", fmt.Errorf("unknown ai provider: %s", provider) // Klarer Fehler: falscher Provider-Wert.
	}
}

func buildPrompt(pattern, text string) string { // Hilfsfunktion: kombiniert Pattern und Text zu einem Prompt.
	pattern = strings.TrimSpace(pattern) // Entfernt Whitespace, damit "leeres" Pattern zuverlässig erkannt wird.
	if pattern == "" {                   // Wenn kein Pattern vorhanden ist…
		return text // …nur den Text zurückgeben (Prompt = Text).
	}
	if strings.Contains(pattern, "%s") { // Wenn Pattern einen Platzhalter enthält…
		return fmt.Sprintf(pattern, text) // …fülle ihn mit text (typisch: "Text:\n\n%s").
	}
	return pattern + text // Sonst: hänge Text direkt ans Pattern (Achtung: ohne Trennzeichen, Pattern muss das ggf. enthalten).
}

func getProvider() string { // Liest den Provider aus ENV oder .env (AI_PROVIDER).
	if val := env.ReadEnv("AI_PROVIDER"); val != "" { // Erst direkt ENV prüfen (schnell, bevorzugt).
		return val // Wenn gesetzt: sofort zurück.
	}
	_ = env.LoadDotEnv()              // Versucht .env zu laden und Env-Variablen zu setzen; Fehler wird hier ignoriert (best-effort).
	return env.ReadEnv("AI_PROVIDER") // Danach ENV erneut prüfen; liefert "" wenn weiterhin nicht gesetzt.
}
