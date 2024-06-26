package main

import (
	"flag"
	"fmt"
	"github.com/antchfx/htmlquery"
	"github.com/ryanfrishkorn/humm"
	"golang.org/x/net/html"
	"net/http"
	"net/url"
	"os"
	"sort"
)

func main() {
	var basicAuth = false

	// handle arguments and flags
	flagHelp := flag.Bool("h", false, "print help message")
	flagUser := flag.String("u", "", "username for basic auth")
	flagPass := flag.String("p", "", "password for basic auth")
	flagCrawl := flag.Bool("c", false, "crawl all links and report urls with non-200 status codes")
	flagVerbose := flag.Bool("v", false, "verbose output of link counts")
	flagHttpTimeout := flag.Int("timeout", 20, "specify timeout for http status codes")
	flagLinksLimit := flag.Int("l", 0, "limit links to crawl")
	flagMaxThreads := flag.Int("m", 10, "set concurrent crawl threads")
	flagShow200 := flag.Bool("200", false, "include urls with status code 200 in crawl report")
	flagSortField := flag.String("s", "url", "sort by specified field <url|time>")
	flagTimeResponse := flag.Bool("t", false, "print time elapsed between request and response")

	flag.Usage = func() {
		fmt.Printf("usage: humm [flags] <url>\n")
		flag.PrintDefaults()

		os.Exit(1)
	}

	flag.Parse()

	if *flagHelp {
		flag.Usage()
	}

	// expect one argument for specifying path
	if flag.NArg() != 1 {
		fmt.Printf("error parsing arguments\n")
		flag.Usage()
		os.Exit(1)
	}

	// create root url object to pass for other url construction
	urlRoot, err := url.Parse(flag.Arg(0))
	fmt.Printf("url: %s\n", urlRoot.String())

	// for basic auth
	if *flagUser != "" && *flagPass != "" {
		// use whitelist for hosts when adding basic auth to avoid user/pass leak
		allowedHosts := []string{
			"mystic.com",
			"staging.mystic.com",
		}

		if containsHost(urlRoot.Host, allowedHosts) == false {
			fmt.Printf("cannot add basic auth outside of %s\n", allowedHosts)
			os.Exit(1)
		}

		urlRoot.User = url.UserPassword(*flagUser, *flagPass)
		basicAuth = true
	}

	// load document and check status code
	response, err := http.Get(urlRoot.String())
	if err != nil {
		fmt.Printf("error %v\n", err)
		os.Exit(1)
	}

	// check for proper status
	if response.StatusCode != 200 {
		fmt.Printf("received status code %d\n", response.StatusCode)
		os.Exit(1)
	}

	// parse
	doc, err := htmlquery.Parse(response.Body)
	if err != nil {
		fmt.Printf("could not load url\n")
		os.Exit(1)
	}

	// exit if we do not wish to crawl page links
	if *flagCrawl == false {
		os.Exit(0)
	}

	// gather all links
	linksAll := gatherLinks(doc)

	// make all links absolute
	linksAll = makeLinksAbsolute(linksAll, urlRoot)

	// separate internal and external links (also removes duplicates)
	linksInternal, linksExternal := splitInternalExternalLinks(linksAll, urlRoot)

	if *flagVerbose {
		fmt.Printf("links_total: %d\n", len(linksAll))
		fmt.Printf("links_internal_uniq: %d\n", len(linksInternal))
		fmt.Printf("links_external_uniq: %d\n", len(linksExternal))
	}

	// truncate links before iteration if specified
	if *flagLinksLimit != 0 && len(linksInternal) > *flagLinksLimit {
		linksInternal = linksInternal[:*flagLinksLimit] // only spider this many found links
	}

	ch := make(chan humm.LinkStats, *flagMaxThreads)
	for _, link := range linksInternal {
		// fetch response codes async
		link := link
		go func() {
			if basicAuth && humm.IsInternal(link, urlRoot) {
				link = humm.AddBasicAuth(link, *urlRoot)
			}

			linkStats, err := humm.GetStatusCode(link, *flagHttpTimeout)
			if err != nil {
				fmt.Printf("could not get status code for %s\n", linkStats.Url.String())
				os.Exit(1)
			}

			ch <- linkStats
		}()
	}

	// store results in an organized way
	summary := humm.SpiderSummary{}
	summary.Results = make(map[int][]humm.LinkStats)

	// read results from channel
	fmt.Printf("crawling internal links: ")
	count := 0
	for range linksInternal {
		stat := <-ch

		// count to report progress
		count++
		countPrevLen := len(fmt.Sprintf("%d/%d", count-1, len(linksInternal)))
		if count != 1 {
			for ; countPrevLen > 0; countPrevLen-- {
				fmt.Fprintf(os.Stderr, "\b")
			}
		}
		fmt.Fprintf(os.Stderr, "%d/%d", count, len(linksInternal))
		// check for presence of key
		_, ok := summary.Results[stat.StatusCode]
		if !ok {
			// add results to summary
			summary.Results[stat.StatusCode] = make([]humm.LinkStats, 0)
		}
		summary.Results[stat.StatusCode] = append(summary.Results[stat.StatusCode], stat)
	}

	fmt.Fprintf(os.Stderr, "\033[2K")
	fmt.Fprintf(os.Stderr, "\r")

	statusKeys := make([]int, 0)

	// gather and sort keys
	for statusKey := range summary.Results {
		for _, key := range statusKeys {
			if statusKey == key {
				continue
			}
		}
		statusKeys = append(statusKeys, statusKey)
	}
	sort.Slice(statusKeys, func(i int, j int) bool { return statusKeys[i] < statusKeys[j] })

	// print summary
	for _, statusKey := range statusKeys {
		fmt.Printf("%d: %d\n", statusKey, len(summary.Results[statusKey]))
		// sort async result urls for easy reading and comparison by default
		if *flagSortField == "url" {
			sort.Slice(summary.Results[statusKey], func(i int, j int) bool {
				return summary.Results[statusKey][i].Url.String() < summary.Results[statusKey][j].Url.String()
			})
		}

		for _, stat := range summary.Results[statusKey] {
			// skip 200's listing if specified
			if statusKey == 200 && *flagShow200 == false {
				continue
			}

			stat.Url = humm.RemoveBasicAuth(stat.Url)
			elapsed := stat.TimeResponse.Sub(stat.TimeRequest)
			if *flagTimeResponse {
				fmt.Printf(" - [%dms] %s [%s]\n", elapsed.Milliseconds(), stat.Url.String(), humm.DeterminePageType(stat.Url))
			} else {
				fmt.Printf(" - %s [%s]\n", stat.Url.String(), humm.DeterminePageType(stat.Url))
			}
		}
	}
	os.Exit(0)
}

func containsHost(host string, allowedHosts []string) bool {
	for _, allowedHost := range allowedHosts {
		if host == allowedHost {
			return true
		}
	}
	return false
}

func gatherLinks(doc *html.Node) []url.URL {
	// gather all links on page
	links := htmlquery.Find(doc, "//a@href")
	var linksFiltered []url.URL

	// parse all links
	for _, link := range links {
		linkAttr := htmlquery.SelectAttr(link, "href")
		linkParsed, err := url.Parse(linkAttr)
		if err != nil {
			fmt.Printf("error parsing %s\n", linkAttr)
			os.Exit(1)
		}
		// make absolute
		linksFiltered = append(linksFiltered, *linkParsed)
	}

	return linksFiltered
}

func makeLinksAbsolute(links []url.URL, urlRoot *url.URL) []url.URL {
	var linksFiltered []url.URL
	for _, link := range links {
		linkAbsolute := humm.MakeAbsolute(link, *urlRoot)
		linksFiltered = append(linksFiltered, linkAbsolute)
	}
	return linksFiltered
}

func splitInternalExternalLinks(links []url.URL, urlRoot *url.URL) ([]url.URL, []url.URL) {
	var linksInternal []url.URL
	var linksExternal []url.URL
	for _, link := range links {
		if humm.IsInternal(link, urlRoot) {
			linksInternal = append(linksInternal, link)
		} else {
			linksExternal = append(linksExternal, link)
		}
	}
	linksInternal = makeLinksUniq(linksInternal)
	linksExternal = makeLinksUniq(linksExternal)

	return linksInternal, linksExternal
}

func makeLinksUniq(links []url.URL) []url.URL {
	var linksFiltered []url.URL
	for _, link := range links {
		found := false
		for _, x := range linksFiltered {
			if x == link {
				found = true
				break
			}
		}
		if found == false {
			linksFiltered = append(linksFiltered, link)
		}
	}
	return linksFiltered
}
