package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
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
	rgxXeon                 = regexp.MustCompile("(Xeon).*([0-9]{4})")
	rgxEpyc                 = regexp.MustCompile("(EPYC).*([0-9][A-Z][0-9]{2})")
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
	// ParseIP returns nil for an invalid IP address.  The bold assumption is that an invalid IP is a hostname.
	if net.ParseIP(hostName) == nil {
		return strings.Split(hostName, ".")[0]
	}
	return hostName
}

func cpuInfo(summary string) (family, model string) {
	isXeon := rgxXeon.FindStringSubmatch(summary)
	if len(isXeon) == 3 {
		return isXeon[1], isXeon[2]
	}
	isEpyc := rgxEpyc.FindStringSubmatch(summary)
	if len(isEpyc) == 3 {
		return isEpyc[1], isEpyc[2]
	}
	return "Unk ", "Unk "
}

// importJSONFromFile simply imports some JSON from a file
func importJSONFromFile(filename string) (gjson.Result, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("Cannot import json from: %s", filename)
	}
	return gjson.ParseBytes(b), err
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

func readAPI(apiUrl string) (gjson.Result, error) {
	omeAPI := api.NewBasicAuthClient(cfg.API.Username, cfg.API.Password, cfg.API.CertFile)
	fullUrl, err := url.JoinPath(cfg.API.URL, apiUrl)
	if err != nil {
		log.Errorf("Unable to construct URL from: %s and %s", cfg.API.URL, apiUrl)
		return gjson.Result{}, err
	}
	bytes, err := omeAPI.GetJSON(fullUrl)
	if err != nil {
		log.Fatalf("Cannot read URL: %s", fullUrl)
		return gjson.Result{}, err
	}
	gj := gjson.ParseBytes(bytes)
	return gj, err
}

func apiMembers(gj gjson.Result) {
	var hostName string
	for n, gjn := range gj.Get("value").Array() {
		if !gjn.Get("SKU").Exists() {
			log.Warnf("No SKU found for item: %d", n)
			continue
		}
		sKU := gjn.Get("SKU").Str
		if !gjn.Get("Name").Exists() {
			log.Warnf("No hostname defined for SKU: %s", sKU)
		}
		hostName = strings.ToLower(gjn.Get("Name").String())
		if !cfg.Output.FQDN {
			hostName = shortName(hostName)
		}
		cpuFamily, cpuModel := cpuInfo(gjn.Get("ProcessorSummary.Model").String())
		fmt.Printf("%s %-20s %-30s %d %s %s %d\n",
			sKU,
			hostName,
			gjn.Get("Model").Str,
			gjn.Get("ProcessorSummary.Count").Int(),
			cpuFamily, cpuModel,
			gjn.Get("MemorySummary.TotalSystemMemoryGiB").Int(),
		)
	}
}

func apiWarranty(gj gjson.Result) {
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

	var gj gjson.Result
	if flags.Warranty {
		gj, err = readAPI("api/WarrantyService/Warranties")
		if err != nil {
			log.Fatalf("API read Warrantyfailed with: %v", err)
		}
		apiWarranty(gj)
	} else {
		gj, err := importJSONFromFile("systems.json")
		//gj, err := readAPI("redfish/v1/Systems/Members")
		if err != nil {
			log.Fatalf("API read Members failed with: %v", err)
		}
		apiMembers(gj)
	}
}
