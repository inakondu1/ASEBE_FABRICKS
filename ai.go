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
	Question     string
	Response     string
	Products     []Product
	History      []AIConversation
	CustomerName string
}

var aiConversations = make(map[string][]AIConversation)
var aiConversationMutex sync.Mutex

const aiConversationCookie = "asebe_ai_session"

const asebeAISystemRules = `
You are ASEBE AI, a friendly fabric shopping assistant for ASEBE FABRICS.

Your job is to help customers discover fabrics, understand the ASEBE website,
choose suitable fabrics, check prices, and make better fabric decisions.

Always be friendly and helpful.

Give useful suggestions based on what the customer is looking for.

Use real ASEBE products and prices from the database.
Never invent products or prices.

If a customer asks something unrelated to ASEBE FABRICS,
politely guide the conversation back to ASEBE FABRICS.
`

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

	navData := getNavData(r)
	customerName := navData.CustomerName

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

		lowerQuestion := strings.ToLower(strings.TrimSpace(question))

		isGreeting := strings.HasPrefix(lowerQuestion, "hello") ||
			strings.HasPrefix(lowerQuestion, "hi ") ||
			strings.HasPrefix(lowerQuestion, "hey ") ||
			strings.HasPrefix(lowerQuestion, "good morning") ||
			strings.HasPrefix(lowerQuestion, "good afternoon") ||
			strings.HasPrefix(lowerQuestion, "good evening")

		var response string

		switch lowerQuestion {

		case "hi", "hello", "hey", "good morning", "good afternoon", "good evening":
			if customerName != "" {
				response = "Hello " + customerName + "! Welcome to ASEBE FABRICS. I am here to help you find fabrics, check prices, and choose the right material for your style."
			} else {
				response = "Hello! Welcome to ASEBE FABRICS. I am here to help you find fabrics, check prices, and choose the right material for your style."
			}

		case "thanks", "thank you", "thank you so much":
			response = "You are welcome 😊. I am always happy to help you find the perfect fabric."

		case "bye", "goodbye":
			response = "Thank you for visiting ASEBE FABRICS. I hope to help you again soon."

		case "who are you", "what are you":
			response = "I am ASEBE AI Assistant 🤖, your fabric shopping assistant. I can help you discover fabrics, compare options, and answer questions about our collection."

		case "help", "help me":
			response = "I can help you find fabrics like lace, Ankara, velvet, brocade, sequin and more. You can also ask things like show me lace below ₦50000."

		}

		if response == "" && isGreeting {
			if customerName != "" {
				response = "Hello " + customerName + "! It is lovely to have you here. How can I help you with your fabric search today?"
			} else {
				response = "Hello! It is lovely to have you here. How can I help you with your fabric search today?"
			}
		}

		if response == "" &&
			(strings.Contains(lowerQuestion, "don't know") ||
				strings.Contains(lowerQuestion, "do not know") ||
				strings.Contains(lowerQuestion, "which fabric should") ||
				strings.Contains(lowerQuestion, "what fabric should") ||
				strings.Contains(lowerQuestion, "help me choose") ||
				strings.Contains(lowerQuestion, "help me pick") ||
				strings.Contains(lowerQuestion, "recommend a fabric") ||
				strings.Contains(lowerQuestion, "recommend something")) {
			response = "No problem 😊 I can help you choose. Tell me what the fabric is for, such as a wedding, birthday, office, traditional wear, or everyday use, and I will suggest some beautiful options from our collection."
		}

		if response != "" {

			conversation := AIConversation{
				Question: question,
				Response: response,
			}

			aiConversationMutex.Lock()
			aiConversations[conversationID] = append(aiConversations[conversationID], conversation)
			history := append([]AIConversation(nil), aiConversations[conversationID]...)

			for i, j := 0, len(history)-1; i < j; i, j = i+1, j-1 {
				history[i], history[j] = history[j], history[i]
			}

			aiConversationMutex.Unlock()

			tmpl.Execute(w, AIPageData{
				Question: question,
				Response: response,
				History:  history,
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
			if strings.Contains(lowerQuestion, "wedding") ||
				strings.Contains(lowerQuestion, "bride") ||
				strings.Contains(lowerQuestion, "bridal") {
				response = "That is beautiful, and you are at the right place! 😊 I found some lovely fabric options for your wedding. Explore the fabrics below, and if you are not sure which one to choose, I can help you find the perfect one."
			} else if strings.Contains(lowerQuestion, "party") ||
				strings.Contains(lowerQuestion, "birthday") ||
				strings.Contains(lowerQuestion, "celebration") {
				response = "Awesome! 🎉 You are in the right place. I found some stylish fabric options for your celebration. If you want something unique, admirable, and comfortable, I can help you choose the perfect one."
			} else if strings.Contains(lowerQuestion, "office") ||
				strings.Contains(lowerQuestion, "work") {
				response = "For office wear, these are some elegant options from our collection."
			} else if strings.Contains(lowerQuestion, "casual") ||
				strings.Contains(lowerQuestion, "everyday") {
				response = "For casual or everyday wear, these are some lovely options from our collection."
			} else if strings.Contains(lowerQuestion, "traditional") ||
				strings.Contains(lowerQuestion, "native") {
				response = "For traditional wear, these are some beautiful options from our collection."
			} else {
				response = "Great choice! 😊 I found these beautiful fabrics that match what you are looking for. Take a look below, and if you are not sure which one to choose, I can help you compare them."
			}
		} else if response == "" &&
			(strings.Contains(lowerQuestion, "wedding") ||
				strings.Contains(lowerQuestion, "bride") ||
				strings.Contains(lowerQuestion, "bridal")) {
			response = "For a wedding, I can help you find something elegant and beautiful. Try asking for a fabric type such as lace, guipure, tulle, beaded fabric or velvet, and I can show you what is available."
		} else if response == "" &&
			(strings.Contains(lowerQuestion, "party") ||
				strings.Contains(lowerQuestion, "birthday") ||
				strings.Contains(lowerQuestion, "celebration")) {
			response = "For a party or celebration, I can help you find something stylish and eye-catching. Try asking for Ankara, lace, sequins or another fabric you like."
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
