package ai // Paket "ai": kapselt alles rund um KI-Integration (hier: Hugging Face Chat Completions API).

import ( // Import-Block: Standardbibliothek-Module, die für HTTP, JSON und Timeouts benötigt werden.
	"bytes"         // Baut einen io.Reader aus []byte für den HTTP-Request-Body (bytes.NewReader).
	"context"       // Ermöglicht Timeouts/Cancel für HTTP-Requests (context.WithTimeout).
	"encoding/json" // JSON (Marshal/Unmarshal) für Request/Response an/von der API.
	"fmt"           // Formatierte Fehlermeldungen mit Kontext (fmt.Errorf).
	"io"            // io.ReadAll: liest Response-Body vollständig.
	"net/http"      // HTTP Client/Request/Response für den API-Call.
	"strings"       // TrimSpace, String-Building für Header/Fehlertexte.
	"time"          // Timeout-Dauer (30s) und Zeitsteuerung.

	"wapuugotchi/feed/app/env"
)

const ( // Konstanten: zentrale API-Konfiguration.
	hfEndpoint = "https://router.huggingface.co/v1/chat/completions" // Hugging Face Router Endpoint für Chat Completions.
	hfModel    = "meta-llama/Llama-3.1-8B-Instruct"                  // Modell-ID, das am Endpoint angefragt wird.
)

type chatRequest struct { // Struktur des JSON-Request-Payloads für /v1/chat/completions.
	Model       string        `json:"model"`       // Modellname, den die API verwenden soll.
	Messages    []chatMessage `json:"messages"`    // Chat-Historie als Liste (hier typischerweise nur 1 User-Message).
	Temperature float64       `json:"temperature"` // Sampling/Randomness; niedriger => stabilere, deterministischere Outputs.
}

type chatMessage struct { // Einzelne Chat-Message im Request.
	Role    string `json:"role"`    // Rolle im Chat ("user", "assistant", "system" etc.).
	Content string `json:"content"` // Inhalt der Nachricht (Prompt/Text).
}

type chatResponse struct { // Minimaler Ausschnitt der erwarteten API-Response-Struktur.
	Choices []struct { // "choices" enthält i.d.R. 1..n Antworten.
		Message struct { // Jede Choice hat eine Message.
			Content string `json:"content"` // Der generierte Text der KI.
		} `json:"message"` // Mappt das "message"-Objekt.
	} `json:"choices"` // Mappt das "choices"-Array.
}

func transformWithHuggingFace(prompt string) (string, error) { // High-Level Funktion: Prompt rein, fertiger Text raus.
	raw, err := postChatCompletion(prompt) // Sendet den Prompt an die API und bekommt Raw-JSON-Response zurück.
	if err != nil {                        // Wenn HTTP/Token/Status/Netzwerk fehlschlägt…
		return "", err // …Fehler nach oben durchreichen.
	}

	var resp chatResponse                                      // Zielvariable für das Unmarshal der JSON-Response.
	if err := json.Unmarshal([]byte(raw), &resp); err != nil { // Parse des JSON-Strings in die Struktur.
		return "", err // Wenn Response kein gültiges JSON ist oder Struktur unerwartet: Fehler zurück.
	}
	if len(resp.Choices) == 0 { // Wenn die API keine Antwortoptionen liefert…
		return "", fmt.Errorf("huggingface api returned no choices") // …ist das ein harter Fehler (nichts zum Weiterverarbeiten).
	}
	translated := strings.TrimSpace(resp.Choices[0].Message.Content) // Nimmt die erste Choice und trimmt Whitespace.
	if translated == "" {                                            // Wenn der resultierende Text leer ist…
		return "", fmt.Errorf("huggingface api returned empty translation") // …Fehler: leere Transformation ist i.d.R. nicht brauchbar.
	}
	return translated, nil // Erfolgsfall: normalisierter Output der KI.
}

func postChatCompletion(prompt string) (string, error) { // Low-Level Funktion: baut Request, macht HTTP Call, liefert Raw-Response.
	token, err := loadHuggingFaceToken() // Holt das HF-Token aus ENV oder .env.
	if err != nil {                      // Wenn kein Token vorhanden oder .env Laden fehlschlägt…
		return "", err // …Fehler zurück (ohne Token keine Auth).
	}

	payload := chatRequest{ // Baut das Request-Payload passend zur Hugging Face Chat Completions API.
		Model: hfModel, // Nutzt die oben definierte Modell-Konstante.
		Messages: []chatMessage{ // Chat-Verlauf: hier nur eine User-Message.
			{Role: "user", Content: prompt}, // Übergibt den Prompt als User-Content.
		},
		Temperature: 0.2, // Niedrige Temperatur für konsistente, weniger "kreative" Antworten.
	}

	body, err := json.Marshal(payload) // Serialisiert payload zu JSON Bytes.
	if err != nil {                    // Kann fehlschlagen bei unmarschallbaren Typen (hier unwahrscheinlich).
		return "", err // Fehler zurück.
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Timeout-Kontext: verhindert Hängen bei API/Netzwerk.
	defer cancel()                                                           // Stellt sicher, dass Ressourcen des Contexts freigegeben werden.

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hfEndpoint, bytes.NewReader(body)) // Baut POST-Request mit Timeout.
	if err != nil {                                                                                 // Fehler bei ungültiger URL oder Reader.
		return "", err // Fehler zurück.
	}
	req.Header.Set("Content-Type", "application/json") // API erwartet JSON.
	req.Header.Set("Authorization", "Bearer "+token)   // Auth via Bearer Token (Standard bei HF).

	resp, err := http.DefaultClient.Do(req) // Führt den HTTP Request aus (DefaultClient nutzt u.a. Keep-Alive).
	if err != nil {                         // Netzwerkfehler, TLS, Timeout, DNS, etc.
		return "", err // Fehler zurück.
	}
	defer resp.Body.Close() // Immer schließen, sonst Leaks/Connection nicht zurück in Pool.

	respBody, err := io.ReadAll(resp.Body) // Liest gesamten Body (auch bei Fehlerstatus hilfreich für Debug).
	if err != nil {                        // Wenn ReadAll fehlschlägt (selten, aber möglich)…
		return "", err // …Fehler zurück.
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 { // Nicht-2xx Status als Fehler behandeln.
		return "", fmt.Errorf("huggingface api status: %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
		// Liefert Status + Body-Text (trimmed) zurück, damit man API-Fehler sieht (Quota, Auth, Invalid payload).
	}

	return string(respBody), nil // Erfolgsfall: Raw JSON Response als String zurück.
}

func loadHuggingFaceToken() (string, error) { // Token-Lader: versucht ENV, dann .env laden, dann ENV erneut.
	if token := env.ReadEnv("HUGGINGFACE_TOKEN", "HF_TOKEN"); token != "" { // Prüft zuerst bereits gesetzte Umgebungsvariablen.
		return token, nil // Wenn vorhanden: sofort zurück.
	}

	if err := env.LoadDotEnv(); err != nil { // Versucht .env Datei zu laden (damit ENV Variablen gesetzt werden).
		return "", err // Wenn das Laden scheitert: Fehler (je nach Design evtl. streng).
	}

	if token := env.ReadEnv("HUGGINGFACE_TOKEN", "HF_TOKEN"); token != "" { // Prüft nach dem Laden erneut ENV.
		return token, nil // Wenn jetzt vorhanden: zurück.
	}

	return "", fmt.Errorf("missing Hugging Face token: set HUGGINGFACE_TOKEN or HF_TOKEN")
	// Endgültiger Fehler: ohne Token kann die API nicht aufgerufen werden; klare Handlungsanweisung.
}
