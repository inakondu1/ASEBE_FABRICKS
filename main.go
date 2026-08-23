package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func renderTemplate(w http.ResponseWriter, filename string, data interface{}) {
	tmpl, err := template.ParseFiles(filename)
	if err != nil {
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, "Page error: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

// =========================
// HOME
// =========================

func homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	renderTemplate(w, "templates/home.html", nil)
}

// =========================
// SHOP
// =========================

func shopHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`
		SELECT id, name, description, price, quantity, image
		FROM products
		ORDER BY id DESC
	`)
	if err != nil {
		http.Error(w, "Could not load fabrics: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var products []Product

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
			http.Error(w, "Could not read fabrics: "+err.Error(), http.StatusInternalServerError)
			return
		}

		products = append(products, product)
	}

	if err := rows.Err(); err != nil {
		http.Error(w, "Could not read fabrics: "+err.Error(), http.StatusInternalServerError)
		return
	}

	renderTemplate(w, "templates/shop.html", products)
}

// =========================
// CART
// =========================

func cartHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "templates/cart.html", nil)
}

// =========================
// PAYMENT
// =========================

func paymentHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "templates/payment.html", nil)
}

// =========================
// CHECKOUT
// =========================

func checkoutHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "templates/checkout.html", nil)
}

// =========================
// ADMIN
// =========================

func adminHandler(w http.ResponseWriter, r *http.Request) {

	rows, err := db.Query(`
		SELECT id, name, description, price, quantity, image
		FROM products
		ORDER BY id DESC
	`)

	if err != nil {
		http.Error(
			w,
			"Could not load fabrics: "+err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	defer rows.Close()

	var products []Product

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
			http.Error(
				w,
				"Could not read fabrics: "+err.Error(),
				http.StatusInternalServerError,
			)
			return
		}

		products = append(products, product)
	}

	if err := rows.Err(); err != nil {
		http.Error(
			w,
			"Could not read fabrics: "+err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	tmpl, err := template.ParseFiles(
		"templates/admin.html",
	)

	if err != nil {
		http.Error(
			w,
			"Template error: "+err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	err = tmpl.Execute(w, products)

	if err != nil {
		http.Error(
			w,
			"Page error: "+err.Error(),
			http.StatusInternalServerError,
		)
		return
	}
}
// =========================
// ADD FABRIC
// =========================

func addFabricHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method == http.MethodGet {
		renderTemplate(w, "templates/add_fabric.html", nil)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, "Could not process form", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	description := r.FormValue("description")

	if name == "" {
		http.Error(w, "Fabric name is required", http.StatusBadRequest)
		return
	}

	price, err := strconv.ParseFloat(r.FormValue("price"), 64)
	if err != nil || price < 0 {
		http.Error(w, "Invalid price", http.StatusBadRequest)
		return
	}

	quantity, err := strconv.Atoi(r.FormValue("quantity"))
	if err != nil || quantity < 0 {
		http.Error(w, "Invalid quantity", http.StatusBadRequest)
		return
	}

	imagePath := ""

	file, header, err := r.FormFile("image")

	if err == nil {
		defer file.Close()

		err = os.MkdirAll("uploads/fabrics", 0755)
		if err != nil {
			http.Error(w, "Could not create upload folder", http.StatusInternalServerError)
			return
		}

		extension := filepath.Ext(header.Filename)

		filename := strconv.FormatInt(time.Now().UnixNano(), 10) + extension

		savePath := filepath.Join("uploads/fabrics", filename)

		destination, err := os.Create(savePath)
		if err != nil {
			http.Error(w, "Could not save picture", http.StatusInternalServerError)
			return
		}
		defer destination.Close()

		_, err = destination.ReadFrom(file)
		if err != nil {
			http.Error(w, "Could not save picture", http.StatusInternalServerError)
			return
		}

		imagePath = "/uploads/fabrics/" + filename
	}

	_, err = db.Exec(`
		INSERT INTO products
		(name, description, price, quantity, image)
		VALUES (?, ?, ?, ?, ?)
	`,
		name,
		description,
		price,
		quantity,
		imagePath,
	)

	if err != nil {
		http.Error(w, "Could not save fabric: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// =========================
// DELETE FABRIC
// =========================

func deleteFabricHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.FormValue("id")

	if id == "" {
		http.Error(w, "Fabric ID is required", http.StatusBadRequest)
		return
	}

	_, err := db.Exec(
		"DELETE FROM products WHERE id = ?",
		id,
	)

	if err != nil {
		http.Error(w, "Could not delete fabric: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// =========================
// MAIN
// =========================

func main() {

	// Connect to database
	initDatabase()

	// Static files
	http.Handle(
		"/static/",
		http.StripPrefix(
			"/static/",
			http.FileServer(http.Dir("static")),
		),
	)

	// Uploaded fabric images
	http.Handle(
		"/uploads/",
		http.StripPrefix(
			"/uploads/",
			http.FileServer(http.Dir("uploads")),
		),
	)

	// Website pages
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/shop", shopHandler)
	http.HandleFunc("/cart", cartHandler)
	http.HandleFunc("/payment", paymentHandler)
	http.HandleFunc("/checkout", checkoutHandler)

	// Admin
	http.HandleFunc("/admin", adminHandler)
	http.HandleFunc("/admin/add-fabric", addFabricHandler)
	http.HandleFunc("/admin/delete-fabric", deleteFabricHandler)

	log.Println("======================================")
	log.Println("🔥 ASEBE FABRICS")
	log.Println("🚀 Server running on http://localhost:8080")
	log.Println("======================================")

	err := http.ListenAndServe(":8080", nil)

	if err != nil {
		log.Fatal(err)
	}
}
