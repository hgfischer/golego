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

	"sort"

	"time"

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
	startPrice      = app.Flag("start-price", "Starting search price").Default("700").Int()
	maxMinPrice     = app.Flag("max-min-price", "Maximum minimum price").Default("50000").Int()
	priceIncrement  = app.Flag("price-increment", "Price increment").Default("100").Int()
	csvFile         = app.Arg("CSV", "Name of the CSV file to write").Required().OpenFile(
		os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)

	keywords = []string{
		"101 Dalmatians",
		"Action Man",
		"Angry Birds",
		"Aschenputtel",
		"Asterix",
		"Bakugan",
		"Barbie",
		"Batman",
		"Ben 10",
		"Benjamin the Elephant",
		"Beyblade",
		"Bionicle",
		"Bob The Builder",
		"Bolt",
		"Calimero",
		"Cars",
		"Chuggington",
		"Cinderella",
		"Despicable Me",
		"Dinosaurs",
		"Disney",
		"Disney Princess",
		"Doc McStuffins",
		"Doctor Who",
		"Donald & Friends",
		"Dora",
		"Filly",
		"GI Joe",
		"Halo",
		"Harry Potter",
		"Hello Kitty",
		"Indiana Jones",
		"Jake and the Never Land Pirates",
		"Kikaninchen",
		"Kung Fu Panda",
		"Little Mermaid",
		"Lord of the Rings",
		"Löwenzahn",
		"Magic: The Gathering",
		"Marvel",
		"Masters of the Universe",
		"Maulwurf",
		"Maya the Bee",
		"Mickey Mouse & Friends",
		"Monster High",
		"Monsters, Inc.",
		"Masha and The Bear",
		"Moshi Monsters",
		"Nemo",
		"Peanuts",
		"Pippi Longstocking",
		"City",
		"Duplo",
		"Construction Set",
		"Technic",
		"Vehicles",
		"Vehicle",
		"Creator",
		"City",
		"Train",
		"Building site",
		"3 in 1",
		"Excavator",
		"With remote control",
		"Policeman",
		"Architecture",
	}
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

	for _, keyword := range keywords {
		if exit {
			break
		}

		for minPrice := *startPrice; minPrice < *maxMinPrice; minPrice += *priceIncrement {
			itemPage := 1
			currItem := 0
			if exit {
				break
			}

			for {
				if exit {
					break
				}

				fmt.Printf("\nKeywords `LEGO %s`, Price range %d -> %d\n",
					keyword, minPrice, minPrice+*priceIncrement)

				res, err := client.ItemSearch(amazon.ItemSearchParameters{
					OnlyAvailable: true,
					Condition:     amazon.ConditionNew,
					SearchIndex:   amazon.SearchIndexToys,
					Keywords:      "LEGO " + keyword,
					ResponseGroups: []amazon.ItemSearchResponseGroup{
						amazon.ItemSearchResponseGroupItemAttributes,
						amazon.ItemSearchResponseGroupItemIds,
						amazon.ItemSearchResponseGroupOfferSummary,
						amazon.ItemSearchResponseGroupOffers,
						amazon.ItemSearchResponseGroupOfferListings,
					},
					ItemPage:     itemPage,
					MinimumPrice: minPrice,
					MaximumPrice: minPrice + *priceIncrement,
				}).Do()
				if err != nil {
					time.Sleep(5 * time.Second)
					if strings.Contains(err.Error(), "AWS.ECommerceService.NoExactMatches") {
						fmt.Printf("Total Results: %d, Total Pages: %d\n\n", 0, 0)
						break
					}
					printError(err)
					continue
				}

				fmt.Printf("Total Results: %d, Total Pages: %d\n\n",
					res.Items.TotalResults, res.Items.TotalPages)

				for _, item := range res.Items.Item {
					if exit {
						break
					}

					currItem++

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
		}
	}

	w.Flush()
	(*csvFile).Close()
	log.Println("DONE!")
}

type legoItem struct {
	ASIN               string
	Title              string
	IsEligibleForPrime bool
	listPrice          string
	ListPrice          float64
	lowestPrice        string
	LowestPrice        float64
	primePrice         string
	PrimePrice         float64
	parts              string
	Parts              int
	weight             string
	WeightGrams        int
	PricePerPart       float64
	PricePerGram       float64
	URL                string
}

func newLegoItem(item *amazon.Item, bow *browser.Browser) *legoItem {
	li := &legoItem{
		ASIN:        item.ASIN,
		Title:       item.ItemAttributes.Title,
		URL:         item.DetailPageURL,
		listPrice:   item.ItemAttributes.ListPrice.Amount,
		lowestPrice: item.OfferSummary.LowestNewPrice.Amount,
	}

	li.ListPrice, _ = strconv.ParseFloat(li.listPrice, 64)
	li.ListPrice = li.ListPrice / float64(100)

	li.LowestPrice, _ = strconv.ParseFloat(li.lowestPrice, 64)
	li.LowestPrice = li.LowestPrice / float64(100)

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

	for _, offer := range item.Offers.Offer {
		if offer.OfferListing.IsEligibleForPrime {
			li.IsEligibleForPrime = true
			li.primePrice = offer.OfferListing.Price.Amount
			f, _ := strconv.Atoi(li.primePrice)
			li.PrimePrice = float64(f) / 100.0
			break
		}
	}

	lowest := getLowest(li.ListPrice, li.PrimePrice, li.LowestPrice)

	if li.Parts > 0 {
		li.PricePerPart = lowest / float64(li.Parts)
	}
	if li.WeightGrams > 0 {
		li.PricePerGram = lowest / float64(li.WeightGrams)
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

	return li
}

func getLowest(prices ...float64) float64 {
	sort.Sort(sort.Reverse(sort.Float64Slice(prices)))
	return prices[0]
}

func (li *legoItem) Headers() []string {
	rec := []string{}
	rec = append(rec, "ASIN")
	rec = append(rec, "Title")
	rec = append(rec, "Prime")
	rec = append(rec, "List Price")
	rec = append(rec, "Prime Price")
	rec = append(rec, "Lowest Price")
	rec = append(rec, "Parts")
	rec = append(rec, "Weight (g)")
	rec = append(rec, "Price/Part")
	rec = append(rec, "Price/Gram")
	rec = append(rec, "URL")
	return rec
}

func (li *legoItem) Columns() []string {
	rec := []string{}
	rec = append(rec, li.ASIN)
	rec = append(rec, li.Title)
	rec = append(rec, fmt.Sprintf("%v", li.IsEligibleForPrime))
	rec = append(rec, fmt.Sprintf("%.02f", li.ListPrice))
	rec = append(rec, fmt.Sprintf("%.02f", li.PrimePrice))
	rec = append(rec, fmt.Sprintf("%.02f", li.LowestPrice))
	rec = append(rec, fmt.Sprintf("%d", li.Parts))
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
