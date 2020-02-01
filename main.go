package main

import (
	"fmt"
	"strings"
	"context"
	"os"
	"time"

	"github.com/k0kubun/pp"
	"github.com/corpix/uarand"
	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/queue"
	"github.com/google/go-github/v29/github"
	log "github.com/sirupsen/logrus"
	"github.com/x0rzkov/go-vcsurl"
	"github.com/iancoleman/strcase"
	ghclient "github.com/x0rzkov/gocolly-kitploit/pkg/client"
)

var (
	clientManager   *ghclient.ClientManager
	clientGH        *ghclient.GHClient
	cachePath 		= "./data/httpcache"
	debug 			= false
	logLevelStr 	= "info"
)

type VcsInfo struct {
	Desc string
	URL string 
	Lang string
	Tags []string
}

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
		log.Println("visiting", r.URL)
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
		log.Println("found href=", e.Attr("href"))
		if strings.HasPrefix(e.Attr("href"), "https://github.com") {
			githubUrls = append(githubUrls, e.Attr("href"))
		}
	})

	q.AddURL("https://www.kitploit.com/sitemap.xml")

	log.Println("All github URLs:")

	// Consume URLs
	q.Run(c)

	// github client init
	clientManager = ghclient.NewManager(cachePath, []string{os.Getenv("GITHUB_TOKEN")})
	defer clientManager.Shutdown()
	clientGH = clientManager.Fetch()

	var vcsRepos []*VcsInfo
	for _, url := range githubUrls {
		fmt.Println("\t", url)
		if info, err := vcsurl.Parse(url); err == nil {
			repoInfo, err := getInfo(clientGH.Client, info.Username, info.Name)
			if err != nil {
				log.Fatal(err)
			}
			var topicCamel []string
			topicCamel = append(topicCamel, *repoInfo.Language)
			topics, err := getTopics(clientGH.Client, info.Username, info.Name)
			if err == nil {
				for _, topic := range topics {
					topicCamel = append(topicCamel, strcase.ToCamel(strings.Replace(topic, "-", "", -1)))
				}
			}
			vcsRepo := &VcsInfo{
				Desc: *repoInfo.Description,
				URL: url,
				Tags: topicCamel,
				Lang: *repoInfo.Language,
			}
			pp.Println(vcsRepo)			
			vcsRepos = append(vcsRepos, vcsRepo)
		}
	}
	log.Println("Collected", len(githubUrls), "URLs")
	
}

func getInfo(client *github.Client, owner, name string) (*github.Repository, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	waitForRemainingLimit(client, true, 10)
	info, _, err := client.Repositories.Get(ctx, owner, name)
	if err != nil {
		return nil, err
	}
	return info, nil
}

func getTopics(client *github.Client, owner, name string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	waitForRemainingLimit(client, true, 10)
	topics, _, err := client.Repositories.ListAllTopics(ctx, owner, name)
	if err != nil {
		return nil, err
	}
	return topics, nil
}

func waitForRemainingLimit(cl *github.Client, isCore bool, minLimit int) {
	for {
		rateLimits, _, err := cl.RateLimits(context.Background())
		if err != nil {
			if debug {
				log.Printf("could not access rate limit information: %s\n", err)
			}
			<-time.After(time.Second * 1)
			continue
		}

		var rate int
		var limit int
		if isCore {
			rate = rateLimits.GetCore().Remaining
			limit = rateLimits.GetCore().Limit
		} else {
			rate = rateLimits.GetSearch().Remaining
			limit = rateLimits.GetSearch().Limit
		}

		if rate < minLimit {
			if debug {
				log.Printf("Not enough rate limit: %d/%d/%d\n", rate, minLimit, limit)
			}
			<-time.After(time.Second * 60)
			continue
		}
		if debug {
			log.Printf("Rate limit: %d/%d\n", rate, limit)
		}
		break
	}
}