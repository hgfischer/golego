package main

import (
	"context"
	"encoding/csv"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"

	"time"

	"io"

	"github.com/ngs/go-amazon-product-advertising-api/amazon"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	appName         = path.Base(os.Args[0])
	app             = kingpin.New(appName, "Go LEGO - special searches on Amazon.de")
	accessKeyID     = app.Flag("access-key-id", "AWS Access Key").Required().String()
	secretAccessKey = app.Flag("secret-access-key", "AWS Secret Key").Required().String()
	associateTag    = app.Flag("associate-tag", "Amazon Associate Tag").Required().String()
	amazonRegion    = app.Flag("region", "Amazon store region").Default(string(amazon.RegionGermany)).
			Enum(string(amazon.RegionBrazil), string(amazon.RegionCanada), string(amazon.RegionChina),
			string(amazon.RegionGermany), string(amazon.RegionSpain), string(amazon.RegionFrance),
			string(amazon.RegionIndia), string(amazon.RegionItaly), string(amazon.RegionJapan),
			string(amazon.RegionMexico), string(amazon.RegionUK), string(amazon.RegionUS))
	minYear       = app.Flag("min-year", "Ignore sets older than this").Default("2000").Int()
	minParts      = app.Flag("min-parts", "Ignore sets with less parts than this").Default("50").Int()
	sleepDuration = app.Flag("sleep", "Secs between api calls").Default("1").Duration()
	csvFile       = app.Arg("CSV", "Name of the CSV file to write").Required().OpenFile(
		os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
)

func main() {
	kingpin.MustParse(app.Parse(os.Args[1:]))
	ctx, cancel := context.WithCancel(context.Background())
	exitCh := make(chan struct{})

	go func(ctx context.Context) {
		resp, err := http.Get("https://m.rebrickable.com/media/downloads/sets.csv")
		fatalize(err)
		csvReader := csv.NewReader(resp.Body)
		csvSets, err := csvReader.ReadAll()
		fatalize(err)
		sets := map[string]*legoSet{}

		csvWriter := csv.NewWriter(io.MultiWriter(*csvFile, os.Stderr))
		csvWriter.Write((&legoSet{}).Columns())
		defer csvWriter.Flush()
		defer (*csvFile).Close()

		for _, csvSet := range csvSets[1:] {
			select {
			case <-ctx.Done():
				time.Sleep(*sleepDuration)
				exitCh <- struct{}{}
				return
			default:
				set := newLegoSet(csvSet[0], csvSet[1], csvSet[2], csvSet[3])
				if set.Year < *minYear && set.Parts < *minParts {
					continue
				}
				if item := search(*accessKeyID, *secretAccessKey, *associateTag, *amazonRegion, set.Num); item != nil {
					set.Fill(item)
					sets[set.Num] = set
					csvWriter.Write(set.Columns())
					csvWriter.Flush()
					time.Sleep(*sleepDuration)
				}
			}
		}

	}(ctx)

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt)
	go func() {
		select {
		case <-signalCh:
			cancel()
			return
		}
	}()
	<-exitCh
	log.Println("DONE!")
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

func search(accessKeyID, secretAccessKey, associateTag, region, setNum string) *amazon.Item {
	client, err := amazon.New(accessKeyID, secretAccessKey, associateTag, amazon.Region(region))
	fatalize(err)
	res, err := client.ItemSearch(
		amazon.ItemSearchParameters{
			OnlyAvailable: true,
			Condition:     amazon.ConditionNew,
			SearchIndex:   amazon.SearchIndexToys,
			Keywords:      "LEGO " + setNum,
			ResponseGroups: []amazon.ItemSearchResponseGroup{
				amazon.ItemSearchResponseGroupItemAttributes,
				amazon.ItemSearchResponseGroupItemIds,
				amazon.ItemSearchResponseGroupOfferSummary,
				amazon.ItemSearchResponseGroupOffers,
				amazon.ItemSearchResponseGroupOfferListings,
			},
		}).Do()
	if err != nil {
		return nil
	}
	if res.Items.TotalResults > 1 {
		log.Printf("Found %d results for `LEGO %s`", res.Items.TotalResults, setNum)
	}
	return &(res.Items.Item[0])
}
