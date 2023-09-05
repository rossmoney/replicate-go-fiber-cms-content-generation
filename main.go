package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
	html "github.com/gofiber/template/html/v2"
	clover "github.com/ostafen/clover"
	replicate "github.com/replicate/replicate-go"
)

type ProductDetail struct {
	Product string `json:"product"`
	Prompt  string `json:"prompt"`
	Details string `json:"details"`
}

func main() {

	// Create a new engine
	engine := html.New("./views", ".html")

	app := fiber.New(fiber.Config{
		Views: engine,
	})

	app.Get("/", func(c *fiber.Ctx) error {

		productName := c.Query("product")
		errStr := ""

		if productName == "" {
			errStr = "Please enter a product on the query string (?product=)"
		}

		if productName != "" {
			client, err := replicate.NewClient(replicate.WithToken("r8_Y0L4FEggwTszo8jCoH2OEQJQABHkahn24wS1Q"))
			if err != nil {
				log.Panicf("Failed to initialize replicate: %v", err)
			}

			// https://replicate.com/a16z-infra/llama-2-13b-chat/versions
			version := "9dff94b1bed5af738655d4a7cbcdcde2bd503aa85c94334fe1f42af7f3dd5ee3"

			input := replicate.PredictionInput{
				"prompt": "write me a 200 word product description of " + productName + ", that could be used on a cms product detail page",
			}

			webhook := replicate.Webhook{
				URL:    "https://5ab9-31-94-14-167.ngrok-free.app/webhook?product=" + productName,
				Events: []replicate.WebhookEventType{"completed"},
			}

			prediction, err := client.CreatePrediction(context.Background(), version, input, &webhook, false)
			if err != nil {
				log.Panicf("Failed to create prediction: %v", err)
			}

			fmt.Println(prediction.Status)
		}

		db, err := clover.Open("clover.db")
		if err != nil {
			log.Panicf("Failed to open db: %v", err)
		}

		// Check if collection already exists
		collectionExists, err := db.HasCollection("product-details")
		if err != nil {
			log.Panicf("Failed to check collection: %v", err)
		}

		productDetails := []ProductDetail{}

		if collectionExists {
			// Sort product details by id (default)
			docs, _ := db.Query("product-details").Sort(clover.SortOption{Field: "product", Direction: 1}).FindAll()

			for _, doc := range docs {
				productDetail := ProductDetail{
					Product: doc.Get("product").(string),
					Prompt:  doc.Get("prompt").(string),
					Details: doc.Get("output").(string),
				}

				productDetails = append(productDetails, productDetail)
			}

		} else {
			errStr = "No products"
		}

		defer db.Close()

		return c.Render("index", fiber.Map{
			"Title":          "Product Details",
			"ProductName":    productName,
			"ProductDetails": productDetails,
			"Error":          errStr,
		})

	})

	app.Post("/webhook", func(c *fiber.Ctx) error {

		productName := c.Query("product")

		type Input struct {
			Prompt string `json:"prompt"`
		}

		type Output []string

		type IncomingData struct {
			*Input  `json:"input"`
			*Output `json:"output"`
		}

		input := &Input{}
		output := &Output{}
		data := IncomingData{
			Input:  input,
			Output: output,
		}

		if err := c.BodyParser(&data); err != nil {
			return err
		}

		db, err := clover.Open("clover.db")
		if err != nil {
			log.Panicf("Failed to open db: %v", err)
		}
		defer db.Close()

		// Check if collection already exists
		collectionExists, err := db.HasCollection("product-details")
		if err != nil {
			log.Panicf("Failed to check collection: %v", err)
		}

		if !collectionExists {
			// Create a collection named 'product-details'
			db.CreateCollection("product-details")
		}

		exists, _ := db.Query("product-details").Where(clover.Field("product").Eq(productName)).FindAll()
		if len(exists) == 0 {
			// Create a document
			output := strings.Join(*data.Output, "")
			output = strings.Replace(output, "Sure thing! Here's a 200-word product description", "", -1)
			output = strings.Replace(output, "Sure! Here's a 200-word product description", "", -1)
			productDetailsDoc := clover.NewDocument()
			productDetailsDoc.Set("output", output)
			productDetailsDoc.Set("prompt", data.Input.Prompt)
			productDetailsDoc.Set("product", productName)

			// InsertOne returns the id of the inserted document
			docId, _ := db.InsertOne("product-details", productDetailsDoc)
			fmt.Println(docId)
		}

		return c.SendStatus(200)
	})

	app.Listen(":3000")
}
