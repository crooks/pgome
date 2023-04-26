package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Masterminds/log-go"
	"github.com/crooks/jlog"
	loglevel "github.com/crooks/log-go-level"
	"github.com/crooks/pgome/api"
	"github.com/crooks/pgome/config"
	"github.com/tidwall/gjson"
)

const (
	omeDateFmt    = "2006-01-02 15:04:05.000"
	outputDateFmt = "2006-01-02"
)

var (
	cfg               *config.Config
	flags             *config.Flags
	errObjectNotFound error = errors.New("gjson: object not found")
)

type unitJSON struct {
	DeviceIdentifier string
	DeviceName       string
	DeviceModel      string
	StartDate        int64
	EndDate          int64
	Description      string
}

func shortName(hostName string) string {
	return strings.SplitN(hostName, ".", 1)[0]
}

// importJSONFromFile simply imports some JSON from a file
func importJSONFromFile(filename string) gjson.Result {
	b, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("Cannot import json: %v", err)
	}
	return gjson.ParseBytes(b)
}

// gjDateParse returns an Epoch Integer representation of a JSON date string in OME format
func gjDateParse(gjDateItem gjson.Result) (int64, error) {
	if !gjDateItem.Exists() {
		return 0, errObjectNotFound
	}
	return omeDateParse(gjDateItem.Str)
}

// omeDateParse take a date string in OME format and returns it as an Epoch integer
func omeDateParse(omeDate string) (int64, error) {
	d, err := time.Parse(omeDateFmt, omeDate)
	if err != nil {
		log.Warn("omeDateParse failed for %s", omeDate)
		return 0, err
	}
	return d.Unix(), nil
}

func apiMembers(omeAPI *api.AuthClient) {
	urlMembers, err := url.JoinPath(cfg.API.URL, "redfish/v1/Systems/Members")
	if err != nil {
		log.Fatalf("Unable to parse Systems/Members URL: %v", err)
	}
	bytes, err := omeAPI.GetJSON(urlMembers)
	if err != nil {
		log.Fatalf("Cannot read %s: %v", urlMembers, err)
	}
	gj := gjson.ParseBytes(bytes)
	for n, gjn := range gj.Get("value").Array() {
		if !gjn.Get("SKU").Exists() {
			log.Warnf("No SKU found for item: %d", n)
			continue
		}
		sKU := gjn.Get("SKU").Str
		if !gjn.Get("Name").Exists() {
			log.Warnf("No hostname defined for SKU: %s", sKU)
		}
		hostName := strings.ToLower(gjn.Get("Name").Str)
		if !cfg.Output.FQDN {
			hostName = shortName(hostName)
		}
		fmt.Printf("%s %s %s %d %d\n",
			sKU,
			hostName,
			gjn.Get("Model").Str,
			gjn.Get("ProcessorSummary.Count").Int(),
			gjn.Get("MemorySummary.TotalSystemMemoryGiB").Int(),
		)
	}
}

func apiWarranty(omeAPI *api.AuthClient) {
	urlWarranty, err := url.JoinPath(cfg.API.URL, "api/WarrantyService/Warranties")
	if err != nil {
		log.Fatalf("Unable to parse warranty URL: %v", err)
	}
	bytes, err := omeAPI.GetJSON(urlWarranty)
	if err != nil {
		log.Fatalf("Cannot read %s: %v", urlWarranty, err)
	}
	gj := gjson.ParseBytes(bytes)
	var jout []*unitJSON
	for n, gjn := range gj.Get("value").Array() {
		gjIdent := gjn.Get("DeviceIdentifier")
		if !gjIdent.Exists() {
			log.Warnf("Warranty item %d has no Device Identifier", n)
			continue
		}
		j := new(unitJSON)
		j.DeviceIdentifier = gjIdent.Str
		j.DeviceName = strings.ToLower(gjn.Get("DeviceName").Str)
		j.DeviceModel = gjn.Get("DeviceModel").Str
		j.Description = gjn.Get("ServiceLevelDescription").Str
		startDate, err := gjDateParse(gjn.Get("StartDate"))
		if err != nil {
			log.Warnf("Cannot parse JSON StartDate: %v", err)
			continue
		}
		j.StartDate = startDate
		endDate, err := gjDateParse(gjn.Get("EndDate"))
		if err != nil {
			log.Warnf("Cannot parse JSON EndDate: %v", err)
			continue
		}
		j.EndDate = endDate
		jout = append(jout, j)
	}
	data, err := json.Marshal(jout)
	if err != nil {
		log.Fatalf("Unable to marshall output JSON: %v", err)
	}
	os.WriteFile(cfg.Output.Filename, data, 0644)
}

func main() {
	var err error
	flags = config.ParseFlags()
	cfg, err = config.ParseConfig(flags.Config)
	if err != nil {
		log.Fatalf("Unable to parse config file: %v", err)
	}

	// Define logging level and method
	loglev, err := loglevel.ParseLevel(cfg.Logging.LevelStr)
	if err != nil {
		log.Fatalf("unable to set log level: %v", err)
	}
	if cfg.Logging.Journal && jlog.Enabled() {
		log.Current = jlog.NewJournal(loglev)
	} else {
		log.Current = log.StdLogger{Level: loglev}
	}

	omeAPI := api.NewBasicAuthClient(cfg.API.Username, cfg.API.Password, cfg.API.CertFile)
	if flags.Warranty {
		apiWarranty(omeAPI)
	} else {
		apiMembers(omeAPI)
	}
}
