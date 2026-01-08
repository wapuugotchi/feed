package cmd

import (
	"fmt"
	"os"
	"encoding/xml"
	"io"
)

func RunInfo()  {
	fmt.Printf("Wapuugotchi Feed Generator\n")
	feedFile := "feed.xml"
	file, err := os.Open(feedFile)
	if err != nil {
		fmt.Printf("Error opening feed file: %v\n", err)
		return
	}
	defer file.Close()

	decoder := xml.NewDecoder(file)
	itemCount := 0

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Printf("Error parsing XML: %v\n", err)
			return
		}
		
		if se, ok := token.(xml.StartElement); ok && se.Name.Local == "item" {
			itemCount++
		// Skip to end of item element to get title
		var item struct {
			Title string `xml:"title"`
		}
		err := decoder.DecodeElement(&item, &se)
		if err != nil {
			fmt.Printf("Error decoding item: %v\n", err)
			continue
		}
		fmt.Printf("%d) Title: %s\n", itemCount, item.Title)
		}
	}

	fmt.Printf("Total items: %d\n", itemCount)
}