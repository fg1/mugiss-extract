package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"sort"
	"strconv"

	"github.com/paulsmith/gogeos/geos"
	"github.com/qedus/osmpbf"
	"gopkg.in/cheggaaa/pb.v1"
)

type Area struct {
	*osmpbf.Relation
	Geom    *geos.Geometry
	Center  *geos.Geometry
	Ignored bool
}

// Transforms a LineString to a Polygon
func LineToPolygon(g *geos.Geometry) (*geos.Geometry, error) {
	if d, err := geos.Must(g.StartPoint()).Distance(geos.Must(g.EndPoint())); err != nil {
		log.Fatal(err)
	} else if d != 0 {
		return nil, fmt.Errorf("Way collection do not form a ring")
	}
	return geos.NewPolygon(geos.MustCoords(g.Coords()))
}

// Merges the different LineStrings into one Polygon or a MultiPolygon
func MergeLines(lines []*geos.Geometry) (*geos.Geometry, error) {
	if len(lines) == 0 {
		return nil, nil
	}

	g := geos.Must(geos.NewCollection(geos.MULTILINESTRING, lines...))
	g, err := g.LineMerge()
	if err != nil {
		log.Fatal(err)
	}

	n, err := g.NGeometry()
	if err != nil {
		return nil, err
	}
	if n == 1 {
		return LineToPolygon(g)
	}

	polys := make([]*geos.Geometry, n)
	for i := 0; i < n; i++ {
		polys[i], err = LineToPolygon(geos.Must(g.Geometry(i)))
		if err != nil {
			return nil, err
		}
	}
	return geos.NewCollection(geos.MULTIPOLYGON, polys...)
}

// Returns a new osmpbf.Decoder along a progress bar following the decoding process
func GetNewOsmpbfDecoder(r io.Reader, fsize int64) (*osmpbf.Decoder, *pb.ProgressBar, error) {
	bar := pb.New(int(fsize)).SetUnits(pb.U_BYTES)
	bar.Start()

	d := osmpbf.NewDecoder(bar.NewProxyReader(r))
	d.SetBufferSize(osmpbf.MaxBlobSize)
	err := d.Start(runtime.GOMAXPROCS(-1))
	if err != nil {
		bar.Finish()
		return nil, nil, err
	}
	return d, bar, nil
}

// Reads an OpenStreetMap PBF file and extracts the areas corresponding to the given admin level
func ExtractCitiesFromOsmpbf(fname string, tgt_admin_level int, min_admin_level int) (map[int64]*Area, error) {
	log.Println("Opening", fname, "...")

	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	fsize := fi.Size()

	// -------------------------------------------------------------------
	// First pass: reading relations
	d, bar, err := GetNewOsmpbfDecoder(f, fsize)
	if err != nil {
		return nil, err
	}
	nodes := make(map[int64]*osmpbf.Node)
	ways := make(map[int64]*osmpbf.Way)
	areas := make(map[int64]*Area)
	var nc, wc, rc uint64
	for {
		if v, err := d.Decode(); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		} else {
			switch v := v.(type) {
			case *osmpbf.Relation:
				if !v.Info.Visible {
					continue
				}
				if _, ok := v.Tags["name"]; !ok {
					continue
				}
				if _, ok := v.Tags["admin_level"]; !ok {
					continue
				}
				if v.Tags["type"] != "boundary" {
					continue
				}
				areas[v.ID] = &Area{v, nil, nil, true}

				for _, m := range v.Members {
					switch m.Type {
					case osmpbf.WayType:
						ways[m.ID] = nil
					case osmpbf.NodeType:
						nodes[m.ID] = nil
					}
				}
				rc++
			}
		}
	}
	bar.Finish()
	log.Println("Parsed", rc, "relations")

	// -------------------------------------------------------------------
	// Second pass: reading relevant ways
	f.Seek(0, 0)
	d, bar, err = GetNewOsmpbfDecoder(f, fsize)
	if err != nil {
		return nil, err
	}
	for {
		if v, err := d.Decode(); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		} else {
			switch v := v.(type) {
			case *osmpbf.Way:
				if w, ok := ways[v.ID]; ok && (w == nil) {
					ways[v.ID] = v
					for _, n := range v.NodeIDs {
						nodes[n] = nil
					}
					wc++
				}
			}
		}
	}
	bar.Finish()
	log.Println("Parsed", wc, "ways")

	// -------------------------------------------------------------------
	// Third pass: reading relevant nodes
	f.Seek(0, 0)
	d, bar, err = GetNewOsmpbfDecoder(f, fsize)
	if err != nil {
		return nil, err
	}
	for {
		if v, err := d.Decode(); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		} else {
			switch v := v.(type) {
			case *osmpbf.Node:
				if n, ok := nodes[v.ID]; ok && (n == nil) {
					nodes[v.ID] = v
					nc++
				}
			}
		}
	}
	bar.Finish()
	log.Println("Parsed", nc, "nodes")

	// -------------------------------------------------------------------
	// Build the geometries of the different areas
	admlvls := make(map[int]int)
	areas_admlvl := make(map[int][]*Area)
	bar = pb.StartNew(len(areas))
	for k, a := range areas {
		bar.Increment()
		inner_lines := []*geos.Geometry{}
		outer_lines := []*geos.Geometry{}
		for _, m := range a.Members {
			switch m.Type {
			case osmpbf.NodeType:
				node, ok := nodes[m.ID]
				if !ok || (node == nil) {
					break
				}
				if (m.Role == "label") || (m.Role == "admin_center") {
					if a.Center == nil {
						a.Center = geos.Must(geos.NewPoint(geos.NewCoord(node.Lon, node.Lat)))
					}
					for k, v := range node.Tags {
						a.Tags[k] = v
					}
				}

			case osmpbf.WayType:
				way, ok := ways[m.ID]
				if !ok || (way == nil) {
					break
				}
				line := make([]geos.Coord, 0, len(way.NodeIDs))
				for _, n := range way.NodeIDs {
					node, ok := nodes[n]
					if !ok || (node == nil) {
						log.Println("WARN: Node", m.ID, "not found!")
						break
					}
					line = append(line, geos.NewCoord(node.Lon, node.Lat))
				}
				geom, err := geos.NewLineString(line...)
				if err != nil {
					return nil, err
				}
				switch m.Role {
				case "outer":
					outer_lines = append(outer_lines, geom)
				case "inner":
					inner_lines = append(inner_lines, geom)
				default:
					log.Println("WARN: In relation", a.ID, ": way", way.ID, "with unknown role:", m.Role)
				}
			}
		}

		g, err := MergeLines(outer_lines)
		if (err != nil) || (g == nil) {
			delete(areas, k)
			continue
		}
		if len(inner_lines) > 0 {
			gi, err := MergeLines(inner_lines)
			if (err != nil) || (gi == nil) {
				delete(areas, k)
				continue
			}
			g = geos.Must(g.Difference(gi))
		}
		a.Geom = g

		admlvl, err := strconv.Atoi(a.Tags["admin_level"])
		if err != nil {
			return nil, err
		}
		areas_admlvl[admlvl] = append(areas_admlvl[admlvl], a)
		admlvls[admlvl]++
	}
	bar.Finish()
	log.Println("Found", len(areas), "areas")

	// -------------------------------------------------------------------
	// Filter relevant areas by admin levels. The heuristic used is as
	// follows. The main target admin level will first be used, the other
	// ones. If it has already been mapped, it is discarded. Gradually
	// builds a polygon for recording which area has been mapped.
	blen := 0
	sorted_admlvls := []int{-1}
	for k, v := range admlvls {
		if k < min_admin_level {
			continue
		}
		blen += v
		if k != tgt_admin_level {
			sorted_admlvls = append(sorted_admlvls, k)
		}
	}
	sort.Ints(sorted_admlvls)
	sorted_admlvls[0] = tgt_admin_level

	var ncities uint64
	fullpoly := geos.Must(geos.EmptyPolygon())
	bar = pb.StartNew(blen)
	for _, admlvl := range sorted_admlvls {
		for _, a := range areas_admlvl[admlvl] {
			bar.Increment()
			if within, err := a.Geom.CoveredBy(fullpoly); err != nil {
				return nil, err
			} else if within {
				continue
			}

			fullpoly = geos.Must(fullpoly.Union(a.Geom))
			a.Ignored = false
			ncities++
		}
	}
	bar.Finish()
	log.Println("Filtered", ncities, "cities")

	return areas, nil
}
