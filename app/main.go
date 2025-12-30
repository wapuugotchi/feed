package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"wapuugotchi/feed/app/cmd"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "ai" {
		runAi(os.Args[2:])
		return
	}

	if err := cmd.RunFeedUpdate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runAi(args []string) {
	flags := flag.NewFlagSet("ai", flag.ExitOnError)
	text := flags.String("text", "", "Text to translate")
	_ = flags.Parse(args)

	input := strings.TrimSpace(*text)
	if input == "" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		input = strings.TrimSpace(string(data))
	}

	if input == "" {
		fmt.Fprintln(os.Stderr, "missing --text or stdin input")
		os.Exit(2)
	}

	translated, err := cmd.TransformTextByAi(input)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println(translated)
}
