package main

import (
	"html/template"
	"net/http"
	"strings"
)

type AIPageData struct {
	Question string
	Response string
	Products []Product
}

var aiStopWords = map[string]bool{
	"i":         true,
	"need":      true,
	"want":      true,
	"looking":   true,
	"for":       true,
	"do":        true,
	"you":       true,
	"have":      true,
	"any":       true,
	"the":       true,
	"is":        true,
	"are":       true,
	"there":     true,
	"some":      true,
	"please":    true,
	"can":       true,
	"show":      true,
	"me":        true,
	"give":      true,
	"tell":      true,
	"about":     true,
	"what":      true,
	"which":     true,
	"how":       true,
	"much":      true,
	"does":      true,
	"this":      true,
	"that":      true,
	"fabric":    true,
	"fabrics":   true,
	"available": true,
}

var aiCategories = map[string][]string{
	"lace":    {"lace", "tulle", "guipure", "cord"},
	"ankara":  {"ankara", "wax", "atampa"},
	"sequins": {"sequin", "sequence"},
	"brocade": {"brocade", "jacquard"},
	"velvet":  {"velvet"},
	"kente":   {"kente"},
	"organza": {"organza"},
	"beaded":  {"beaded", "bead"},
}

func aiHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/ai.html")
	if err != nil {
		http.Error(w, "Unable to load AI assistant", http.StatusInternalServerError)
		return
	}

	if r.Method == http.MethodGet {
		tmpl.Execute(w, AIPageData{})
		return
	}

	if r.Method == http.MethodPost {
		question := strings.TrimSpace(r.FormValue("question"))

		if question == "" {
			tmpl.Execute(w, AIPageData{
				Response: "Please enter a question so I can help you.",
			})
			return
		}

		words := strings.Fields(strings.ToLower(question))
		var searchWords []string

		for _, word := range words {
			word = strings.Trim(word, ".,!?;:'\"()[]{}")

			if len(word) < 3 {
				continue
			}

			if aiStopWords[word] {
				continue
			}

			if categoryWords, ok := aiCategories[word]; ok {
				searchWords = append(searchWords, categoryWords...)
				continue
			}

			searchWords = append(searchWords, word)
		}

		var products []Product

		for _, word := range searchWords {
			searchTerm := "%" + word + "%"

			rows, err := db.Query(`
				SELECT name, description, price, quantity, image
				FROM products
				WHERE LOWER(name) LIKE ?
				   OR LOWER(description) LIKE ?
			`, searchTerm, searchTerm)

			if err != nil {
				http.Error(w, "Unable to search fabrics.", http.StatusInternalServerError)
				return
			}

			for rows.Next() {
				var product Product

				err := rows.Scan(
					&product.Name,
					&product.Description,
					&product.Price,
					&product.Quantity,
					&product.Image,
				)

				if err != nil {
					rows.Close()
					http.Error(w, "Unable to read fabric information.", http.StatusInternalServerError)
					return
				}

				found := false

				for _, existing := range products {
					if existing.Name == product.Name {
						found = true
						break
					}
				}

				if !found {
					products = append(products, product)
				}
			}

			rows.Close()
		}

		response := ""

		if len(products) > 0 {
			response = "I found these fabrics that match your request."
		} else {
			response = "I couldn't find that fabric in our current collection. You can try another fabric name or ask me about the fabrics currently available."
		}

		data := AIPageData{
			Question: question,
			Response: response,
			Products: products,
		}

		tmpl.Execute(w, data)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}
