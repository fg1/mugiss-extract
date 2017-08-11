package main

import (
	"encoding/csv"
	"log"
	"os"
	"strconv"

	"github.com/voxelbrain/goptions"
)

func main() {
	options := struct {
		InputFile     string `goptions:"-i, obligatory, description='Input OpenStreetMap PBF data file'"`
		AdminLevel    int    `goptions:"-l, description='Admin level'"`
		MinAdminLevel int    `goptions:"-m, description='Minimal admin level'"`
		OutputFile    string `goptions:"-o, obligatory, description='Output CSV data file'"`
	}{
		AdminLevel:    8,
		MinAdminLevel: 6,
	}
	goptions.ParseAndFail(&options)

	areas, err := ExtractCitiesFromOsmpbf(options.InputFile, options.AdminLevel, options.MinAdminLevel)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Exporting OSM data to", options.OutputFile)
	fcsv, err := os.Create(options.OutputFile)
	if err != nil {
		log.Fatal(err)
	}
	defer fcsv.Close()

	w := csv.NewWriter(fcsv)
	w.Comma = '\t'
	defer w.Flush()

	for _, a := range areas {
		if a.Ignored {
			continue
		}

		center := []byte{}
		if a.Center != nil {
			center, _ = a.Center.Hex()
		}
		shape, _ := a.Geom.Hex()
		row := []string{
			"R", // 0 :  Node type;   N|W|R (in uppercase), wheter it is a Node, Way or Relation in the openstreetmap Model
			strconv.FormatInt(a.ID, 10), // 1 :  id;  The openstreetmap id
			a.Tags["name"],              // 2 :  name;    the default name of the city
			"",                          // 3 :  countrycode; The iso3166-2 country code (2 letters)
			a.Tags["addr:postcode"], // 4 :  postcode;    The postcode / zipcode / ons code / municipality code / ...
			a.Tags["population"],    // 5 :  population;  How many people lives in that city
			string(center),          // 6 :  location;    The middle location of the city in HEXEWKB
			string(shape),           // 7 :  shape; The delimitation of the city in HEXEWKB
			a.Tags["place"],         // 8 :  type; the type of city ('city', 'village', 'town', 'hamlet', ...)
			a.Tags["is_in"],         // 9 :  is_in ; where the cities is located (generally the fully qualified administrative division)
			"",                      // 10 : alternatenames;     the names of the city in other languages
		}

		if err := w.Write(row); err != nil {
			log.Fatal(err)
		}
	}
}
