package main

import (
	"sort"
	"strconv"
	"strings"
)

func parseMoneyToken(token string) float64 {
	token = strings.TrimSpace(strings.ToLower(token))
	token = strings.Trim(token, ".,!?;:()[]{}₦")

	multiplier := 1.0

	if strings.HasSuffix(token, "k") {
		multiplier = 1000
		token = strings.TrimSuffix(token, "k")
	}

	token = strings.ReplaceAll(token, ",", "")

	value, err := strconv.ParseFloat(token, 64)
	if err != nil {
		return 0
	}

	return value * multiplier
}

func filterByPrice(products []Product, minPrice float64, maxPrice float64) []Product {
	var results []Product

	for _, product := range products {
		if product.Price < minPrice {
			continue
		}

		if maxPrice > 0 && product.Price > maxPrice {
			continue
		}

		results = append(results, product)
	}

	return results
}

func extractPriceRange(question string) (float64, float64) {
	words := strings.Fields(strings.ToLower(question))

	for i, word := range words {
		word = strings.Trim(word, ".,!?;:'\"()[]{}₦")

		if (word == "between" || word == "betwen") && i+1 < len(words) {
			min := parseMoneyToken(words[i+1])

			if min <= 0 {
				continue
			}

			if i+3 < len(words) {
				andWord := strings.Trim(words[i+2], ".,!?;:'\"()[]{}₦")
				max := parseMoneyToken(words[i+3])

				if andWord == "and" && max > 0 {
					return min, max
				}
			}

			return min, 0
		}

		if word == "from" && i+1 < len(words) {
			min := parseMoneyToken(words[i+1])

			if min <= 0 {
				continue
			}

			if i+3 < len(words) {
				toWord := strings.Trim(words[i+2], ".,!?;:'\"()[]{}₦")
				max := parseMoneyToken(words[i+3])

				if toWord == "to" && max > 0 {
					return min, max
				}
			}

			return min, 0
		}

		if (word == "above" || word == "over") && i+1 < len(words) {
			min := parseMoneyToken(words[i+1])

			if min > 0 {
				return min, 0
			}
		}

		if (word == "under" || word == "below") && i+1 < len(words) {
			max := parseMoneyToken(words[i+1])

			if max > 0 {
				return 0, max
			}
		}
	}

	return 0, 0
}

func sortByPrice(products []Product, highest bool) []Product {
	sort.Slice(products, func(i, j int) bool {
		if highest {
			return products[i].Price > products[j].Price
		}

		return products[i].Price < products[j].Price
	})

	return products
}
