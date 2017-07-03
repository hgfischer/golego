package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"time"

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
	minYear       = app.Flag("min-year", "Ignore sets older than this").Default("2007").Int()
	minParts      = app.Flag("min-parts", "Ignore sets with less parts than this").Default("50").Int()
	sleepDuration = app.Flag("sleep", "Time duration between API calls").Default("1.5s").Duration()
	csvFile       = app.Arg("CSV", "Name of the CSV file to write").Required().OpenFile(
		os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
)

func main() {
	kingpin.MustParse(app.Parse(os.Args[1:]))
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	ctx, cancel := context.WithCancel(context.Background())
	exitCh := make(chan struct{})

	// for debugging...
	// item := search(*accessKeyID, *secretAccessKey, *associateTag, *amazonRegion, *sleepDuration,
	// 	"41101-1", "Heartlake Grand Hotel")
	// fmt.Printf("%#v", item)
	// os.Exit(1)

	go func(ctx context.Context) {
		resp, err := http.Get("https://m.rebrickable.com/media/downloads/sets.csv")
		if err != nil {
			log.Fatal(err)
		}
		csvReader := csv.NewReader(resp.Body)
		csvSets, err := csvReader.ReadAll()
		if err != nil {
			log.Fatal(err)
		}
		sets := map[string]*legoSet{}

		csvWriter := csv.NewWriter(io.MultiWriter(os.Stderr, *csvFile))
		csvWriter.Comma = '\t'
		csvWriter.Write((&legoSet{}).Headers())
		defer csvWriter.Flush()
		defer (*csvFile).Close()

		for _, csvSet := range csvSets[1:] {
			select {
			case <-ctx.Done():
				time.Sleep(*sleepDuration)
				exitCh <- struct{}{}
				return
			default:
				set := newLegoSet(csvSet[0], csvSet[1], csvSet[2], csvSet[4])
				if set.Year > *minYear && set.Parts > *minParts {
					item := search(*accessKeyID, *secretAccessKey, *associateTag, *amazonRegion,
						*sleepDuration, set.Num, set.Name)
					if item != nil {
						set.Fill(item)
						sets[set.Num] = set
						csvWriter.Write(set.Columns())
						csvWriter.Flush()
					}

				}
			}
		}
		exitCh <- struct{}{}
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

func amzSearch(accessKeyID, secretAccessKey, associateTag, region string, sleep time.Duration, keywords string) *amazon.Items {
	log.Printf("Searching for `%s`", keywords)
	client, err := amazon.New(accessKeyID, secretAccessKey, associateTag, amazon.Region(region))
	if err != nil {
		log.Fatal(err)
	}
	res, err := client.ItemSearch(
		amazon.ItemSearchParameters{
			OnlyAvailable: true,
			Condition:     amazon.ConditionNew,
			SearchIndex:   amazon.SearchIndexToys,
			Keywords:      keywords,
			ResponseGroups: []amazon.ItemSearchResponseGroup{
				amazon.ItemSearchResponseGroupItemAttributes,
				amazon.ItemSearchResponseGroupItemIds,
				amazon.ItemSearchResponseGroupOfferSummary,
				amazon.ItemSearchResponseGroupOffers,
				amazon.ItemSearchResponseGroupOfferListings,
			},
		}).Do()
	time.Sleep(sleep)
	if err != nil {
		log.Printf("Error while searching: %s", err)
		return nil
	}
	return &res.Items
}

func search(accessKeyID, secretAccessKey, associateTag, region string, sleep time.Duration, setNum, setName string) *amazon.Item {
	setNum = strings.Replace(setNum, "-1", "", -1)
	nameParts := strings.Split(setName, " ")
	max := len(nameParts)
	if max >= 3 {
		max = 3
	}
	setName = strings.Join(nameParts[:max], " ")

	keywords := fmt.Sprintf("LEGO %s", setNum)
	items := amzSearch(accessKeyID, secretAccessKey, associateTag, region, sleep, keywords)
	if items != nil {
		log.Printf("Found %d results for `%s`", items.TotalResults, keywords)
		for _, item := range items.Item {
			if item.ItemAttributes.PartNumber == setNum && strings.Contains(item.ItemAttributes.Manufacturer, "LEGO") {
				return &(items.Item[0])
			}
		}
	}

	keywords = fmt.Sprintf("LEGO %s %s", setNum, setName)
	items = amzSearch(accessKeyID, secretAccessKey, associateTag, region, sleep, keywords)
	if items != nil {
		log.Printf("Found %d results for `%s`", items.TotalResults, keywords)
		for _, item := range items.Item {
			if item.ItemAttributes.PartNumber == setNum && strings.Contains(item.ItemAttributes.Manufacturer, "LEGO") {
				return &(items.Item[0])
			}
		}
	}

	return nil
}
