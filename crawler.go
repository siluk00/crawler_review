package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type Book struct {
	Title  string
	Price  string
	Rating string
}

func fetchPage(client *http.Client, url string) (*html.Node, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept-Encoding", "zstd")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	//slog.Info("found ", "body", string(body))
	if err != nil {
		return nil, err
	}

	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func parseBooks(doc *html.Node, results *[]Book) {
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && hasClass(n, "product_pod") {
			book := extractBook(n)
			*results = append(*results, book)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
}

func extractBook(n *html.Node) Book {
	var title, price, rating string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "p":
				if hasClass(n, "star-rating") {
					for _, a := range n.Attr {
						if a.Key == "class" {
							parts := strings.Split(a.Val, " ")
							if len(parts) > 1 {
								rating = parts[1]
							}
						}
					}
				}
				if hasClass(n, "price_color") && n.FirstChild != nil {
					price = n.FirstChild.Data
				}
			case "a":
				for _, a := range n.Attr {
					if a.Key == "title" {
						title = a.Val
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return Book{title, price, rating}
}

func getNextPage(doc *html.Node) string {
	var next string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "li" && hasClass(n, "next") {
			if n.FirstChild != nil && n.FirstChild.NextSibling != nil {
				for _, a := range n.FirstChild.NextSibling.Attr {
					if a.Key == "href" {
						next = a.Val
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return next
}

func crawl() {
	base := "http://books.toscrape.com/catalogue/"
	// 1. o original começava na pagina 2 e terminava com .htm e a pagina é .html
	page := "page-1.html"

	// 2. substitui a variavel global por uma variavel local
	// melhor pra escalabilidade em caso de multiplas goroutines
	var results []Book

	// 3. Reutilização do cliente com timeout
	client := &http.Client{Timeout: 10 * time.Second}

	for page != "" {
		// 4.substituação por slog
		slog.Info("Coletando:", "page", page)

		doc, err := fetchPage(client, base+page)
		if err != nil {
			slog.Error("erro ao buscar página", "page", page, "error", err)
			break
		}

		parseBooks(doc, &results)

		next := getNextPage(doc)
		if next == "" {
			parseBooks(doc, &results)
		}
		page = next
	}

	for _, b := range results {
		fmt.Printf("Título: %s | Preço: %s | Avaliação: %s\n", b.Title, b.Price, b.Rating)
	}
}

func hasClass(n *html.Node, class string) bool {
	for _, a := range n.Attr {
		if a.Key == "class" && strings.Contains(a.Val, class) {
			return true
		}
	}
	return false
}

func main() {
	crawl()
}
