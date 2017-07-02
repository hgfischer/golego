package main

import (
	"fmt"
	"strconv"

	"github.com/ngs/go-amazon-product-advertising-api/amazon"
)

type legoSet struct {
	Num   string
	Name  string
	Year  int
	Parts int

	ASIN          string
	AmazonTitle   string
	PrimeEligible bool
	ListPrice     float64
	LowestPrice   float64
	PrimePrice    float64
	PricePerPart  float64
	AmazonURL     string
}

func newLegoSet(num, name, year, parts string) *legoSet {
	nYear, err := strconv.Atoi(year)
	fatalize(err)
	nParts, err := strconv.Atoi(parts)
	fatalize(err)
	set := &legoSet{
		Num:   num,
		Name:  name,
		Year:  nYear,
		Parts: nParts,
	}
	return set
}

func (ls *legoSet) Fill(item *amazon.Item) {
	ls.ASIN = item.ASIN
	ls.AmazonTitle = item.ItemAttributes.Title
	ls.AmazonURL = item.DetailPageURL

	listPrice, err := strconv.ParseFloat(item.ItemAttributes.ListPrice.Amount, 64)
	fatalize(err)
	ls.ListPrice = listPrice / 100.0

	lowestPrice, err := strconv.ParseFloat(item.OfferSummary.LowestNewPrice.Amount, 64)
	fatalize(err)
	ls.LowestPrice = lowestPrice / 100.0

	for _, offer := range item.Offers.Offer {
		if offer.OfferListing.IsEligibleForPrime {
			ls.PrimeEligible = true
			primePrice, err := strconv.ParseFloat(offer.OfferListing.Price.Amount, 64)
			fatalize(err)
			ls.PrimePrice = primePrice / 100.0
			break
		}
	}

	ls.PricePerPart = ls.LowestPrice / float64(ls.Parts)
}

func (ls *legoSet) Headers() []string {
	rec := []string{}
	rec = append(rec, "Num")
	rec = append(rec, "Name")
	rec = append(rec, "ASIN")
	rec = append(rec, "Amazon Title")
	rec = append(rec, "Year")
	rec = append(rec, "Lowest Price")
	rec = append(rec, "List Price")
	rec = append(rec, "Prime Eligible")
	rec = append(rec, "Prime Price")
	rec = append(rec, "Parts")
	rec = append(rec, "Price/Part")
	rec = append(rec, "URL")
	return rec
}

func (ls *legoSet) Columns() []string {
	rec := []string{}
	rec = append(rec, ls.Num)
	rec = append(rec, ls.Name)
	rec = append(rec, ls.ASIN)
	rec = append(rec, ls.AmazonTitle)
	rec = append(rec, fmt.Sprintf("%v", ls.Year))
	rec = append(rec, fmt.Sprintf("%.02f", ls.LowestPrice))
	rec = append(rec, fmt.Sprintf("%.02f", ls.ListPrice))
	rec = append(rec, fmt.Sprintf("%v", ls.PrimeEligible))
	rec = append(rec, fmt.Sprintf("%.02f", ls.PrimePrice))
	rec = append(rec, fmt.Sprintf("%d", ls.Parts))
	rec = append(rec, fmt.Sprintf("%.03f", ls.PricePerPart))
	rec = append(rec, ls.AmazonURL)
	return rec
}
