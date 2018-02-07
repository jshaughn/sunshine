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

	"github.com/jshaughn/sunshine/tree"
	"github.com/jshaughn/sunshine/vizceral"

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

func (ts TSExpression) buildTree(n *tree.Tree, start time.Time, interval time.Duration, api v1.API) {
	query := fmt.Sprintf("sum(rate(%v{source_service=\"%v\",source_version=\"%v\",response_code=~\"%v\"} [%vs]) * 60) by (%v)",
		ts,                                                      // the metric
		n.Name,                                                  // parent service name
		n.Version,                                               // parent service version
		"[2345][0-9][0-9]",                                      // regex for valid response_codes
		interval.Seconds(),                                      // rate over the entire query period
		"destination_service,destination_version,response_code") // group by

	// fetch the root time series
	vector := promQuery(query, start, api)
	//fmt.Printf("Found [%v] parent series\n", len(series))

	// identify the unique destination services
	destinations := toDestinations(n.Name, n.Version, vector)
	//fmt.Printf("Found [%v] child destinations\n", len(destinations))

	if len(destinations) > 0 {
		n.Children = make([]*tree.Tree, len(destinations))
		i := 0
		for k, d := range destinations {
			s := strings.Split(k, " ")
			child := tree.Tree{
				Name:    s[0],
				Version: s[1],
				Normal:  d["normal"],
				Warning: d["warning"],
				Danger:  d["danger"],
				Parent:  n,
			}
			fmt.Printf("Child Service: %v(%v)->%v(%v)\n", n.Name, n.Version, child.Name, child.Version)
			n.Children[i] = &child
			i++
			ts.buildTree(&child, start, interval, api)
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
			promExpr := fmt.Sprintf("%v{destination_service=\"%v\",destination_version=\"%v\"}", ts, s[0], s[1])
			promUrl := fmt.Sprintf("%v/graph?g0.range_input=1h&g0.tab=0&g0.expr=%v", o.server, url.QueryEscape(promExpr))
			root := tree.Tree{
				Query:   promUrl,
				Name:    s[0],
				Version: s[1],
				Normal:  d["normal"],
				Warning: d["warning"],
				Danger:  d["danger"],
				Parent:  nil,
			}
			fmt.Printf("Root Service: %v(%v)\n", root.Name, root.Version)
			ts.buildTree(&root, queryTime, o.interval, api)
			trees[i] = root
			i++
		}

		for _, t := range trees {
			c := vizceral.NewConfig(&t)
			fmt.Printf("Config:\n%+v\n", c)
			b, err := json.MarshalIndent(c, "", "  ")
			checkError(err)
			fmt.Printf("Config:\n%v\n", string(b))
		}

		break
		//time.Sleep(o.interval)
		//start = start.Add(o.interval)
	}
}

type Destination map[string]float64

// toDestinations takes a slice of [istio] series and returns a map with keys "destSvc destVersion", removing self-invocation
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
				dest = Destination(make(map[string]float64))
			}
			val := float64(s.Value)
			switch {
			case strings.HasPrefix(string(code), "2"):
				dest["normal"] += val
			case strings.HasPrefix(string(code), "4"):
			case strings.HasPrefix(string(code), "5"):
				dest["danger"] += val
			default:
				dest["warn"] += val
			}

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
