package httpapi

import (
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var placeSearchState = struct {
	sync.Mutex
	last  time.Time
	cache map[string][]map[string]any
}{cache: map[string][]map[string]any{}}

func validEventGeo(value string) bool {
	if value == "" {
		return true
	}
	parts := strings.Split(value, ";")
	if len(parts) != 2 {
		return false
	}
	lat, e1 := strconv.ParseFloat(parts[0], 64)
	lon, e2 := strconv.ParseFloat(parts[1], 64)
	return e1 == nil && e2 == nil && !math.IsNaN(lat) && !math.IsNaN(lon) && math.Abs(lat) <= 90 && math.Abs(lon) <= 180
}
func (s *Server) handleCalendarPlaces(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(query) < 3 || len(query) > 200 {
		writeJSON(w, 400, map[string]string{"error": "place query must contain 3–200 characters"})
		return
	}
	placeSearchState.Lock()
	defer placeSearchState.Unlock()
	if cached, ok := placeSearchState.cache[query]; ok {
		writeJSON(w, 200, map[string]any{"places": cached})
		return
	}
	if time.Since(placeSearchState.last) < time.Second {
		writeJSON(w, 429, map[string]string{"error": "please wait a moment before searching again"})
		return
	}
	placeSearchState.last = time.Now()
	endpoint := os.Getenv("TAYGA_GEOCODER_URL")
	if endpoint == "" {
		endpoint = "https://photon.komoot.io/api/"
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		writeJSON(w, 503, map[string]string{"error": "geocoder is not configured"})
		return
	}
	params := parsed.Query()
	params.Set("q", query)
	params.Set("limit", "5")
	parsed.RawQuery = params.Encode()
	req, err := http.NewRequestWithContext(r.Context(), "GET", parsed.String(), nil)
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "geocoder unavailable"})
		return
	}
	req.Header.Set("User-Agent", "Tayga-Mail/0.9 calendar-place-search (+https://github.com/rkislov/tayga-mail-server)")
	client := &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		writeJSON(w, 502, map[string]string{"error": "geocoder unavailable"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		writeJSON(w, 502, map[string]string{"error": "geocoder unavailable"})
		return
	}
	var data struct {
		Features []struct {
			Geometry struct {
				Coordinates []float64 `json:"coordinates"`
			} `json:"geometry"`
			Properties map[string]any `json:"properties"`
		} `json:"features"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1024*1024)).Decode(&data) != nil {
		writeJSON(w, 502, map[string]string{"error": "invalid geocoder response"})
		return
	}
	places := []map[string]any{}
	for _, feature := range data.Features {
		if len(feature.Geometry.Coordinates) != 2 {
			continue
		}
		coords := feature.Geometry.Coordinates
		geo := strconv.FormatFloat(coords[1], 'f', 6, 64) + ";" + strconv.FormatFloat(coords[0], 'f', 6, 64)
		if !validEventGeo(geo) {
			continue
		}
		labels := []string{}
		for _, key := range []string{"name", "street", "housenumber", "city", "state", "country"} {
			if v, ok := feature.Properties[key].(string); ok && v != "" {
				labels = append(labels, v)
			}
		}
		places = append(places, map[string]any{"label": strings.Join(labels, ", "), "geo": geo})
		if len(places) == 5 {
			break
		}
	}
	if len(placeSearchState.cache) >= 256 {
		placeSearchState.cache = map[string][]map[string]any{}
	}
	placeSearchState.cache[query] = places
	writeJSON(w, 200, map[string]any{"places": places})
}
