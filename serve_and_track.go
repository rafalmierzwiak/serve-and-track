/*

A tracking web server.

Main function is to serve an image and log requests in apache log format. Served
are also service status (health established based on presence of a state file),
and service metrics (via use of Prometheus client library).

*/
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

// Command line configuration options
var (
	listenAddress = kingpin.Flag("listen-address", "Address on which to expose metrics and web interface.").Default(":8080").String()

	trackingURLPath = kingpin.Flag("tracking-url-path", "Path under which to expose tracking image.").Default("/track").String()
	metricsURLPath  = kingpin.Flag("metrics-url-path", "Path under which to expose service metrics.").Default("/metrics").String()
	stateURLPath    = kingpin.Flag("state-url-path", "Path under which to expose service state.").Default("/state").String()

	stateFilePath = kingpin.Flag("state-file-path", "File path which indicates service state.").Default("./state").String()

	accessLogFilePath  = kingpin.Flag("access-log-path", "File path where requests will be logged.").String()
	serviceLogFilePath = kingpin.Flag("service-log-path", "File path where requests will be logged.").String()
)

// GIF transparent image to serve as a tracking image
var GIF = []byte{
	71, 73, 70, 56, 57, 97, 1, 0, 1, 0, 128, 0, 0, 0, 0, 0,
	255, 255, 255, 33, 249, 4, 1, 0, 0, 0, 0, 44, 0, 0, 0, 0,
	1, 0, 1, 0, 0, 2, 1, 68, 0, 59,
}

// Monitoring metrics
var (
	serveImageRequestDuration = prometheus.NewSummary(prometheus.SummaryOpts{
		Name:       "tracking_request_duration",
		Help:       "Duration of requests.",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	})

	serveImageRequestsSize = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "tracking_requests_size_total",
		Help: "Size of requests, total.",
	})

	serveImageRequestsCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tracking_requests_count_total",
			Help: "Number of requests served partitioned by status (failure or success).",
		},
		[]string{"status"},
	)
)

// Initializes service metrics.
func initMetrics() {
	prometheus.MustRegister(serveImageRequestDuration)
	prometheus.MustRegister(serveImageRequestsCount)
	prometheus.MustRegister(serveImageRequestsSize)
}

// Initializes the http server.
func initServer() *http.Server {
	r := mux.NewRouter()

	r.HandleFunc(*trackingURLPath, serveImage)
	r.HandleFunc(*stateURLPath, serveState)
	r.Handle(*metricsURLPath, promhttp.Handler())

	if serviceLog, err := os.OpenFile(*serviceLogFilePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600); err != nil {
		log.SetOutput(os.Stderr)
	} else {
		log.SetOutput(serviceLog)
	}

	accessLog, err := os.OpenFile(*accessLogFilePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		accessLog = os.Stdout
	}

	return &http.Server{
		Addr:    *listenAddress,
		Handler: handlers.CombinedLoggingHandler(accessLog, r)}
}

// Starts the http server.
func startServer(srv *http.Server) {
	log.Println("INFO http: Server started", *listenAddress)

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal("ERROR ", err)
	}
}

// Stops the http server.
func stopServer(srv *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	log.Println("INFO http: Server stopping")

	if err := srv.Shutdown(ctx); err != nil {
		log.Println("INFO http: Server stopped forcefully")
	} else {
		log.Println("INFO http: Server stopped gracefully")
	}
}

// Measures function execution time.
func trackServeImageDuration(start time.Time, id string) {
	elapsed := time.Since(start)
	serveImageRequestDuration.Observe(float64(elapsed.Seconds()))
}

// Serves tracking image.
func serveImage(w http.ResponseWriter, r *http.Request) {
	defer trackServeImageDuration(time.Now(), "serveImage")

	if r.Method != "GET" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Content-Type", "image/gif")

	if _, err := w.Write(GIF); err != nil {
		serveImageRequestsCount.WithLabelValues("failure").Inc()
		return
	}

	serveImageRequestsCount.WithLabelValues("success").Inc()
	serveImageRequestsSize.Add(float64(len(GIF)))
}

// Checks service state: true if service is healthy, false otherwise. Service is
// considered healthy when stateFilePath is present.
func serviceHealthy() bool {
	_, err := os.Stat(*stateFilePath)
	return err == nil
}

// Serves service state: http 200 when healthy, http 503 otherwise.
func serveState(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Content-Type", "text/html")

	if !serviceHealthy() {
		w.WriteHeader(503)
		if _, err := w.Write([]byte("Error 503 (Service not available)")); err != nil {
			log.Println("WARNING", err)
		}
		return
	}

	if _, err := w.Write([]byte("OK")); err != nil {
		log.Println("WARNING", err)
	}
}

func main() {
	kingpin.Parse()

	terminateServer := make(chan os.Signal)
	signal.Notify(terminateServer, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	initMetrics()

	srv := initServer()

	go func() {
		startServer(srv)
	}()

	<-terminateServer

	stopServer(srv)
}
