package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

const fetchURL = "http://web.archive.org/cdx/search/cdx?url=*.%s/*&output=json&collapse=urlkey"

func main() {

	var domains []string

	var dates bool
	flag.BoolVar(&dates, "dates", false, "show date of fetch in the first column")

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

		wurls, err := getWaybackURLs(domain)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to fetch URLs for [%s]\n", domain)
			continue
		}

		for _, w := range wurls {
			if dates {

				d, err := time.Parse("20060102150405", w.date)
				if err != nil {
					fmt.Fprintf(os.Stderr, "failed to parse date [%s] for URL [%s]\n", w.date, w.url)
				}

				fmt.Printf("%s %s\n", d.Format(time.RFC3339), w.url)

			} else {
				fmt.Println(w.url)
			}
		}
	}

}

type wurl struct {
	date string
	url  string
}

func getWaybackURLs(domain string) ([]wurl, error) {

	res, err := http.Get(fmt.Sprintf(fetchURL, domain))
	if err != nil {
		return []wurl{}, err
	}

	raw, err := ioutil.ReadAll(res.Body)

	res.Body.Close()
	if err != nil {
		return []wurl{}, err
	}

	var wrapper [][]string
	err = json.Unmarshal(raw, &wrapper)

	out := make([]wurl, 0, len(wrapper))

	skip := true
	for _, urls := range wrapper {
		// The first item is always just the string "original",
		// so we should skip the first item
		if skip {
			skip = false
			continue
		}
		out = append(out, wurl{date: urls[1], url: urls[2]})
	}

	return out, nil

}
