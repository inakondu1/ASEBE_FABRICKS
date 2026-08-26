package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// =========================
// DATE FORMATTER
// =========================

func formatNigeriaDate(value string) string {

	if value == "" {
		return ""
	}

	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
	}

	for _, layout := range layouts {

		parsed, err := time.Parse(layout, value)

		if err == nil {
			return parsed.In(time.FixedZone("WAT", 60*60)).
				Format("02 January 2006, 03:04 PM")
		}
	}

	return value
}

// =========================
// CUSTOMER SESSION HELPERS
// =========================

func setCustomerSession(w http.ResponseWriter, customerID int) {

	http.SetCookie(w, &http.Cookie{
		Name:     "asebe_customer",
		Value:    strconv.Itoa(customerID),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func getCustomerSession(r *http.Request) (int, bool) {

	cookie, err := r.Cookie("asebe_customer")

	if err != nil {
		return 0, false
	}

	customerID, err := strconv.Atoi(cookie.Value)

	if err != nil || customerID <= 0 {
		return 0, false
	}

	return customerID, true
}

func renderTemplate(w http.ResponseWriter, filename string, data interface{}) {
	tmpl, err := template.New(filepath.Base(filename)).Funcs(template.FuncMap{
		"subtract": func(a, b float64) float64 {
			return a - b
		},
	}).ParseFiles(filename)
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

	if r.Method != http.MethodGet {
		http.Error(
			w,
			"Method not allowed",
			http.StatusMethodNotAllowed,
		)
		return
	}

	orderID := r.URL.Query().Get("order")

	if orderID == "" {
		http.Error(
			w,
			"Order number is required",
			http.StatusBadRequest,
		)
		return
	}

	// Load the complete order breakdown.
	var total float64
	var previousBalance float64
	var previousBalanceOrderID int64
	var amountPaid float64

	err := db.QueryRow(`
                SELECT
                        total_amount,
                        COALESCE(previous_balance, 0),
                        COALESCE(previous_balance_order_id, 0),
                        amount_paid
                FROM orders
                WHERE id = ?
        `, orderID).Scan(
		&total,
		&previousBalance,
		&previousBalanceOrderID,
		&amountPaid,
	)

	if err == sql.ErrNoRows {
		http.Error(
			w,
			"Order not found",
			http.StatusNotFound,
		)
		return
	}

	if err != nil {
		http.Error(
			w,
			"Could not load order: "+err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	// Calculate only the fabrics currently inside this order.
	var currentOrderTotal float64

	err = db.QueryRow(`
                SELECT COALESCE(SUM(subtotal), 0)
                FROM order_items
                WHERE order_id = ?
        `, orderID).Scan(&currentOrderTotal)

	if err != nil {
		http.Error(
			w,
			"Could not calculate order items: "+err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	type PaymentPage struct {
		OrderID                string
		CurrentOrderTotal      float64
		PreviousBalance        float64
		PreviousBalanceOrderID int64
		Total                  float64
		AmountPaid             float64
		Balance                float64
	}

	data := PaymentPage{
		OrderID:                orderID,
		CurrentOrderTotal:      currentOrderTotal,
		PreviousBalance:        previousBalance,
		PreviousBalanceOrderID: previousBalanceOrderID,
		Total:                  total,
		AmountPaid:             amountPaid,
		Balance:                total - amountPaid,
	}

	renderTemplate(
		w,
		"templates/payment.html",
		data,
	)
}

// =========================
// CHECKOUT
// =========================

func checkoutHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method == http.MethodGet {

		orderID := r.URL.Query().Get("order")

		if orderID == "" {
			http.Error(
				w,
				"Order number is required",
				http.StatusBadRequest,
			)
			return
		}

		renderTemplate(
			w,
			"templates/checkout.html",
			orderID,
		)

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
			"Could not process payment",
			http.StatusBadRequest,
		)
		return
	}

	orderID := r.FormValue("order")
	amountPaidText := r.FormValue("amount_paid")

	if orderID == "" {
		http.Error(
			w,
			"Order number is required",
			http.StatusBadRequest,
		)
		return
	}

	amountPaid, err := strconv.ParseFloat(
		amountPaidText,
		64,
	)

	if err != nil || amountPaid <= 0 {
		http.Error(
			w,
			"Invalid payment amount",
			http.StatusBadRequest,
		)
		return
	}

	// Load the order's total and previous payments.
	var total float64
	var currentAmountPaid float64

	err = db.QueryRow(`
                SELECT total_amount, amount_paid
                FROM orders
                WHERE id = ?
        `, orderID).Scan(
		&total,
		&currentAmountPaid,
	)

	if err == sql.ErrNoRows {
		http.Error(
			w,
			"Order not found",
			http.StatusNotFound,
		)
		return
	}

	if err != nil {
		http.Error(
			w,
			"Could not load order: "+err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	// The amount entered is the NEW payment.
	// Add it to all previous payments.
	newTotalPaid := currentAmountPaid + amountPaid

	if newTotalPaid > total {
		remaining := total - currentAmountPaid

		http.Error(
			w,
			fmt.Sprintf(
				"Payment is greater than the remaining balance of ₦%.2f",
				remaining,
			),
			http.StatusBadRequest,
		)
		return
	}

	paymentStatus := "UNPAID"

	if newTotalPaid > 0 && newTotalPaid < total {
		paymentStatus = "PART PAYMENT"
	}

	if newTotalPaid == total {
		paymentStatus = "PAID"
	}

	_, err = db.Exec(`
                UPDATE orders
                SET
                        amount_paid = ?,
                        payment_status = ?,
                        last_payment_date = CURRENT_TIMESTAMP
                WHERE id = ?
        `,
		newTotalPaid,
		paymentStatus,
		orderID,
	)

	if err != nil {
		http.Error(
			w,
			"Could not save payment: "+err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	// A fully paid order is no longer the customer's active order.
	// The next purchase must create a new order number.
	if paymentStatus == "PAID" {
		_, err = db.Exec(`
			UPDATE customers
			SET active_order_id = NULL
			WHERE active_order_id = ?
		`, orderID)

		if err != nil {
			http.Error(
				w,
				"Could not close paid order: "+err.Error(),
				http.StatusInternalServerError,
			)
			return
		}
	}

	http.Redirect(
		w,
		r,
		"/receipt?order="+url.QueryEscape(orderID),
		http.StatusSeeOther,
	)
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
	cartJSON := r.FormValue("cart")

	if name == "" || phone == "" {
		http.Error(w, "Name and phone number are required", http.StatusBadRequest)
		return
	}

	if cartJSON == "" {
		http.Error(w, "Your cart is empty", http.StatusBadRequest)
		return
	}

	var cart []struct {
		ID       int     `json:"id"`
		Name     string  `json:"name"`
		Price    float64 `json:"price"`
		Quantity int     `json:"quantity"`
	}

	err = json.Unmarshal([]byte(cartJSON), &cart)
	if err != nil {
		http.Error(w, "Could not read your cart", http.StatusBadRequest)
		return
	}

	if len(cart) == 0 {
		http.Error(w, "Your cart is empty", http.StatusBadRequest)
		return
	}

	// Find the registered customer.
	var customerID int

	err = db.QueryRow(`
		SELECT id
		FROM customers
		WHERE phone = ?
	`, phone).Scan(&customerID)

	if err == sql.ErrNoRows {

		http.Error(
			w,
			"Please register before placing an order.",
			http.StatusBadRequest,
		)

		return
	}

	if err != nil {
		http.Error(
			w,
			"Could not find customer: "+err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	// Calculate order total.
	var total float64

	for _, item := range cart {

		if item.Quantity <= 0 || item.Price < 0 {
			http.Error(
				w,
				"Invalid item in cart",
				http.StatusBadRequest,
			)
			return
		}

		total += item.Price * float64(item.Quantity)
	}

	if total <= 0 {
		http.Error(w, "Order total must be greater than zero", http.StatusBadRequest)
		return
	}

	// =========================================================
	// ACTIVE ORDER SYSTEM
	// =========================================================

	// Check whether this customer already has an active order.
	var activeOrderID sql.NullInt64

	err = db.QueryRow(`
        SELECT active_order_id
        FROM customers
        WHERE id = ?
    `, customerID).Scan(&activeOrderID)

	if err != nil {
		http.Error(
			w,
			"Could not check active order: "+err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	var orderID int64
	var previousBalance float64

	// =========================================================
	// REUSE EXISTING ACTIVE ORDER
	// =========================================================

	if activeOrderID.Valid && activeOrderID.Int64 > 0 {

		orderID = activeOrderID.Int64

		err = db.QueryRow(`
            SELECT previous_balance
            FROM orders
            WHERE id = ?
              AND customer_id = ?
        `,
			orderID,
			customerID,
		).Scan(&previousBalance)

		if err == sql.ErrNoRows {

			// The saved active order no longer exists.
			// We will create a new one below.
			activeOrderID.Valid = false
			activeOrderID.Int64 = 0

		} else if err != nil {

			http.Error(
				w,
				"Could not load active order: "+err.Error(),
				http.StatusInternalServerError,
			)
			return
		}
	}

	// =========================================================
	// CREATE NEW ACTIVE ORDER
	// =========================================================

	if !activeOrderID.Valid || activeOrderID.Int64 == 0 {

		var previousBalanceOrderID int64

		err = db.QueryRow(`
            SELECT
                total_amount - amount_paid,
                id
            FROM orders
            WHERE customer_id = ?
              AND amount_paid < total_amount
              AND carried_forward_to_order_id IS NULL
            ORDER BY id ASC
            LIMIT 1
        `,
			customerID,
		).Scan(
			&previousBalance,
			&previousBalanceOrderID,
		)

		if err == sql.ErrNoRows {

			previousBalance = 0
			previousBalanceOrderID = 0

		} else if err != nil {

			http.Error(
				w,
				"Could not check previous balance: "+err.Error(),
				http.StatusInternalServerError,
			)
			return
		}

		totalWithBalance := total

		result, err := db.Exec(`
            INSERT INTO orders
            (
                customer_id,
                total_amount,
                amount_paid,
                payment_status,
                previous_balance,
                previous_balance_order_id
            )
            VALUES (?, ?, 0, 'UNPAID', ?, ?)
        `,
			customerID,
			totalWithBalance,
			previousBalance,
			previousBalanceOrderID,
		)

		if err != nil {

			http.Error(
				w,
				"Could not create order: "+err.Error(),
				http.StatusInternalServerError,
			)
			return
		}

		orderID, err = result.LastInsertId()

		if err != nil {

			http.Error(
				w,
				"Could not get order ID: "+err.Error(),
				http.StatusInternalServerError,
			)
			return
		}

		// Remember this order for this customer.
		_, err = db.Exec(`
            UPDATE customers
            SET active_order_id = ?
            WHERE id = ?
        `,
			orderID,
			customerID,
		)

		if err != nil {

			http.Error(
				w,
				"Could not save active order: "+err.Error(),
				http.StatusInternalServerError,
			)
			return
		}

		// Carry forward any previous unpaid balance.
		if previousBalanceOrderID > 0 {

			_, err = db.Exec(`
                UPDATE orders
                SET carried_forward_to_order_id = ?
                WHERE id = ?
            `,
				orderID,
				previousBalanceOrderID,
			)

			if err != nil {

				http.Error(
					w,
					"Could not link previous balance: "+err.Error(),
					http.StatusInternalServerError,
				)
				return
			}
		}

		log.Println("========== NEW ACTIVE ORDER ==========")
		log.Println("Order ID:", orderID)

	} else {

		// =====================================================
		// UPDATE EXISTING ACTIVE ORDER
		// =====================================================

		// The previous balance belongs to this active order.
		// Include it once together with the current cart total.
		totalWithBalance := previousBalance + total

		_, err = db.Exec(`
            UPDATE orders
            SET
                total_amount = ?,
                payment_status = CASE
                    WHEN amount_paid >= ? THEN 'PAID'
                    WHEN amount_paid > 0 THEN 'PARTIAL'
                    ELSE 'UNPAID'
                END
            WHERE id = ?
              AND customer_id = ?
        `,
			totalWithBalance,
			totalWithBalance,
			orderID,
			customerID,
		)

		if err != nil {

			http.Error(
				w,
				"Could not update active order: "+err.Error(),
				http.StatusInternalServerError,
			)
			return
		}

		log.Println("========== REUSING ACTIVE ORDER ==========")
		log.Println("Order ID:", orderID)
	}

	// =========================================================
	// REFRESH ORDER ITEMS
	// =========================================================

	// Remove the previous cart contents from this active order.
	// This prevents duplicate quantities when the customer
	// submits the same cart again.
	_, err = db.Exec(`
        DELETE FROM order_items
        WHERE order_id = ?
    `, orderID)

	if err != nil {

		http.Error(
			w,
			"Could not refresh order items: "+err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	// Save every fabric in the order.
	for _, item := range cart {

		subtotal := item.Price * float64(item.Quantity)

		_, err = db.Exec(`
			INSERT INTO order_items
			(order_id, product_id, product_name, price, quantity, subtotal)
			VALUES (?, ?, ?, ?, ?, ?)
		`,
			orderID,
			item.ID,
			item.Name,
			item.Price,
			item.Quantity,
			subtotal,
		)

		if err != nil {
			http.Error(
				w,
				"Could not save order item: "+err.Error(),
				http.StatusInternalServerError,
			)
			return
		}
	}

	log.Println("========== NEW ORDER ==========")
	log.Println("Order ID:", orderID)
	log.Println("Customer:", name)
	log.Println("Phone:", phone)
	log.Println("Address:", address)
	log.Println("Note:", note)
	log.Println("Total:", total)
	log.Println("===============================")

	http.Redirect(
		w,
		r,
		"/payment?order="+strconv.FormatInt(orderID, 10),
		http.StatusSeeOther,
	)
}

// =========================
// ORDER RECEIPT
// =========================

func receiptHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orderID := r.URL.Query().Get("order")

	if orderID == "" {
		http.Error(w, "Order number is required", http.StatusBadRequest)
		return
	}

	type ReceiptItem struct {
		ProductName string
		Price       float64
		Quantity    int
		Subtotal    float64
	}

	type Receipt struct {
		OrderID                int64
		Date                   string
		CustomerName           string
		Phone                  string
		Items                  []ReceiptItem
		Total                  float64
		AmountPaid             float64
		Balance                float64
		PaymentStatus          string
		PreviousBalance        float64
		PreviousBalanceOrderID int64
		LastPayment            string
	}

	var receipt Receipt

	err := db.QueryRow(`
		SELECT
			o.id,
			o.created_at,
			c.full_name,
			c.phone,
			o.total_amount,
			o.amount_paid,
			o.payment_status,
                        o.previous_balance,
                        COALESCE(o.previous_balance_order_id, 0),
			COALESCE(o.last_payment_date, '')
		FROM orders o
		JOIN customers c ON c.id = o.customer_id
		WHERE o.id = ?
	`, orderID).Scan(
		&receipt.OrderID,
		&receipt.Date,
		&receipt.CustomerName,
		&receipt.Phone,
		&receipt.Total,
		&receipt.AmountPaid,
		&receipt.PaymentStatus,
		&receipt.PreviousBalance,
		&receipt.PreviousBalanceOrderID,
		&receipt.LastPayment,
	)

	if err == sql.ErrNoRows {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	if err != nil {
		http.Error(
			w,
			"Could not load receipt: "+err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	receipt.Date = formatNigeriaDate(receipt.Date)
	receipt.LastPayment = formatNigeriaDate(receipt.LastPayment)

	receipt.Balance = receipt.Total - receipt.AmountPaid

	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM order_items
		WHERE order_id = ?
	`, orderID).Scan(new(int))

	if err != nil {
		http.Error(
			w,
			"Could not check order items: "+err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	rows, err := db.Query(`
		SELECT product_name, price, quantity, subtotal
		FROM order_items
		WHERE order_id = ?
		ORDER BY id ASC
	`, orderID)

	if err != nil {
		http.Error(
			w,
			"Could not load order items: "+err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	defer rows.Close()

	for rows.Next() {

		var item ReceiptItem

		err := rows.Scan(
			&item.ProductName,
			&item.Price,
			&item.Quantity,
			&item.Subtotal,
		)

		if err != nil {
			http.Error(
				w,
				"Could not read order items: "+err.Error(),
				http.StatusInternalServerError,
			)
			return
		}

		receipt.Items = append(receipt.Items, item)
	}

	if err := rows.Err(); err != nil {
		http.Error(
			w,
			"Could not read order items: "+err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	renderTemplate(
		w,
		"templates/receipt.html",
		receipt,
	)
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
	setCustomerSession(w, customerID)

	http.Redirect(
		w,
		r,
		"/customer?phone="+url.QueryEscape(phone)+"&new_login=1",
		http.StatusSeeOther,
	)
}

// =========================
// CUSTOMER LOGOUT
// =========================

func logoutHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		http.Error(
			w,
			"Method not allowed",
			http.StatusMethodNotAllowed,
		)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "asebe_customer",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(
		w,
		r,
		"/",
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

// =========================
// CUSTOMER ORDER HISTORY
// =========================

func orderHistoryHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	phone := r.URL.Query().Get("phone")

	if phone == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// ---------------------------------------------------------
	// FIND CUSTOMER
	// ---------------------------------------------------------

	var customerID int

	err := db.QueryRow(`
		SELECT id
		FROM customers
		WHERE phone = ?
	`, phone).Scan(&customerID)

	if err == sql.ErrNoRows {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
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

	// ---------------------------------------------------------
	// DATA STRUCTURES
	// ---------------------------------------------------------

	type OrderHistoryItem struct {
		ID   int64
		Date string

		// Current order information
		CurrentOrderTotal float64

		// Previous debt carried into this order
		PreviousBalance        float64
		PreviousBalanceOrderID int64

		// Complete financial picture
		Total         float64
		AmountPaid    float64
		Balance       float64
		PaymentStatus string

		// Human-friendly date
		DisplayDate string
	}

	type OrderHistoryPage struct {
		Phone  string
		Orders []OrderHistoryItem
	}

	// ---------------------------------------------------------
	// LOAD ORDERS
	// ---------------------------------------------------------

	rows, err := db.Query(`
		SELECT
			o.id,
			o.created_at,
			o.total_amount,
			o.amount_paid,
			o.payment_status,
			COALESCE(o.previous_balance, 0),
			COALESCE(o.previous_balance_order_id, 0)
		FROM orders o
		WHERE o.customer_id = ?
		ORDER BY o.id DESC
	`, customerID)

	if err != nil {
		http.Error(
			w,
			"Could not load order history: "+err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	defer rows.Close()

	var orders []OrderHistoryItem

	for rows.Next() {

		var order OrderHistoryItem

		err := rows.Scan(
			&order.ID,
			&order.Date,
			&order.Total,
			&order.AmountPaid,
			&order.PaymentStatus,
			&order.PreviousBalance,
			&order.PreviousBalanceOrderID,
		)

		if err != nil {
			http.Error(
				w,
				"Could not read order history: "+err.Error(),
				http.StatusInternalServerError,
			)
			return
		}

		// -----------------------------------------------------
		// CURRENT FABRICS TOTAL
		// -----------------------------------------------------

		err = db.QueryRow(`
			SELECT COALESCE(SUM(subtotal), 0)
			FROM order_items
			WHERE order_id = ?
		`, order.ID).Scan(&order.CurrentOrderTotal)

		if err != nil {
			http.Error(
				w,
				"Could not calculate order fabrics: "+err.Error(),
				http.StatusInternalServerError,
			)
			return
		}

		// -----------------------------------------------------
		// REMAINING BALANCE
		// -----------------------------------------------------

		order.Balance = order.Total - order.AmountPaid

		// Prevent tiny floating-point negative values.
		if order.Balance < 0 {
			order.Balance = 0
		}

		// -----------------------------------------------------
		// FORMAT DATE FOR CUSTOMER
		// -----------------------------------------------------

		order.DisplayDate = order.Date

		if parsed, err := time.Parse(time.RFC3339, order.Date); err == nil {
			order.DisplayDate = parsed.In(time.FixedZone("WAT", 60*60)).
				Format("02 January 2006, 3:04 PM")
		}

		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		http.Error(
			w,
			"Could not read order history: "+err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	// ---------------------------------------------------------
	// SEND READY-TO-DISPLAY DATA TO TEMPLATE
	// ---------------------------------------------------------

	data := OrderHistoryPage{
		Phone:  phone,
		Orders: orders,
	}

	renderTemplate(
		w,
		"templates/order-history.html",
		data,
	)
}

// =========================
// CUSTOMER LOGOUT
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
	http.HandleFunc("/receipt", receiptHandler)
	http.HandleFunc("/register", registerHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/logout", logoutHandler)

	http.HandleFunc("/customer", customerHandler)
	http.HandleFunc("/order-history", orderHistoryHandler)

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
