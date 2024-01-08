# humm
This is a minimalist web spider built custom for my personal site. It is designed to check the availability and performance of web pages concurrently.

### Install
```
cd cmd/humm
go build
go install
```

### Usage
```
usage: humm [flags] <url>
  -200
        include urls with status code 200 in crawl report
  -c    crawl all links and report urls with non-200 status codes
  -h    print help message
  -l int
        limit links to crawl
  -m int
        set concurrent crawl threads (default 10)
  -p string
        password for basic auth
  -s string
        sort by specified field <url|time> (default "url")
  -t    print time elapsed between request and response
  -u string
        username for basic auth
exit status 1
```

### Examples
```

# crawl internal links on page for response codes
# by default it will report links with status code other than 200
humm -c https://www.example.com

# same, but use basic auth
humm -c -u user -p pass https://www.example.com

# print 200's also
humm -c -200 https://www.example.com

# show elapsed time for each link crawled
humm -c -200 -t https://www.example.com

# sort by response time
humm -c -200 -t -s time https://www.example.com
```
