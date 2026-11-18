package main

import (
	"encoding/json"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Rates are the euro reference rates of the European Central Bank: 1 EUR = PerEUR[currency].
type Rates struct {
	Date    string             `json:"date"`
	PerEUR  map[string]float64 `json:"perEUR"`
	Fetched time.Time          `json:"fetched"`
}

var (
	reECBTime = regexp.MustCompile(`time='(\d{4}-\d{2}-\d{2})'`)
	reECBRate = regexp.MustCompile(`currency='([A-Z]{3})' rate='([\d.]+)'`)
	ratesMu   sync.Mutex
	ratesMem  Rates
	ratesTry  time.Time
)

// rates returns the ECB rates, fetched at most twice a day (one try every 30
// minutes when offline) and kept in the cache: the last ones saved serve offline.
func (a *App) rates() Rates {
	ratesMu.Lock()
	defer ratesMu.Unlock()
	file := filepath.Join(a.cacheDir, "rates.json")
	if ratesMem.PerEUR == nil {
		if data, err := os.ReadFile(file); err == nil {
			_ = json.Unmarshal(data, &ratesMem)
		}
	}
	if time.Since(ratesMem.Fetched) < 12*time.Hour || time.Since(ratesTry) < 30*time.Minute {
		return ratesMem
	}
	ratesTry = time.Now()
	if fresh, ok := fetchECB(); ok {
		ratesMem = fresh
		if data, err := json.Marshal(fresh); err == nil {
			_ = os.MkdirAll(a.cacheDir, 0o755)
			_ = os.WriteFile(file, data, 0o644)
		}
	}
	return ratesMem
}

func fetchECB() (Rates, bool) {
	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Get("https://www.ecb.europa.eu/stats/eurofxref/eurofxref-daily.xml")
	if err != nil {
		return Rates{}, false
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil || resp.StatusCode != http.StatusOK {
		return Rates{}, false
	}
	r := Rates{PerEUR: map[string]float64{}, Fetched: time.Now()}
	if m := reECBTime.FindSubmatch(body); m != nil {
		r.Date = string(m[1])
	}
	for _, m := range reECBRate.FindAllSubmatch(body, -1) {
		if v, err := strconv.ParseFloat(string(m[2]), 64); err == nil && v > 0 {
			r.PerEUR[string(m[1])] = v
		}
	}
	return r, len(r.PerEUR) > 0
}

// toEUR converts an amount; false when the rate of its currency is unknown.
func (r Rates) toEUR(amount float64, currency string) (float64, bool) {
	if currency == "" || currency == "EUR" {
		return amount, true
	}
	if rate := r.PerEUR[currency]; rate > 0 {
		return amount / rate, true
	}
	return 0, false
}

// ForecastPoint is the balance left after one due date.
type ForecastPoint struct {
	Date    string  `json:"date"`
	Name    string  `json:"name"`
	Domain  string  `json:"domain,omitempty"`
	Amount  float64 `json:"amount"` // in EUR
	Balance float64 `json:"balance"`
}

// Forecast is the reserve of the account spent by the planned payments.
type Forecast struct {
	Set      bool            `json:"set"`
	Balance  float64         `json:"balance"`
	Date     string          `json:"date,omitempty"`
	RunsOut  string          `json:"runsOut,omitempty"` // first day below zero, within three years
	PerMonth float64         `json:"perMonth"`          // the running subscriptions, in EUR per month
	USDToEUR float64         `json:"usdToEur,omitempty"`
	RateDate string          `json:"rateDate,omitempty"`
	Approx   bool            `json:"approx"` // an amount in a currency with no known rate was left out
	Points   []ForecastPoint `json:"points"`
}

var perMonthFactor = map[string]float64{"weekly": 52.0 / 12, "monthly": 1, "quarterly": 1.0 / 3, "yearly": 1.0 / 12}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// forecast takes off the reserve, at its date, each due date after the day
// the reserve was written, until it goes below zero or for three years.
func (m *model) forecast(rates Rates) Forecast {
	f := Forecast{Points: []ForecastPoint{}, RateDate: rates.Date}
	if usd := rates.PerEUR["USD"]; usd > 0 {
		f.USDToEUR = math.Round(10000/usd) / 10000
	}
	today := m.today.Format(dayFmt)
	for _, p := range m.db.Payments {
		if perMonthFactor[p.Cycle] == 0 || (p.End != "" && p.End < today) {
			continue
		}
		est := m.estimate(p)
		eur, ok := rates.toEUR(est.Amount, est.Currency)
		f.Approx = f.Approx || !ok
		f.PerMonth += eur * perMonthFactor[p.Cycle]
	}
	f.PerMonth = round2(f.PerMonth)
	res := m.db.Reserve
	start, ok := time.Time{}, false
	if res != nil {
		start, ok = parseDay(res.Date)
	}
	if !ok {
		return f
	}
	f.Set, f.Balance, f.Date = true, res.Amount, res.Date
	var points []ForecastPoint
	for _, p := range m.db.Payments {
		// The reserve counts a variable service at the highest of its last invoices.
		est := m.estimate(p)
		eur, ok := rates.toEUR(est.Prudent, est.Currency)
		if !ok {
			f.Approx = true
			continue
		}
		for _, d := range p.dues(start.AddDate(0, 0, 1), start.AddDate(3, 0, 0)) {
			points = append(points, ForecastPoint{Date: d.Format(dayFmt), Name: p.Name, Domain: p.Domain, Amount: round2(eur)})
		}
	}
	sort.SliceStable(points, func(i, j int) bool { return points[i].Date < points[j].Date })
	balance := res.Amount
	for _, pt := range points {
		balance -= pt.Amount
		pt.Balance = round2(balance)
		f.Points = append(f.Points, pt)
		if balance < 0 {
			f.RunsOut = pt.Date
			break
		}
	}
	return f
}
