package main

import (
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type OrderResponse struct {
	Orders []Order `json:"orders"`
	Total  int     `json:"total"`
	Page   int     `json:"page"`
	Pages  int     `json:"pages"`
}

type Order struct {
	OrderID   string  `json:"orderNumber"`
	Email     string  `json:"customerEmail"`
	BillTo    BillTo  `json:"billTo"`
	TotalPaid float64 `json:"amountPaid"`
	TaxPaid   float64 `json:"taxAmount"`
}

type BillTo struct {
	State string `json:"state"`
}

func main() {
	// Retrieve the ShipStation API credentials from environment variables
	apiKey := os.Getenv("SSKEY")
	apiSecret := os.Getenv("SSSECRET")

	// Make sure the API credentials are set
	if apiKey == "" || apiSecret == "" {
		log.Println("Please set the SSKEY and SSSECRET environment variables.")
		return
	}

	// Create the Basic Authentication header
	authHeader := fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(apiKey+":"+apiSecret)))

	// Define the start and end dates for the orders query
	startDate := time.Now().AddDate(0, 0, -30) // Start date is 7 days ago
	endDate := time.Now()                      // End date is current date

	// Format the dates to the required ShipStation format
	startDateStr := startDate.Format("2006-01-02")
	endDateStr := endDate.Format("2006-01-02")

	// Specify the limit for the number of orders per page
	limit := 100

	// Make the initial API request to get the total number of pages
	url := fmt.Sprintf("https://ssapi.shipstation.com/orders?orderDateStart=%s&orderDateEnd=%s&pageSize=%d&page=1", startDateStr, endDateStr, limit)
	orderResponse, err := makeAPIRequest(url, authHeader)
	if err != nil {
		log.Println("Error making API request:", err)
		return
	}

	// Process orders for each page
	var stateResults map[string]StateSummary
	stateResults = make(map[string]StateSummary)

	for page := 1; page <= orderResponse.Pages; page++ {
		url := fmt.Sprintf("https://ssapi.shipstation.com/orders?orderDateStart=%s&orderDateEnd=%s&pageSize=%d&page=%d", startDateStr, endDateStr, limit, page)
		orderResponse, err := makeAPIRequest(url, authHeader)
		if err != nil {
			log.Println("Error making API request:", err)
			return
		}

		processOrders(orderResponse.Orders, stateResults)
	}

	// Save state results to CSV file
	if err := saveToCSV(stateResults, "state_results.csv"); err != nil {
		log.Println("Error saving state results to CSV:", err)
		return
	}

	log.Println("State results saved to state_results.csv")
}

// Make an API request and return the OrderResponse
func makeAPIRequest(url, authHeader string) (*OrderResponse, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Authorization", authHeader)

	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	var orderResponse OrderResponse
	err = json.Unmarshal(body, &orderResponse)
	if err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	log.Println("JSON RESPONSE:", orderResponse)

	return &orderResponse, nil
}

// Process orders and update state results
func processOrders(orders []Order, stateResults map[string]StateSummary) {
	for _, order := range orders {
		state := order.BillTo.State
		amountPaid := order.TotalPaid
		taxPaid := order.TaxPaid
		summary, exists := stateResults[state]
		if exists {
			summary.NumOrders++
			summary.TotalPaid += amountPaid
			summary.TotalTaxPaid += taxPaid
			stateResults[state] = summary
		} else {
			stateResults[state] = StateSummary{
				NumOrders:    1,
				TotalPaid:    amountPaid,
				TotalTaxPaid: taxPaid,
			}
		}
	}
}

// StateSummary represents the summary for a specific state
type StateSummary struct {
	NumOrders    int
	TotalPaid    float64
	TotalTaxPaid float64
}

// Save state results to a CSV file
func saveToCSV(stateResults map[string]StateSummary, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("error creating CSV file: %w", err)
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write CSV header
	header := []string{"State", "NumOrders", "TotalPaid", "TotalTaxPaid"}
	err = writer.Write(header)
	if err != nil {
		return fmt.Errorf("error writing CSV header: %w", err)
	}

	// Write state results to CSV
	for state, summary := range stateResults {
		row := []string{
			state,
			fmt.Sprintf("%d", summary.NumOrders),
			fmt.Sprintf("%.2f", summary.TotalPaid),
			fmt.Sprintf("%.2f", summary.TotalTaxPaid),
		}
		err := writer.Write(row)
		if err != nil {
			return fmt.Errorf("error writing CSV row: %w", err)
		}
	}

	return nil
}
