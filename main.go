package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"

	"github.com/sethgrid/pester"
)

type config struct {
	target     string
	result     string
	dates      bool
	targetFile string
}

var (
	cfg config
)

func loadConfig() {
	t := flag.String("target", "", "Target Domain")
	r := flag.String("result", "", "Result location")
	d := flag.Bool("dates", false, "show date of fetch in the first column")
	tf := flag.String("targetFile", "", "Target File of Domains")

	flag.Parse()

	cfg = config{
		target:     *t,
		result:     *r,
		dates:      *d,
		targetFile: *tf,
	}

	validateParams()
}

func validateParams() {
	var didError = false

	if cfg.target == "" && cfg.targetFile == "" {
		log.Println("Error: Either target or targetFile is a required parameter, both cannot be blank. If both are provided, targetFile is given preference")
		didError = true
	}

	if cfg.result == "" {
		log.Println("Error: result is a required parameter, cannot be blank.")
		didError = true
	}

	if didError {
		log.Fatalf("Usage: waybackurls -target TODO -result TODO -targetFile TODO -dates")
		os.Exit(1)
	}
}

const wayBackURL = "http://web.archive.org/cdx/search/cdx?url=*.%s/*&output=json&collapse=urlkey"
const commonCrawlURL = "http://index.commoncrawl.org/CC-MAIN-2018-22-index?url=*.%s&output=json"

func main() {

	loadConfig()

	var domains []string

	if cfg.targetFile == "" {
		// fetch for a single domain
		domains = []string{cfg.target}
	} else {

		f, err := os.Open(cfg.targetFile)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		// fetch for all domains from the target file
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			domains = append(domains, sc.Text())
		}

		if err := sc.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to read input: %s\n", err)
		}
	}

	f, err := os.Create(cfg.result)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	fetchFns := []fetchFn{getWaybackURLs, getCommonCrawlURLs}

	for _, domain := range domains {

		var wg sync.WaitGroup
		wurls := make(chan wurl)

		for _, fn := range fetchFns {
			wg.Add(1)
			fetch := fn
			go func() {
				defer wg.Done()
				resp, err := fetch(domain)
				if err != nil {
					fmt.Fprintf(os.Stderr, "failed to fetch URLs for [%s]\n", domain)
					f.WriteString("http://" + domain)
					return
				}
				for _, r := range resp {
					wurls <- r
				}
			}()
		}

		go func() {
			wg.Wait()
			close(wurls)
		}()

		seen := make(map[string]bool)
		for w := range wurls {
			if _, ok := seen[w.url]; ok {
				continue
			}
			seen[w.url] = true

			if cfg.dates {

				d, err := time.Parse("20060102150405", w.date)
				if err != nil {
					fmt.Fprintf(os.Stderr, "WARNING: failed to parse date [%s] for URL [%s]\n", w.date, w.url)
				}

				f.WriteString(fmt.Sprintf("%s %s\n", d.Format(time.RFC3339), w.url))

			} else {
				f.WriteString(w.url + "\n")
			}
		}

		f.Sync()
	}

}

type wurl struct {
	date string
	url  string
}

type fetchFn func(string) ([]wurl, error)

func getWaybackURLs(domain string) ([]wurl, error) {

	c := pester.New()
	c.MaxRetries = 10
	c.KeepLog = true
	c.Backoff = pester.ExponentialBackoff
	c.Timeout = 300 * time.Second

	res, err := c.Get(fmt.Sprintf(wayBackURL, domain))
	if err != nil {
		fmt.Println(c.LogString())
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
		out = append(out, wurl{date: "NA", url: "http://" + domain})
	}

	return out, nil

}

func getCommonCrawlURLs(domain string) ([]wurl, error) {

	c := pester.New()
	c.MaxRetries = 10
	c.KeepLog = true
	c.Backoff = pester.ExponentialBackoff
	c.Timeout = 300 * time.Second

	res, err := c.Get(fmt.Sprintf(commonCrawlURL, domain))
	if err != nil {
		fmt.Println(c.LogString())
		return []wurl{}, err
	}

	defer res.Body.Close()

	sc := bufio.NewScanner(res.Body)

	out := make([]wurl, 0)

	if res.StatusCode == 200 {
		for sc.Scan() {

			wrapper := struct {
				URL       string `json:"url"`
				Timestamp string `json:"timestamp"`
			}{}
			err = json.Unmarshal([]byte(sc.Text()), &wrapper)

			if err != nil {
				continue
			}

			out = append(out, wurl{date: wrapper.Timestamp, url: wrapper.URL})
		}
	} else {
		out = append(out, wurl{date: "NA", url: "http://" + domain})
	}

	return out, nil

}
