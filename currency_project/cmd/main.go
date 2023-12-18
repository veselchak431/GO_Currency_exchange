package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Currency struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	ExchangeToRUB float64 `json:"exchange_to_rub"`
	UpdateTime  time.Time `json:"update_time"`
}

type OpenExchangeRatesResponse struct {
	Rates map[string]float64 `json:"rates"`
}

const openExchangeRatesAPIKey = "4f691702c815480e96aef332f9bf7c3b"

func main() {
	db, err := sql.Open("sqlite3", "./currencies.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	createTable(db)

	// Обновляем курсы валют при старте микросервиса
	updateCurrencies(db)

	// Запускаем обновление курсов валют каждую минуту
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				updateCurrencies(db)
			}
		}
	}()

	http.HandleFunc("/", helloHandler)
	http.HandleFunc("/currency", currencyHandler)
	http.HandleFunc("/currency/latest", latestCurrencyHandler)
	http.HandleFunc("/currency/all", allCurrenciesHandler) 

	addr := "127.0.0.1:8080" // IP и порт, на которых будет запущен сервер

	fmt.Printf("Сервер запущен на http://%s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, I am microservice")
}

func createTable(db *sql.DB) {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS currencies (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT,
		exchange_to_rub REAL,
		update_time TIMESTAMP
	);
	`

	_, err := db.Exec(createTableSQL)
	if err != nil {
		log.Fatal(err)
	}
}

func updateCurrencies(db *sql.DB) {
	resp, err := http.Get("https://openexchangerates.org/api/latest.json?app_id=" + openExchangeRatesAPIKey)
	if err != nil {
		log.Println("Ошибка при получении данных от Open Exchange Rates API:", err)
		return
	}
	defer resp.Body.Close()

	var data OpenExchangeRatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		log.Println("Ошибка при декодировании данных:", err)
		return
	}

	rubToUSD, found := data.Rates["RUB"]
	log.Println(rubToUSD)
	if !found {
		log.Println("Курс рубля к доллару не найден в полученных данных")
		return
	}

	insertSQL := `
		INSERT INTO currencies (name, exchange_to_rub, update_time)
		VALUES (?, ?, ?)
	`

	stmt, err := db.Prepare(insertSQL)
	if err != nil {
		log.Println("Ошибка при подготовке запроса:", err)
		return
	}
	defer stmt.Close()

	currentTime := time.Now()

	for currencyName, exchangeRate := range data.Rates {
		exchangeToRUB := 1/(exchangeRate / rubToUSD)

		_, err := stmt.Exec(currencyName, exchangeToRUB, currentTime)
		if err != nil {
			log.Println("Ошибка при выполнении запроса вставки:", err)
			continue
		}
	}
}


func currencyHandler(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "./currencies.db")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer db.Close()

	if r.Method != http.MethodGet {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	queryValues := r.URL.Query()
	currencyName := queryValues.Get("currency")

	query := "SELECT name, exchange_to_rub, update_time FROM currencies WHERE name = ?"

	rows, err := db.Query(query, currencyName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var currencies []Currency

	for rows.Next() {
		var currency Currency
		if err := rows.Scan(&currency.Name, &currency.ExchangeToRUB, &currency.UpdateTime); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		currencies = append(currencies, currency)
	}

	jsonBytes, err := json.MarshalIndent(currencies, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonBytes)
}

func latestCurrencyHandler(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "./currencies.db")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer db.Close()

	if r.Method != http.MethodGet {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	queryValues := r.URL.Query()
	currencyName := queryValues.Get("currency")

	query := "SELECT name, exchange_to_rub, update_time FROM currencies WHERE name = ? ORDER BY update_time DESC LIMIT 1"

	var currency Currency
	err = db.QueryRow(query, currencyName).Scan(&currency.Name, &currency.ExchangeToRUB, &currency.UpdateTime)
	switch {
	case err == sql.ErrNoRows:
		http.Error(w, "Валюта не найдена", http.StatusNotFound)
		return
	case err != nil:
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonBytes, err := json.Marshal(currency)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonBytes)
}

func allCurrenciesHandler(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "./currencies.db")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer db.Close()

	if r.Method != http.MethodGet {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	query := `
	SELECT name, exchange_to_rub, update_time
	FROM currencies c
	WHERE update_time = (
		SELECT MAX(update_time)
		FROM currencies
		WHERE name = c.name
	)
`

	rows, err := db.Query(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var currencies []Currency

	for rows.Next() {
		var currency Currency
		if err := rows.Scan(&currency.Name, &currency.ExchangeToRUB, &currency.UpdateTime); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		currencies = append(currencies, currency)
	}

	jsonBytes, err := json.MarshalIndent(currencies, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonBytes)
}