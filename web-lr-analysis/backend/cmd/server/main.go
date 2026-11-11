package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/local/pedal-balance-web/internal/analyzer"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const maxUpload = 25 << 20

func main() {
	addr := env("ADDR", "127.0.0.1:8080")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /api/analyse", analyse)
	s := &http.Server{Addr: addr, Handler: security(mux), ReadHeaderTimeout: 5000000000}
	log.Printf("Go analysis API listening on http://%s", addr)
	log.Fatal(s.ListenAndServe())
}
func analyse(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		problem(w, http.StatusBadRequest, "upload must be a multipart FIT file no larger than 25 MB")
		return
	}
	f, h, err := r.FormFile("file")
	if err != nil {
		problem(w, http.StatusBadRequest, "missing FIT file")
		return
	}
	defer f.Close()
	if !strings.HasSuffix(strings.ToLower(h.Filename), ".fit") {
		problem(w, http.StatusBadRequest, "file must have a .fit extension")
		return
	}
	o := analyzer.DefaultOptions()
	for name, dst := range map[string]*float64{"leftCrankMM": &o.LeftCrankMM, "rightCrankMM": &o.RightCrankMM, "configuredCrankMM": &o.ConfiguredCrankMM, "powerBucketW": &o.PowerBucketW, "cadenceBucketRPM": &o.CadenceBucketRPM, "minDurationSeconds": &o.MinDurationSeconds, "minRegressionPowerW": &o.MinRegressionPowerW, "minRegressionCadenceRPM": &o.MinRegressionCadenceRPM} {
		if v := r.FormValue(name); v != "" {
			n, e := strconv.ParseFloat(v, 64)
			if e != nil {
				problem(w, 400, fmt.Sprintf("invalid %s", name))
				return
			}
			*dst = n
		}
	}
	if v := r.FormValue("minSamples"); v != "" {
		n, e := strconv.Atoi(v)
		if e != nil {
			problem(w, 400, "invalid minSamples")
			return
		}
		o.MinSamples = n
	}
	samples, skipped, err := analyzer.Decode(f)
	if err != nil {
		problem(w, 422, err.Error())
		return
	}
	report, err := analyzer.Analyse(h.Filename, samples, skipped, o)
	if err != nil {
		problem(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, report)
}
func security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		if origin := os.Getenv("CORS_ORIGIN"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		next.ServeHTTP(w, r)
	})
}
func problem(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil && !errors.Is(err, http.ErrHandlerTimeout) {
		log.Printf("encode response: %v", err)
	}
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
