package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

type config struct {
	target string
	result string
	dates  bool
}

type wurl struct {
	date string
	url  string
}

var (
	cfg config
)

func loadConfig() {
	t := flag.String("target", "", "Target Domain")
	r := flag.String("result", "", "Result location")
	d := flag.Bool("dates", false, "show date of fetch in the first column")

	flag.Parse()

	cfg = config{
		target: *t,
		result: *r,
		dates:  *d,
	}

	validateParams()
}

func validateParams() {
	var didError = false

	if cfg.target == "" {
		log.Println("Error: target is a required parameter, cannot be blank.")
		didError = true
	}

	if cfg.result == "" {
		log.Println("Error: result is a required parameter, cannot be blank.")
		didError = true
	}

	if didError {
		log.Fatalf("Usage: waybackurls -target TODO -result TODO")
		os.Exit(1)
	}
}

const fetchURL = "http://web.archive.org/cdx/search/cdx?url=*.%s/*&output=json&collapse=urlkey"

func main() {

	loadConfig()

	var domains []string

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

		f, err := os.Create(cfg.result)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		for _, w := range wurls {
			if cfg.dates {

				d, err := time.Parse("20060102150405", w.date)
				if err != nil {
					fmt.Fprintf(os.Stderr, "failed to parse date [%s] for URL [%s]\n", w.date, w.url)
				}

				f.WriteString(fmt.Sprintf("%s %s\n", d.Format(time.RFC3339), w.url))

			} else {
				f.WriteString(w.url + "\n")
			}
		}

		f.Sync()
	}

}

func getWaybackURLs(domain string) ([]wurl, error) {

	c := &http.Client{
		Timeout: 300 * time.Second, //TODO: keep trying if timeout occurs
	}

	res, err := c.Get(fmt.Sprintf(fetchURL, domain))
	if err != nil {
		return []wurl{}, err
	}

	defer res.Body.Close()

	raw, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return []wurl{}, err
	}

	var wrapper [][]string
	err = json.Unmarshal(raw, &wrapper)

	out := make([]wurl, 0, len(wrapper))

	if res.StatusCode == 200 {
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

	} else {
		out = append(out, wurl{date: time.Now().String(), url: "http://" + cfg.target + "\n"})
	}

	return out, nil

}
