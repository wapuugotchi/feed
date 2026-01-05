package main // Paket "main": Einstiegspunkt für das ausführbare Programm (Binary), hier liegt die main()-Funktion.

import ( // Import-Block: Standardlib + internes cmd-Paket.
	"fmt"    // Ausgabe auf Stdout/Stderr und formatierte Fehlerausgabe.
	"os"     // Zugriff auf Args, Stdin/Stdout/Stderr, Exit-Codes.
	"wapuugotchi/feed/app/cmd" // Internes cmd-Paket: enthält RunFeedUpdate() und AI-Wrapper für CLI.
)

func main() {
	if err := cmd.RunFeedUpdate(); err != nil { // Standardpfad: Feed aktualisieren und feed.xml schreiben.
		fmt.Fprintln(os.Stderr, err) // Fehler auf stderr ausgeben (CLI-Konvention).
		os.Exit(1) // Exit-Code 1 für generischen Fehler.
	}
}
