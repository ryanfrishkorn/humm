package humm

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

type LinkStats struct {
	Url          url.URL
	StatusCode   int
	TimeRequest  time.Time
	TimeResponse time.Time
}

type SpiderSummary struct {
	Results map[int][]LinkStats
}

// MakeAbsolute adds host and path information for relative urls
func MakeAbsolute(urlNew url.URL, urlBase url.URL) url.URL {
	// copy scheme and host from urlBase
	if urlNew.Scheme == "" {
		urlNew.Scheme = urlBase.Scheme
	}
	if urlNew.Host == "" {
		urlNew.Host = urlBase.Host
	}
	if urlNew.Path == "" {
		urlNew.Path = urlBase.Path
	}

	return urlNew
}

// AddBasicAuth adds username/password to url.URL derived from another URL
func AddBasicAuth(urlNew url.URL, urlBase url.URL) url.URL {
	urlNew.User = urlBase.User
	return urlNew
}

// RemoveBasicAuth removes username/password from url.URL
func RemoveBasicAuth(urlNew url.URL) url.URL {
	urlNew.User = nil
	return urlNew
}

// IsInternal returns true if link is within domain
func IsInternal(urlOther url.URL, urlBase *url.URL) bool {
	if urlOther.Host == urlBase.Host {
		return true
	}
	return false
}

// IsExternal returns true if link is outside domain
func IsExternal(urlOther url.URL, urlBase *url.URL) bool {
	if urlOther.Host != urlBase.Host {
		return true
	}
	return false
}

// DeterminePageType returns a string identifier for the type of page
// based on criteria of url structure.
func DeterminePageType(urlPage url.URL) string {

	// associate path structure with unique string
	// may change to actual types if extended
	type pageType struct {
		re   *regexp.Regexp
		kind string
	}

	var allTypes []pageType

	allTypes = append(allTypes, pageType{
		kind: "index",
		re:   regexp.MustCompile("^/$"),
	}, pageType{
		kind: "technology",
		re:   regexp.MustCompile("^/technology/[a-z0-9-]+/?$"),
	}, pageType{
		kind: "project",
		re:   regexp.MustCompile("^/projects/[a-z0-9-]+/?$"),
	}, pageType{
		kind: "static",
		re:   regexp.MustCompile("^/[a-z0-9-]+/?$"),
	})

	// return first match
	for _, t := range allTypes {
		if t.re.MatchString(urlPage.Path) {
			return t.kind
		}
	}

	// unknown
	return "unknown"
}

// CheckXPath looks for the presence of a particular xpath
// and returns the InnerText of the element
func CheckXPath(doc *html.Node, path string) string {
	node := htmlquery.FindOne(doc, path)

	if node == nil {
		return ""
	}

	return fmt.Sprintf("%s", strings.TrimSpace(htmlquery.InnerText(node)))
}

// GetStatusCode makes a http request and records stats about
// the response and the duration of the operation
func GetStatusCode(urlPage url.URL, timeoutSeconds int) (LinkStats, error) {
	stats := LinkStats{
		Url:         urlPage,
		TimeRequest: time.Now(),
	}

	httpClient := http.Client{
		Timeout: time.Duration(timeoutSeconds) * time.Second,
	}
	response, err := httpClient.Get(urlPage.String())
	stats.TimeResponse = time.Now()
	if err != nil {
		return stats, errors.New("error getting status code for url")
	}

	stats.StatusCode = response.StatusCode

	return stats, nil
}
