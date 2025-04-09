package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"os/exec"
)

// Define structures for XML parsing
type TagInfo struct {
	XMLName xml.Name `xml:"taginfo"`
	Tables  []Table  `xml:"table"`
}

type Table struct {
	Name string `xml:"name,attr"`
	G0   string `xml:"g0,attr"`
	Tags []Tag  `xml:"tag"`
}

type Tag struct {
	Name     string `xml:"name,attr"`
	Type     string `xml:"type,attr"`
	Writable string `xml:"writable,attr"`
	Descs    []Desc `xml:"desc"`
}

type Desc struct {
	Lang string `xml:"lang,attr"`
	Text string `xml:",chardata"`
}

func tagsHandler(w http.ResponseWriter, r *http.Request) {
	// Set headers for JSON response
	w.Header().Set("Content-Type", "application/json")

	// Initialize JSON encoder
	enc := json.NewEncoder(w)
	w.Write([]byte(`{"tags":[`)) // Start the JSON array

	// Create a cancellable context from the HTTP request
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel() // Ensure cancellation when the handler exits

	// Execute exiftool command
	cmd := exec.CommandContext(ctx, "exiftool", "-listx")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get stdout pipe: %v", err), http.StatusInternalServerError)
		return
	}

	if err := cmd.Start(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to start exiftool: %v", err), http.StatusInternalServerError)
		return
	}

	// Ensure cmd.Wait() is called after the response is completed
	defer func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("Error waiting for exiftool: %v", err)
		}
	}()

	// Create XML decoder
	decoder := xml.NewDecoder(stdout)
	first := true

	// Parse XML tokens and stream JSON response
	for {
		select {
		case <-ctx.Done():
			// Client disconnected; terminate the exiftool process
			cmd.Process.Kill()
			return
		default:
			tok, err := decoder.Token()
			if err != nil {
				// End of XML or error while decoding
				if err.Error() == "EOF" {
					// End of XML, break the loop and finish the response
					w.Write([]byte(`]}`)) // End the JSON array and object
				} else {
					// An error occurred during XML decoding
					http.Error(w, fmt.Sprintf("Error parsing XML: %v", err), http.StatusInternalServerError)
				}

				return
			}

			switch se := tok.(type) {
			case xml.StartElement:
				if se.Name.Local == "table" {
					var tbl Table
					if err := decoder.DecodeElement(&tbl, &se); err != nil {
						log.Printf("Error decoding table: %v", err)
						continue
					}
					for _, t := range tbl.Tags {
						if !first {
							w.Write([]byte(","))
						}
						first = false
						tag := map[string]interface{}{
							"writable":    t.Writable == "true",
							"path":        fmt.Sprintf("%s:%s", tbl.Name, t.Name),
							"group":       fmt.Sprintf("%s::%s", tbl.G0, tbl.Name),
							"description": map[string]string{},
							"type":        t.Type,
						}
						for _, d := range t.Descs {
							tag["description"].(map[string]string)[d.Lang] = d.Text
						}
						if err := enc.Encode(tag); err != nil {
							log.Printf("Error encoding JSON: %v", err)
						}
					}
				}
			}
		}
	}

	// End of XML and response handling
	// The deferred function will handle cmd.Wait()
}

func main() {
	http.HandleFunc("/tags", tagsHandler)
	port := ":8080"
	fmt.Printf("Server starting on http://localhost%s...\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
