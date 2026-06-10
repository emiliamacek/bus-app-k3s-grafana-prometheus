package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// metryki

var reg = prometheus.NewRegistry()

var (
	// counter: inkrementowany przy każdym żądaniu HTTP — do oblcizeń request rate oraz error rate
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "total HTTP requests by method, path, and status code.",
		},
		[]string{"method", "path", "status"},
	)

	// histogram: sortuje czasy odpowiedzi do kubełków — umożliwia kwerendy wyznaczające percentyle, np. histogram_quantile(0.99, ...)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.0, 5.0},
		},
		[]string{"method", "path"},
	)

	// gauge: obecna liczba aktywnych żądań
	httpActiveRequests = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_active_requests",
			Help: "number of in-flight HTTP requests.",
		},
	)


	// gauge: inkrementowany przez /subscribe/{stop_id}, ale nigdy nie spada [wyciek pamięci]
	busSubscribers = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "bus_stop_subscribers_total",
			Help: "number of registered stop subscribers (never cleaned up).",
		},
	)
)

func init() {
	reg.MustRegister(httpRequestsTotal, httpRequestDuration, httpActiveRequests, busSubscribers)
}

// middleware

// przechwycenie kodu statusu zanim zostanie wysłany do klienta
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// instrument() opakowuje handler i ogarnia 3 główne metryki dla każdego żądania
func instrument(path string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		start := time.Now()
		httpActiveRequests.Inc()
		defer func() {
			httpActiveRequests.Dec()
			httpRequestsTotal.WithLabelValues(r.Method, path, strconv.Itoa(rec.status)).Inc()
			httpRequestDuration.WithLabelValues(r.Method, path).Observe(time.Since(start).Seconds())
		}()
		next(rec, r)
	}
}

// handlers

type Bus struct {
	ID       string  `json:"id"`
	Line     string  `json:"line"`
	Lat      float64 `json:"latitude"`
	Lon      float64 `json:"longitude"`
	SpeedKmh float64 `json:"speed_kmh"`
}

var fleetMu sync.RWMutex

var fleet = []Bus{
	{ID: "bus-001", Line: "194", Lat: 52.2297, Lon: 21.0122, SpeedKmh: 42.5},
	{ID: "bus-002", Line: "194", Lat: 52.2310, Lon: 21.0145, SpeedKmh: 38.2},
	{ID: "bus-003", Line: "124", Lat: 52.2150, Lon: 20.9810, SpeedKmh: 55.0},
	{ID: "bus-004", Line: "124", Lat: 52.2200, Lon: 20.9900, SpeedKmh: 0.0},
	{ID: "bus-005", Line: "503", Lat: 52.2450, Lon: 21.0300, SpeedKmh: 61.3},
}

// GET /buses
func handleBuses(w http.ResponseWriter, r *http.Request) {
	fleetMu.RLock()
	snapshot := make([]Bus, len(fleet))
	copy(snapshot, fleet)
	fleetMu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(snapshot)
	if err != nil {
		return
	}
}

// GET /route/{id}
func handleRoute(w http.ResponseWriter, r *http.Request) {
	time.Sleep(time.Duration(rand.Intn(2000)) * time.Millisecond)
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(map[string]any{
		"route_id":          r.PathValue("id"),
		"stops":             rand.Intn(20) + 5,
		"estimated_minutes": rand.Intn(45) + 10,
	})
	if err != nil {
		return
	}
}

// POST /buses/{id}/position
func handleGPSUpdate(w http.ResponseWriter, r *http.Request) {
	if rand.Float64() < 0.2 {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":"GPS module timeout"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, err := fmt.Fprintf(w, `{"status":"ok","bus_id":%q}`, r.PathValue("id"))
	if err != nil {
		return
	}
}

// GET /subscribe/{stop_id}
func handleSubscribe(w http.ResponseWriter, r *http.Request) {
	busSubscribers.Inc()
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(map[string]string{
		"stop_id": r.PathValue("stop_id"),
		"status":  "subscribed",
	})
	if err != nil {
		return
	}
}

func startSimulation() {
	time.Sleep(1 * time.Second)

	client := &http.Client{Timeout: 5 * time.Second}

	go func() {
		for {
			_, err := client.Get("http://localhost:8080/buses")
			if err != nil {
				return
			}
			time.Sleep(time.Duration(108000) * time.Millisecond)
		}
	}()


}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /buses", instrument("/buses", handleBuses))
	mux.HandleFunc("GET /route/{id}", instrument("/route/{id}", handleRoute))
	mux.HandleFunc("POST /buses/{id}/position", instrument("/buses/{id}/position", handleGPSUpdate))
	mux.HandleFunc("GET /subscribe/{stop_id}", instrument("/subscribe/{stop_id}", handleSubscribe))
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	// catch-all
	mux.HandleFunc("/", instrument("/unknown", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}))

	go startSimulation()

	fmt.Println("Bus tracker API running on :8080")
	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		return
	}
}
