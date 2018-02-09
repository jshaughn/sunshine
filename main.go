// main.go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jshaughn/sunshine/cytoscope"
	"github.com/jshaughn/sunshine/tree"
	//"github.com/jshaughn/sunshine/vizceral"

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

func (ts TSExpression) buildTree(n *tree.Tree, start time.Time, o options, api v1.API) {
	query := fmt.Sprintf("sum(rate(%v{source_service=\"%v\",source_version=\"%v\",response_code=~\"%v\"} [%vs]) * 60) by (%v)",
		ts,                                                      // the metric
		n.Name,                                                  // parent service name
		n.Version,                                               // parent service version
		"[2345][0-9][0-9]",                                      // regex for valid response_codes
		o.interval.Seconds(),                                    // rate over the entire query period
		"destination_service,destination_version,response_code") // group by

	// fetch the root time series
	vector := promQuery(query, start, api)

	// identify the unique destination services
	destinations := toDestinations(n.Name, n.Version, vector)

	if len(destinations) > 0 {
		n.Children = make([]*tree.Tree, len(destinations))
		i := 0
		for k, d := range destinations {
			s := strings.Split(k, " ")
			d["link_prom_graph"] = linkPromGraph(o.server, string(ts), s[0], s[1])
			child := tree.Tree{
				Name:     s[0],
				Version:  s[1],
				Parent:   n,
				Metadata: d,
			}
			fmt.Printf("Child Service: %v(%v)->%v(%v)\n", n.Name, n.Version, child.Name, child.Version)
			n.Children[i] = &child
			i++
			ts.buildTree(&child, start, o, api)
		}
	}
}

var trees []tree.Tree

// process() is expected to execute as a goroutine
func (ts TSExpression) process(o options, wg *sync.WaitGroup, api v1.API) {
	defer wg.Done()

	queryTime := time.Now()
	if o.offset.Seconds() > 0 {
		queryTime = queryTime.Add(-o.offset)
	}

	for {
		// query first for root time series (note that this captures source_service "" or "ingress.istio-system"
		query := fmt.Sprintf("sum(rate(%v{source_version=\"unknown\",response_code=~\"%v\"} [%vs]) * 60) by (%v)",
			ts,                                                      // the metric
			"[2345][0-9][0-9]",                                      // regex for valid response_codes
			o.interval.Seconds(),                                    // rate for the entire query period
			"destination_service,destination_version,response_code") // group by

		// fetch the root time series
		vector := promQuery(query, queryTime, api)

		// identify the unique top-level destination services
		destinations := toDestinations("", "", vector)
		fmt.Printf("Found [%v] root destinations\n", len(destinations))

		// generate a tree rooted at each top-level destination
		trees = make([]tree.Tree, len(destinations))
		i := 0
		for k, d := range destinations {
			s := strings.Split(k, " ")
			d["link_prom_graph"] = linkPromGraph(o.server, string(ts), s[0], s[1])
			root := tree.Tree{
				Name:     s[0],
				Version:  s[1],
				Parent:   nil,
				Metadata: d,
			}
			fmt.Printf("Root Service: %v(%v)\n", root.Name, root.Version)
			ts.buildTree(&root, queryTime, o, api)
			trees[i] = root
			i++
		}

		for _, t := range trees {
			//v := vizceral.NewConfig(&t)
			//b, err := json.MarshalIndent(v, "", "  ")
			//checkError(err)
			//fmt.Printf("Vizceral Config:\n%v\n", string(b))

			c := cytoscope.NewConfig(&t)
			b, err := json.MarshalIndent(c, "", "  ")
			checkError(err)
			fmt.Printf("Cytoscope Config:\n%v\n", string(b))
		}

		break
		//time.Sleep(o.interval)
		//start = start.Add(o.interval)
	}
}

func linkPromGraph(server, ts, name, version string) (link string) {
	promExpr := fmt.Sprintf("%v{destination_service=\"%v\",destination_version=\"%v\"}", ts, name, version)
	link = fmt.Sprintf("%v/graph?g0.range_input=1h&g0.tab=0&g0.expr=%v", server, url.QueryEscape(promExpr))
	return link
}

type Destination map[string]interface{}

// toDestinations takes a slice of [istio] series and returns a map K => D
// key = "destSvc destVersion"
// val = Destination (map) with the following keys
//          req_per_min     float64
//          req_per_min_2xx float64
//          req_per_min_3xx float64
//          req_per_min_4xx float64
//          req_per_min_5xx float64
func toDestinations(sourceSvc, sourceVer string, vector model.Vector) (destinations map[string]Destination) {
	destinations = make(map[string]Destination)
	for _, s := range vector {
		m := s.Metric
		destSvc, destSvcOk := m["destination_service"]
		destVer, destVerOk := m["destination_version"]
		code, codeOk := m["response_code"]
		if !destSvcOk || !destVerOk || !codeOk {
			fmt.Printf("Skipping %v, missing destination labels", m.String())
		}

		// not expected but remove any self-invocations
		if sourceSvc == string(destSvc) && sourceVer == string(destVer) {
			continue
		}

		if destSvcOk && destVerOk {
			k := fmt.Sprintf("%v %v", destSvc, destVer)
			dest, destOk := destinations[k]
			if !destOk {
				dest = Destination(make(map[string]interface{}))
				dest["req_per_min"] = 0.0
				dest["req_per_min_2xx"] = 0.0
				dest["req_per_min_3xx"] = 0.0
				dest["req_per_min_4xx"] = 0.0
				dest["req_per_min_5xx"] = 0.0
			}
			val := float64(s.Value)
			var ck string
			switch {
			case strings.HasPrefix(string(code), "2"):
				ck = "req_per_min_2xx"
			case strings.HasPrefix(string(code), "3"):
				ck = "req_per_min_3xx"
			case strings.HasPrefix(string(code), "4"):
				ck = "req_per_min_4xx"
			case strings.HasPrefix(string(code), "5"):
				ck = "req_per_min_5xx"
			}
			dest[ck] = dest[ck].(float64) + val
			dest["req_per_min"] = dest["req_per_min"].(float64) + val

			destinations[k] = dest
		}
	}
	return destinations
}

// TF is the TimeFormat for printing timestamp
const TF = "2006-01-02 15:04:05"

func promQuery(query string, queryTime time.Time, api v1.API) model.Vector {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Printf("Executing query %s&time=%v (now=%v)\n", query, queryTime.Format(TF), time.Now().Format(TF))

	value, err := api.Query(ctx, query, queryTime)
	checkError(err)

	switch t := value.Type(); t {
	case model.ValVector: // Instant Vector
		return value.(model.Vector)
	default:
		checkError(errors.New(fmt.Sprintf("No handling for type %v!\n", t)))
	}

	return nil
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
