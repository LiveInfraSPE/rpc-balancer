package main

import (
	"flag"
	"fmt"
	_ "net/http/pprof"
	"sync"

	"github.com/LiveInfraSPE/rpc-balancer/rpcbalancer"
	log "github.com/sirupsen/logrus"
)

// Define a custom type to store a slice of strings
type stringSlice []string

// Implement the flag.String interface method for the custom type
func (s *stringSlice) String() string {
	return fmt.Sprintf("%v", *s)
}

// Implement the flag.Set interface method for the custom type
func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	var (
		rpcPort             string
		metricsPort         string
		healthcheckinterval int
		nodes               []string
		fallback            string
		selectionMethod     string
		loglevel            string
		urlKey              string
	)

	//parse command line flags

	flag.StringVar(&rpcPort, "rpcport", "8080", "Port to run the server on (default 8080)")
	flag.StringVar(&metricsPort, "metricsport", "8081", "Port to run the metrics server on (default 8081)")
	flag.StringVar(&fallback, "fallback", "", "Fallback node to use (default: none)")
	flag.StringVar(&selectionMethod, "selectionmethod", "failover", "Selection method to use (default: failover; other options are roundrobin and random)")
	flag.StringVar(&loglevel, "loglevel", "info", "Log level to use (default: info)")
	flag.StringVar(&rpcbalancer.UrlKey, "urlkey", "", "URL key to use (default: \"\"")
	flag.IntVar(&healthcheckinterval, "healthcheckinterval", 5, "Interval in seconds to check node health (default 5)")
	flag.Var((*stringSlice)(&nodes), "node", "Node to add to the pool")
	flag.Parse()

	rpcbalancer.SetLogLevel(loglevel)
	// create a new pool
	pool := rpcbalancer.NewPool(selectionMethod)

	// add nodes to the pool
	for i, addr := range nodes {
		pool.AddNode(rpcbalancer.NewNode(addr), i)
	}

	// set the fallback node if provided
	if fallback != "" {
		pool.Fallback = rpcbalancer.NewNode(fallback)
	} else {
		pool.Fallback = rpcbalancer.NewNode(nodes[0])
	}

	go pool.StartHealthCheckLoop(healthcheckinterval)

	// create a waitgroup to handle multiple running goroutines
	var wg sync.WaitGroup

	wg.Add(1)
	log.Info("Starting webserver")
	go rpcbalancer.RunWebServer(&wg, rpcPort, pool, urlKey)

	wg.Add(1)
	log.Info("Starting metrics server")
	go rpcbalancer.RunMetricsServer(&wg, metricsPort)

	wg.Wait()

	fmt.Println("Webservers have exited.")

}
