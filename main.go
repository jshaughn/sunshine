// main.go
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type options struct {
	server     string
	sampleSize int
	offset     time.Duration
	interval   time.Duration
	endpoint   string
}

func parseFlags() options {
	serverDefault, ok := os.LookupEnv("PROMETHEUS_SERVER")
	if !ok {
		serverDefault = "http://localhost:9090"
	}
	server := flag.String("server", serverDefault, "Prometheus server URL (can be set via PROMETHEUS_SERVER environment variable)")
	offset := flag.String("offset", "0m", "Offset (Xm, Xh, or Xd) from now to start metric sample collection.")
	interval := flag.String("interval", "30s", "Query interval (Xs). Recommended 2 times the scrape interval.")
	endpoint := flag.String("endpoint", ":8080", "The scrape endpoint")

	flag.Parse()

	return options{
		server:   *server,
		offset:   durationOption(*offset),
		interval: durationOption(*interval),
		endpoint: *endpoint,
	}
}

func intOption(option string) int {
	val, err := strconv.Atoi(option)
	checkError(err)
	return val
}

func durationOption(option string) time.Duration {
	val, err := time.ParseDuration(option)
	checkError(err)
	return val
}

func validateOptions(options options) error {
	fmt.Printf("Options: %+v\n", options)

	if options.server == "" {
		return errors.New("Server must be set")
	}

	return nil
}

type TSExpression string

var (
	tsRoots = []TSExpression{
		"istio_request_count",
	}
)

type Node struct {
	Name     string
	Version  string
	Parent   *Node
	Children []*Node
}

func (ts TSExpression) buildTree(n *Node, start, end time.Time, api v1.API) {
	match := fmt.Sprintf("%v{source_service=\"%v\",source_version=\"%v\"}", ts, n.Name, n.Version)

	// fetch the root time series
	series := promSeries(match, start, end, api)
	//fmt.Printf("Found [%v] parent series\n", len(series))

	// identify the unique destination services
	destinations := toDestinations(n.Name, n.Version, series)
	//fmt.Printf("Found [%v] child destinations\n", len(destinations))

	if len(destinations) > 0 {
		n.Children = make([]*Node, len(destinations))
		i := 0
		for k, _ := range destinations {
			s := strings.Split(k, " ")
			child := Node{
				Name:    s[0],
				Version: s[1],
				Parent:  n,
			}
			fmt.Printf("Child Service: %v(%v)->%v(%v)\n", n.Name, n.Version, child.Name, child.Version)
			n.Children[i] = &child
			i++
			ts.buildTree(&child, start, end, api)
		}
	}
}

var trees []Node

// process() is expected to execute as a goroutine
func (ts TSExpression) process(o options, wg *sync.WaitGroup, api v1.API) {
	defer wg.Done()

	start := time.Now()
	if o.offset.Seconds() > 0 {
		start = start.Add(-o.offset)
	}
	start = start.Add(-o.interval)

	for {
		end := start.Add(o.interval)
		match := fmt.Sprintf("%v{source_service=\"\"}", ts)

		// fetch the root time series
		series := promSeries(match, start, end, api)
		//fmt.Printf("Found [%v] root series\n", len(series))

		// identify the unique top-level destination services
		destinations := toDestinations("", "", series)
		//fmt.Printf("Found [%v] root destinations\n", len(destinations))

		// generate a tree rooted at each top-level destination
		trees = make([]Node, len(destinations))
		i := 0
		for k, _ := range destinations {
			s := strings.Split(k, " ")
			root := Node{
				Name:    s[0],
				Version: s[1],
				Parent:  nil,
			}
			fmt.Printf("Root Service: %v(%v)\n", root.Name, root.Version)
			ts.buildTree(&root, start, end, api)
			trees[i] = root
			i++
		}

		break
		//time.Sleep(o.interval)
		//start = start.Add(o.interval)
	}
}

// toDestinations takes a slice of [istio] series and returns a map with keys "destSvc destVersion", removing self-invocation
func toDestinations(fromService, fromVersion string, series []model.LabelSet) (destinations map[string]bool) {
	destinations = make(map[string]bool)
	for _, s := range series {
		service, serviceOk := s["destination_service"]
		version, versionOk := s["destination_version"]
		if fromService == string(service) && fromVersion == string(version) {
			continue
		}
		if serviceOk && versionOk {
			k := fmt.Sprintf("%v %v", service, version)
			destinations[k] = true
		}
	}
	return destinations
}

// TF is the TimeFormat for printing timestamp
const TF = "2006-01-02 15:04:05"

func promSeries(match string, start, end time.Time, api v1.API) []model.LabelSet {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Printf("Executing query %s&start=%v&end=%v (now=%v)\n", match, start.Format(TF), end.Format(TF), time.Now().Format(TF))

	value, err := api.Series(ctx, []string{match}, start, end)
	checkError(err)

	return value
}

func checkError(err error) {
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func main() {
	options := parseFlags()
	checkError(validateOptions(options))

	config := api.Config{options.server, nil}
	client, err := api.NewClient(config)
	checkError(err)

	api := v1.NewAPI(client)

	var wg sync.WaitGroup

	for _, ts := range tsRoots {
		wg.Add(1)
		go ts.process(options, &wg, api)
	}

	wg.Wait()
}
