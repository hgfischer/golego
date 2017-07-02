package main

import (
	"encoding/csv"
	"log"
	"os"
	"path"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/headzoo/surf"
	"github.com/headzoo/surf/agent"
	"github.com/ngs/go-amazon-product-advertising-api/amazon"
	"gopkg.in/alecthomas/kingpin.v2"
	pb "gopkg.in/cheggaaa/pb.v1"
)

var (
	appName         = path.Base(os.Args[0])
	app             = kingpin.New(appName, "Go LEGO - special searches on Amazon.de")
	accessKeyID     = app.Flag("access-key-id", "AWS Access Key").Required().String()
	secretAccessKey = app.Flag("secret-access-key", "AWS Secret Key").Required().String()
	associateTag    = app.Flag("associate-tag", "Amazon Associate Tag").Required().String()
	csvFile         = app.Arg("CSV", "Name of the CSV file to write").Required().OpenFile(os.O_TRUNC|os.O_CREATE, 0644)
)

func main() {
	kingpin.MustParse(app.Parse(os.Args[1:]))

	bow := surf.NewBrowser()
	bow.SetUserAgent(agent.Chrome())

	client, err := amazon.New(*accessKeyID, *secretAccessKey, *associateTag, amazon.RegionGermany)
	fatalize(err)

	w := csv.NewWriter(*csvFile)
	w.Write((&legoItem{}).Headers())

	var bar *pb.ProgressBar

	itemPage := 1
	for {
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
		fatalize(err)
		fatalize(res.Error())

		if bar == nil {
			bar = pb.StartNew(res.Items.TotalResults)
		}

		for _, item := range res.Items.Item {
			rec := &legoItem{
				ASIN:        item.ASIN,
				Title:       item.ItemAttributes.Title,
				URL:         item.DetailPageURL,
				ListPrice:   item.ItemAttributes.ListPrice.Amount,
				LowestPrice: item.OfferSummary.LowestNewPrice.Amount,
			}

			err := bow.Open(item.DetailPageURL)
			fatalize(err)

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
					rec.Parts = v
					continue
				}
				if strings.Contains(k, "Artikelgewicht") {
					rec.Weight = v
					continue
				}
			}

			w.Write(rec.Columns())
			bar.Increment()
			w.Flush()
		}

		if res.Items.TotalPages > itemPage {
			itemPage++
		} else {
			break
		}
	}

	bar.FinishPrint("Done!")
}

type legoItem struct {
	ASIN        string
	Title       string
	ListPrice   string
	LowestPrice string
	Parts       string
	Weight      string
	URL         string
}

func (li *legoItem) Headers() []string {
	rec := []string{}
	rec = append(rec, "Title")
	rec = append(rec, "List Price")
	rec = append(rec, "Lowest Price")
	rec = append(rec, "Parts")
	rec = append(rec, "Weight")
	rec = append(rec, "URL")
	return rec
}

func (li *legoItem) Columns() []string {
	rec := []string{}
	rec = append(rec, li.Title)
	rec = append(rec, li.ListPrice)
	rec = append(rec, li.LowestPrice)
	rec = append(rec, li.Parts)
	rec = append(rec, li.Weight)
	rec = append(rec, li.URL)
	return rec
}

func fatalize(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
