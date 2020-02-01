package main

import (
	"fmt"

	"github.com/corpix/uarand"
	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/queue"
)

func main() {
	// Array containing all the known URLs in a sitemap
	knownUrls := []string{}
	githubUrls := []string{}

	// Create a Collector specifically for Shopify
	c := colly.NewCollector(
		colly.UserAgent(uarand.GetRandom()),
		colly.AllowedDomains("www.kitploit.com"),
		colly.CacheDir("./data/cache"),
	)

	// create a request queue with 2 consumer threads
	q, _ := queue.New(
		64, // Number of consumer threads
		&queue.InMemoryQueueStorage{
			MaxSize: 100000,
		}, // Use default queue storage
	)

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("visiting", r.URL)
	})

	// Create a callback on the XPath query searching for the URLs
	c.OnXML("//sitemap/loc", func(e *colly.XMLElement) {
		knownUrls = append(knownUrls, e.Text)
		q.AddURL(e.Text)
	})

	// Create a callback on the XPath query searching for the URLs
	c.OnXML("//urlset/url/loc", func(e *colly.XMLElement) {
		knownUrls = append(knownUrls, e.Text)
		q.AddURL(e.Text)
	})

	// On every a element which has .top-matter attribute call callback
	// This class is unique to the div that holds all information about a story
	c.OnHTML(".kiploit-download", func(e *colly.HTMLElement) {
		// t := e.ChildAttr("a", "href")
		fmt.Println("found ", e.Attr("href"))
		githubUrls = append(githubUrls, e.Attr("href"))
	})

	q.AddURL("https://www.kitploit.com/sitemap.xml")

	fmt.Println("All github URLs:")

	// Consume URLs
	q.Run(c)

	for _, url := range githubUrls {
		fmt.Println("\t", url)
	}
	fmt.Println("Collected", len(githubUrls), "URLs")
	
}