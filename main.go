package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

const fetchURL = "http://web.archive.org/cdx/search/cdx?url=*.%s/*&output=json&fl=original&collapse=urlkey"

func main() {

	var domains []string

	flag.Parse()

	if flag.NArg() > 0 {
		// fetch for a single domain
		domains = []string{flag.Arg(0)}
	} else {

		// fetch for all domains from stdin
		sc := bufio.NewScanner(os.Stdin)
		for sc.Scan() {
			domains = append(domains, sc.Text())
		}

		if err := sc.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to read input: %s\n", err)
		}
	}

	for _, domain := range domains {

		urls, err := getWaybackURLs(domain)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to fetch URLs for [%s]\n", domain)
			continue
		}

		for _, url := range urls {
			fmt.Println(url)
		}
	}

}

func getWaybackURLs(domain string) ([]string, error) {

	out := make([]string, 0)

	res, err := http.Get(fmt.Sprintf(fetchURL, domain))
	if err != nil {
		return out, err
	}

	raw, err := ioutil.ReadAll(res.Body)

	res.Body.Close()
	if err != nil {
		return out, err
	}

	var wrapper [][]string
	err = json.Unmarshal(raw, &wrapper)

	skip := true
	for _, urls := range wrapper {
		// The first item is always just the string "original",
		// so we should skip the first item
		if skip {
			skip = false
			continue
		}
		out = append(out, urls...)
	}

	return out, nil

}
