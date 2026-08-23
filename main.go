package main

import (
	"database/sql"
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

// =========================
// EDIT FABRIC
// =========================

func editFabricHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method == http.MethodGet {

		id := r.URL.Query().Get("id")

		if id == "" {
			http.Error(
				w,
				"Fabric ID is missing",
				http.StatusBadRequest,
			)
			return
		}

		var product Product

		err := db.QueryRow(`
			SELECT id, name, description, price, quantity, image
			FROM products
			WHERE id = ?
		`, id).Scan(
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
				"Fabric not found: "+err.Error(),
				http.StatusNotFound,
			)
			return
		}

		tmpl, err := template.ParseFiles(
			"templates/edit_fabric.html",
		)

		if err != nil {
			http.Error(
				w,
				"Template error: "+err.Error(),
				http.StatusInternalServerError,
			)
			return
		}

		err = tmpl.Execute(w, product)

		if err != nil {
			http.Error(
				w,
				"Could not display edit page: "+err.Error(),
				http.StatusInternalServerError,
			)
		}

		return
	}

	if r.Method != http.MethodPost {

		http.Error(
			w,
			"Method not allowed",
			http.StatusMethodNotAllowed,
		)

		return
	}

	err := r.ParseMultipartForm(10 << 20)

	if err != nil {

		http.Error(
			w,
			"Could not process form",
			http.StatusBadRequest,
		)

		return
	}

	id := r.FormValue("id")
	name := r.FormValue("name")
	description := r.FormValue("description")

	price, err := strconv.ParseFloat(
		r.FormValue("price"),
		64,
	)

	if err != nil {

		http.Error(
			w,
			"Invalid price",
			http.StatusBadRequest,
		)

		return
	}

	quantity, err := strconv.Atoi(
		r.FormValue("quantity"),
	)

	if err != nil {

		http.Error(
			w,
			"Invalid quantity",
			http.StatusBadRequest,
		)

		return
	}

	// Keep the existing image
	var oldImage string

	err = db.QueryRow(
		"SELECT image FROM products WHERE id = ?",
		id,
	).Scan(&oldImage)

	if err != nil {

		http.Error(
			w,
			"Fabric not found",
			http.StatusNotFound,
		)

		return
	}

	imagePath := oldImage

	// Check if a new image was uploaded
	file, header, err := r.FormFile("image")

	if err == nil {

		defer file.Close()

		err = os.MkdirAll(
			"uploads/fabrics",
			0755,
		)

		if err != nil {

			http.Error(
				w,
				"Could not create upload folder",
				http.StatusInternalServerError,
			)

			return
		}

		extension := filepath.Ext(
			header.Filename,
		)

		filename := strconv.FormatInt(
			time.Now().UnixNano(),
			10,
		) + extension

		savePath := filepath.Join(
			"uploads/fabrics",
			filename,
		)

		destination, err := os.Create(savePath)

		if err != nil {

			http.Error(
				w,
				"Could not save new picture",
				http.StatusInternalServerError,
			)

			return
		}

		defer destination.Close()

		_, err = destination.ReadFrom(file)

		if err != nil {

			http.Error(
				w,
				"Could not save new picture",
				http.StatusInternalServerError,
			)

			return
		}

		imagePath = "/uploads/fabrics/" + filename
	}

	// Update fabric
	_, err = db.Exec(`
		UPDATE products
		SET name = ?,
		    description = ?,
		    price = ?,
		    quantity = ?,
		    image = ?
		WHERE id = ?
	`,
		name,
		description,
		price,
		quantity,
		imagePath,
		id,
	)

	if err != nil {

		http.Error(
			w,
			"Could not update fabric: "+err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	// Back to admin
	http.Redirect(
		w,
		r,
		"/admin",
		http.StatusSeeOther,
	)
}
func orderHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method == http.MethodGet {
		renderTemplate(w, "templates/order.html", nil)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Could not process order", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	phone := r.FormValue("phone")
	address := r.FormValue("address")
	note := r.FormValue("note")

	log.Println("========== NEW ORDER ==========")
	log.Println("Name:", name)
	log.Println("Phone:", phone)
	log.Println("Address:", address)
	log.Println("Note:", note)
	log.Println("===============================")

	http.Redirect(w, r, "/payment", http.StatusSeeOther)
}

// =========================
// CUSTOMER REGISTRATION
// =========================

// =========================
// CUSTOMER REGISTRATION
// =========================

func registerHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method == http.MethodGet {
		renderTemplate(w, "templates/register.html", nil)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(
			w,
			"Method not allowed",
			http.StatusMethodNotAllowed,
		)
		return
	}

	err := r.ParseForm()

	if err != nil {
		http.Error(
			w,
			"Could not process registration",
			http.StatusBadRequest,
		)
		return
	}

	name := r.FormValue("name")
	phone := r.FormValue("phone")

	if name == "" || phone == "" {

		http.Error(
			w,
			"Full name and phone number are required",
			http.StatusBadRequest,
		)

		return
	}

	// Check if phone number already exists
	var existingID int

	err = db.QueryRow(
		"SELECT id FROM customers WHERE phone = ?",
		phone,
	).Scan(&existingID)

	if err == nil {

		http.Error(
			w,
			"An account with this phone number already exists",
			http.StatusConflict,
		)

		return
	}

	// Create customer
	_, err = db.Exec(`
        INSERT INTO customers
        (full_name, phone)
        VALUES (?, ?)
    `,
		name,
		phone,
	)

	if err != nil {

		http.Error(
			w,
			"Could not create account: "+err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	// Registration successful
	http.Redirect(
		w,
		r,
		"/login",
		http.StatusSeeOther,
	)
}

// =========================
// CUSTOMER LOGIN
// =========================

// =========================
// CUSTOMER LOGIN
// =========================

func loginHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method == http.MethodGet {
		renderTemplate(w, "templates/login.html", nil)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Could not process login", http.StatusBadRequest)
		return
	}

	phone := r.FormValue("phone")

	if phone == "" {
		http.Error(w, "Phone number is required", http.StatusBadRequest)
		return
	}

	var customerID int
	var customerName string

	err = db.QueryRow(`
		SELECT id, full_name
		FROM customers
		WHERE phone = ?
	`, phone).Scan(
		&customerID,
		&customerName,
	)

	if err == sql.ErrNoRows {
		http.Error(
			w,
			"No account found with this phone number. Please register first.",
			http.StatusNotFound,
		)
		return
	}

	if err != nil {
		http.Error(
			w,
			"Could not login: "+err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	log.Println("Customer logged in:", customerName)

	http.Redirect(
		w,
		r,
		"/customer",
		http.StatusSeeOther,
	)
}

// =========================
// CUSTOMER DASHBOARD
// =========================

func customerHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		http.Error(
			w,
			"Method not allowed",
			http.StatusMethodNotAllowed,
		)
		return
	}

	phone := r.URL.Query().Get("phone")

	if phone == "" {
		http.Redirect(
			w,
			r,
			"/login",
			http.StatusSeeOther,
		)
		return
	}

	var customer struct {
		ID    int
		Name  string
		Phone string
	}

	err := db.QueryRow(`
		SELECT id, full_name, phone
		FROM customers
		WHERE phone = ?
	`, phone).Scan(
		&customer.ID,
		&customer.Name,
		&customer.Phone,
	)

	if err == sql.ErrNoRows {
		http.Redirect(
			w,
			r,
			"/login",
			http.StatusSeeOther,
		)
		return
	}

	if err != nil {
		http.Error(
			w,
			"Could not load customer: "+err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	renderTemplate(
		w,
		"templates/customer.html",
		customer,
	)
}

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
	http.HandleFunc("/order", orderHandler)
	http.HandleFunc("/register", registerHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/customer", customerHandler)

	// Admin
	// Admin
	http.HandleFunc("/admin", adminHandler)
	http.HandleFunc("/admin/add-fabric", addFabricHandler)
	http.HandleFunc("/admin/edit-fabric", editFabricHandler)
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
