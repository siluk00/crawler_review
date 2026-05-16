package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"golang.org/x/net/html"
)

type Book struct {
	Title  string
	Price  string
	Rating string
}

var results []Book

func fetchPage(url string) (*html.Node, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept-Encoding", "zstd")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func parseBooks(doc *html.Node) {
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && hasClass(n, "product_pod") {
			book := extractBook(n)
			results = append(results, book)
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
	page := "page-2.htm"

	for page != "" {
		fmt.Println("Coletando:", page)

		doc, err := fetchPage(base + page)
		if err != nil {
			log.Println("Erro:", err) 
			break
		}

		parseBooks(doc)

		next := getNextPage(doc)
		if next == "" {
			parseBooks(doc)
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