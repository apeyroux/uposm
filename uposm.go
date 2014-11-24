package main

import (
	"compress/gzip"
	"encoding/xml"
	"flag"
	"fmt"
	"github.com/j4/gosm"
	//"gopkg.in/yaml.v2"
	"github.com/go-yaml/yaml"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

var (
	flosmfile = flag.String("f", "", "OSM File")
)

type State struct {
	LastRun  string `yaml:"last_run"` // todo:mettre un time
	Sequence string `yaml:"sequence"`
}

type ChangeSet struct {
	Id        int64     `xml:"id,attr"`
	CreatedAt time.Time `xml:"created_at,attr"`
	Open      bool      `xml:"open,attr"`
	User      string    `xml:"user,attr"`
	MinLat    float64   `xml:"min_lat,attr"`
	MaxLat    float64   `xml:"max_lat,attr"`
	MinLon    float64   `xml:"min_lon,attr"`
	MaxLon    float64   `xml:"max_lon,attr"`
}

type OsmFile struct {
	ChangeSets []ChangeSet `xml:"changeset"`
}

func getDiff(url string) []byte {
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("I can't dwl diff ...")
	}
	defer resp.Body.Close()
	if err != nil {
		log.Fatalf("I can't read diff body ...")
	}
	reader, _ := gzip.NewReader(resp.Body)
	defer reader.Close()
	body, err := ioutil.ReadAll(reader)
	if err != nil {
		log.Fatal("Oops i can't read diff ...")
	}
	return body
}

func getDiffUrl() string {
	resp, err := http.Get("http://planet.openstreetmap.org/replication/changesets/state.yaml")
	if err != nil {
		log.Fatalf("I can't dwl diff ...")
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("I can't read diff body ...")
	}
	state := State{}
	err = yaml.Unmarshal(body, &state)
	if err != nil {
		log.Fatalf("I can't decode yaml (%s)...", err)
	}
	url := fmt.Sprintf("http://planet.openstreetmap.org/replication/changesets/00%c/%s/%s.osm.gz", state.Sequence[0], state.Sequence[1:4], state.Sequence[4:])

	return url
}

func parseDiff(buffer []byte) {
	osm := OsmFile{}
	err := xml.Unmarshal(buffer, &osm)
	if err != nil {
		fmt.Errorf("Oops : %s", err)
	}
	for _, changeset := range osm.ChangeSets {
		if changeset.MaxLat != 0 {
			fmt.Printf("change %d by %s %d %s %d\nminlong %f maxlon %f minlat %f maxlat %f\n", changeset.Id, changeset.User, changeset.CreatedAt.Day(), changeset.CreatedAt.Month().String(), changeset.CreatedAt.Year(), changeset.MinLon, changeset.MaxLon, changeset.MinLat, changeset.MaxLat)

			for _, z := range []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15} {
				tMax := gosm.NewTileWithLatLong(changeset.MaxLat, changeset.MaxLon, z)
				tMin := gosm.NewTileWithLatLong(changeset.MinLat, changeset.MinLon, z)

				nbtiles := math.Abs((float64(tMax.X))-float64(tMin.X)) + math.Abs(float64(tMax.Y)-float64(tMin.Y))
				fmt.Printf("z:%d download %d tiles ...\n", tMax.Z, int64(nbtiles))

				for x := tMin.X; x <= tMax.X; x++ {
					for y := tMax.Y; y <= tMin.Y; y++ {
						fmt.Printf("\t up a.tile.openstreetmap.org/%d/%d/%d.png\n", z, x, y)
					}
				}
			}
			fmt.Printf("---\n")
		}
	}
}

func getTileFromOSM(t *gosm.Tile) ([]byte, error) {
	// utiliser le proxy http si il est config
	url := fmt.Sprintf("http://a.tile.openstreetmap.org/%d/%d/%d.png", t.Z, t.X, t.Y)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("Trouve pas la tuile sur WWW")
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Je ne comprend pas très bien le contenu de cette tuile")
	}

	log.Printf("hitwww z:%d x:%d y:%d from %s", t.Z, t.X, t.Y, url)

	return body, nil
}

func getsrvosm() string {
	srvs := []string{"a.tile.openstreetmap.org"}
	return srvs[rand.Intn(len(srvs))]
}

func mainhandler(w http.ResponseWriter, r *http.Request) {
	//began := time.Now()
	path := r.URL.Path[1:]
	re := regexp.MustCompile("^([0-9]+)/([0-9]+)/([0-9]+).png$")
	tilecoord := re.FindStringSubmatch(path)
	t := new(gosm.Tile)
	if tilecoord != nil {
		// TODO: mettre du test sur les err de strconv
		z, _ := strconv.Atoi(tilecoord[1])
		x, _ := strconv.Atoi(tilecoord[2])
		y, _ := strconv.Atoi(tilecoord[3])

		t.Z = z
		t.X = x
		t.Y = y
	}
	tbin, err := getTileFromOSM(t)
	if err != nil {
		log.Fatal(err)
	}
	w.Write(tbin)

}

func main() {
	flag.Parse()

	go func() {
		for {
			parseDiff(getDiff(getDiffUrl()))
			time.Sleep(1 * time.Minute)
		}
	}()

	http.HandleFunc("/", mainhandler)
	http.ListenAndServe(":8080", nil)
}
