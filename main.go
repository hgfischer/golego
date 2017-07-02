package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/headzoo/surf"
	"github.com/headzoo/surf/agent"
	"github.com/headzoo/surf/browser"
	"github.com/ngs/go-amazon-product-advertising-api/amazon"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	appName         = path.Base(os.Args[0])
	app             = kingpin.New(appName, "Go LEGO - special searches on Amazon.de")
	accessKeyID     = app.Flag("access-key-id", "AWS Access Key").Required().String()
	secretAccessKey = app.Flag("secret-access-key", "AWS Secret Key").Required().String()
	associateTag    = app.Flag("associate-tag", "Amazon Associate Tag").Required().String()
	csvFile         = app.Arg("CSV", "Name of the CSV file to write").Required().OpenFile(
		os.O_TRUNC|os.O_WRONLY, 0644)
)

func main() {
	var exit bool

	kingpin.MustParse(app.Parse(os.Args[1:]))

	bow := surf.NewBrowser()
	bow.SetUserAgent(agent.Chrome())

	client, err := amazon.New(*accessKeyID, *secretAccessKey, *associateTag, amazon.RegionGermany)
	fatalize(err)

	w := csv.NewWriter(io.MultiWriter(os.Stderr, *csvFile))
	w.Comma = '\t'
	w.Write((&legoItem{}).Headers())

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		exit = true
	}()

	itemPage := 1
	currItem := 0

	for {
		if exit {
			break
		}

		res, err := client.ItemSearch(amazon.ItemSearchParameters{
			OnlyAvailable: true,
			Condition:     amazon.ConditionNew,
			SearchIndex:   amazon.SearchIndexToys,
			Keywords:      "LEGO",
			ResponseGroups: []amazon.ItemSearchResponseGroup{
				amazon.ItemSearchResponseGroupItemAttributes,
				amazon.ItemSearchResponseGroupItemIds,
				amazon.ItemSearchResponseGroupOfferSummary,
			},
			ItemPage: itemPage,
		}).Do()
		printError(err)
		printError(res.Error())

		for _, item := range res.Items.Item {
			if exit {
				break
			}

			currItem++
			fmt.Printf("Item %d of %d >> ", currItem, res.Items.TotalResults)

			rec := newLegoItem(&item, bow)

			w.Write(rec.Columns())
			w.Flush()
			printError(w.Error())
		}

		if res.Items.TotalPages > itemPage {
			itemPage++
		} else {
			break
		}
	}

	w.Flush()
	(*csvFile).Close()
	log.Println("DONE!")
}

type legoItem struct {
	ASIN         string
	Title        string
	listPrice    string
	ListPrice    float64
	lowestPrice  string
	LowestPrice  float64
	parts        string
	Parts        int
	weight       string
	WeightGrams  int
	URL          string
	PricePerPart float64
	PricePerGram float64
}

func newLegoItem(item *amazon.Item, bow *browser.Browser) *legoItem {
	li := &legoItem{
		ASIN:        item.ASIN,
		Title:       item.ItemAttributes.Title,
		URL:         item.DetailPageURL,
		listPrice:   item.ItemAttributes.ListPrice.Amount,
		lowestPrice: item.OfferSummary.LowestNewPrice.Amount,
	}

	err := bow.Open(item.DetailPageURL)
	printError(err)

	lastLabel := ""
	kvs := map[string]string{}
	bow.Find("div.pdTab td").Each(func(i int, td *goquery.Selection) {
		if td.HasClass("label") {
			lastLabel = strings.TrimSpace(td.Text())
		}
		if td.HasClass("value") {
			kvs[lastLabel] = strings.TrimSpace(td.Text())
		}
	})

	for k, v := range kvs {
		if strings.Contains(k, "Anzahl Teile") {
			li.parts = v
		}
		if strings.Contains(k, "Artikelgewicht") {
			li.weight = v
		}
	}

	li.ListPrice, _ = strconv.ParseFloat(li.listPrice, 64)
	li.ListPrice /= 100
	li.LowestPrice, _ = strconv.ParseFloat(li.lowestPrice, 64)
	li.LowestPrice /= 100
	li.Parts, _ = strconv.Atoi(li.parts)

	if strings.Contains(li.weight, "Kg") {
		w := strings.Replace(li.weight, ",", ".", -1)
		w = strings.Replace(w, "Kg", "", -1)
		w = strings.TrimSpace(w)
		f, _ := strconv.ParseFloat(w, 32)
		li.WeightGrams = int(f * 100)
	} else if strings.Contains(li.weight, "g") {
		w := strings.TrimSpace(strings.Replace(li.weight, "g", "", -1))
		li.WeightGrams, _ = strconv.Atoi(w)
	}

	if li.Parts > 0 {
		li.PricePerPart = li.LowestPrice / float64(li.Parts)
	}
	if li.WeightGrams > 0 {
		li.PricePerGram = li.LowestPrice / float64(li.WeightGrams)
	}

	return li
}

func (li *legoItem) Headers() []string {
	rec := []string{}
	rec = append(rec, "ASIN")
	rec = append(rec, "Title")
	rec = append(rec, "List Price")
	rec = append(rec, "Lowest Price")
	rec = append(rec, "Parts")
	rec = append(rec, "Weight (g)")
	rec = append(rec, "URL")
	rec = append(rec, "Price/Part")
	rec = append(rec, "Price/Gram")
	return rec
}

func (li *legoItem) Columns() []string {
	rec := []string{}
	rec = append(rec, li.ASIN)
	rec = append(rec, li.Title)
	rec = append(rec, fmt.Sprintf("%.02f", li.ListPrice))
	rec = append(rec, fmt.Sprintf("%.02f", li.LowestPrice))
	rec = append(rec, fmt.Sprintf("%d", li.Parts))
	rec = append(rec, fmt.Sprintf("%d", li.WeightGrams))
	rec = append(rec, fmt.Sprintf("%.03f", li.PricePerPart))
	rec = append(rec, fmt.Sprintf("%.03f", li.PricePerGram))
	rec = append(rec, li.URL)
	return rec
}

func printError(err error) {
	if err != nil {
		log.Println(err)
	}
}

func fatalize(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
