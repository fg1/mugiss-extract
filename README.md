µGISS-Extract: OpenStreetMap cities extractor
=============================================

`mugiss_extract` is a tool for extracting cities boundaries from [OpenStreetMap PBF files](https://wiki.openstreetmap.org/wiki/PBF_Format) and exporting them in a CSV file.
City extraction is based on the [`boundary`](https://wiki.openstreetmap.org/wiki/Tag:boundary=administrative) and [`admin_level`](https://wiki.openstreetmap.org/wiki/Key:admin_level) tags in OpenStreetMap.


## Installation

On Debian/Ubuntu:
```
# apt-get install libgeos-dev
$ git clone https://github.com/fg1/mugiss-extract.git
$ cd mugiss-extract
$ go get
$ go build
```


## Usage

```
Usage: mugiss_extract [global options] 

Global options:
        -i,     Input OpenStreetMap PBF data file (*)
        -l,     Admin level (default: 8)
        -m,     Minimal admin level (default: 6)
        -o,     Output CSV data file (*)
```


## CSV file format

The CSV file format follows the [gisgraphy cities file format](http://download.gisgraphy.com/format.txt).
The columns are:
```
1 :  Node type;   N|W|R (in uppercase), wheter it is a Node, Way or Relation in the openstreetmap Model
2 :  id;  The openstreetmap id
3 :  name;    the default name of the city
4 :  countrycode; The iso3166-2 country code (2 letters)
5 :  postcode;    The postcode / zipcode / ons code / municipality code / ...
6 :  population;  How many people lives in that city
7 :  location;    The middle location of the city in HEXEWKB
8 :  shape; The delimitation of the city in HEXEWKB
9 :  type; the type of city ('city', 'village', 'town', 'hamlet', ...)
10 : is_in ; where the cities is located (generally the fully qualified administrative division)
11 : alternatenames;     the names of the city in other languages
```

## Contributing

Contributions are welcome. Have a look at the TODO list above, or come with your own features.

1. [Fork the repository](https://github.com/fg1/mugiss-extract/fork)
2. Create your feature branch (`git checkout -b my-feature`)
3. Format your changes (`go fmt`) and commit it (`git commit -am 'Commit message'`)
4. Push to the branch (`git push origin my-feature`)
5. Create a pull request
