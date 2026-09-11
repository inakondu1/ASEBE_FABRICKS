package main

import (
	"database/sql"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type AIConversation struct {
	Question string
	Response string
	Products []Product
}

type AIPageData struct {
	Question string
	Response string
	Products []Product
	History  []AIConversation
}

var aiConversations = make(map[string][]AIConversation)
var aiConversationMutex sync.Mutex

const aiConversationCookie = "asebe_ai_session"

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

func getAIConversationID(w http.ResponseWriter, r *http.Request) string {

	cookie, err := r.Cookie(aiConversationCookie)

	if err == nil && cookie.Value != "" {
		return cookie.Value
	}

	id := strconv.FormatInt(time.Now().UnixNano(), 36)

	http.SetCookie(w, &http.Cookie{
		Name:     aiConversationCookie,
		Value:    id,
		Path:     "/ai",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	return id
}

func aiHandler(w http.ResponseWriter, r *http.Request) {
	conversationID := getAIConversationID(w, r)

	if r.Method == http.MethodGet && r.URL.Query().Get("new") == "1" {
		aiConversationMutex.Lock()
		delete(aiConversations, conversationID)
		aiConversationMutex.Unlock()

		conversationID = strconv.FormatInt(time.Now().UnixNano(), 36)

		http.SetCookie(w, &http.Cookie{
			Name:     aiConversationCookie,
			Value:    conversationID,
			Path:     "/ai",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	}

	tmpl, err := template.ParseFiles("templates/ai.html")
	if err != nil {
		http.Error(w, "Unable to load AI assistant", http.StatusInternalServerError)
		return
	}

	if r.Method == http.MethodGet {
		aiConversationMutex.Lock()
		history := append([]AIConversation(nil), aiConversations[conversationID]...)
		for i, j := 0, len(history)-1; i < j; i, j = i+1, j-1 {
			history[i], history[j] = history[j], history[i]
		}
		aiConversationMutex.Unlock()

		tmpl.Execute(w, AIPageData{
			History: history,
		})
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

		aiConversationMutex.Lock()
		previousHistory := append([]AIConversation(nil), aiConversations[conversationID]...)
		aiConversationMutex.Unlock()

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

			if _, ok := aiCategories[word]; ok {
				searchWords = append(searchWords, word)
				continue
			}

			searchWords = append(searchWords, word)

		}
		minPrice, maxPrice := extractPriceRange(question)
		var products []Product

		for _, word := range searchWords {
			searchTerm := "%" + word + "%"

			var rows *sql.Rows
			var err error

			if _, isCategory := aiCategories[word]; isCategory {
				rows, err = db.Query(`
                                    SELECT id, name, description, price, quantity, image
                                    FROM products
                                    WHERE LOWER(name) LIKE ?
                            `, searchTerm)
			} else {
				rows, err = db.Query(`
                                    SELECT id, name, description, price, quantity, image
                                    FROM products
                                    WHERE LOWER(name) LIKE ?
                                       OR LOWER(description) LIKE ?
                            `, searchTerm, searchTerm)
			}

			if err != nil {
				http.Error(w, "Unable to search fabrics.", http.StatusInternalServerError)
				return
			}

			for rows.Next() {
				var product Product

				err := rows.Scan(
					&product.ID,
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
		lowerQuestion := strings.ToLower(question)

		isCheapestQuestion :=
			strings.Contains(lowerQuestion, "cheaper") ||
				strings.Contains(lowerQuestion, "cheapest") ||
				strings.Contains(lowerQuestion, "least price") ||
				strings.Contains(lowerQuestion, "lowest price") ||
				strings.Contains(lowerQuestion, "lowest priced") ||
				strings.Contains(lowerQuestion, "least expensive")

		isMostExpensiveQuestion :=
			strings.Contains(lowerQuestion, "most expensive") ||
				strings.Contains(lowerQuestion, "highest price") ||
				strings.Contains(lowerQuestion, "highest priced") ||
				strings.Contains(lowerQuestion, "most costly")

		if isCheapestQuestion {
			if len(products) == 0 && len(previousHistory) > 0 {
				lastConversation := previousHistory[len(previousHistory)-1]

				if len(lastConversation.Products) > 0 {
					products = append([]Product(nil), lastConversation.Products...)
				}
			}

			if len(products) > 0 {
				products = sortByPrice(products, false)
				cheapest := products[0]
				products = []Product{cheapest}

				response = "The cheapest option is " + cheapest.Name + " at ₦" + strconv.FormatFloat(cheapest.Price, 'f', 2, 64) + "."
			}
		}

		if isMostExpensiveQuestion {
			if len(products) == 0 && len(previousHistory) > 0 {
				lastConversation := previousHistory[len(previousHistory)-1]

				if len(lastConversation.Products) > 0 {
					products = append([]Product(nil), lastConversation.Products...)
				}
			}

			if len(products) > 0 {
				products = sortByPrice(products, true)
				mostExpensive := products[0]
				products = []Product{mostExpensive}

				response = "The most expensive option is " + mostExpensive.Name + " at ₦" + strconv.FormatFloat(mostExpensive.Price, 'f', 2, 64) + "."
			}
		}

		products = filterByPrice(products, minPrice, maxPrice)

		if response == "" && len(products) > 0 {
			response = "I found these fabrics that match your request."
		} else if response == "" {
			response = "I couldn't find that fabric in our current collection. You can try another fabric name or ask me about the fabrics currently available."
		}

		conversation := AIConversation{
			Question: question,
			Response: response,
			Products: products,
		}

		aiConversationMutex.Lock()
		aiConversations[conversationID] = append(aiConversations[conversationID], conversation)
		history := append([]AIConversation(nil), aiConversations[conversationID]...)
		for i, j := 0, len(history)-1; i < j; i, j = i+1, j-1 {
			history[i], history[j] = history[j], history[i]
		}
		aiConversationMutex.Unlock()

		data := AIPageData{
			Question: question,
			Response: response,
			Products: products,
			History:  history,
		}

		tmpl.Execute(w, data)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}
