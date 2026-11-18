package ledger

import (
	"math"
	"sort"
	"time"

	"github.com/coddyum/kanso/internal/day"
	"github.com/coddyum/kanso/internal/rates"
)

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

// Forecast takes off the reserve, at its date, each due date after the day
// the reserve was written, until it goes below zero or for three years. A
// variable service counts at the highest of its last invoices.
func (m *Model) Forecast(r rates.Rates) Forecast {
	f := Forecast{Points: []ForecastPoint{}, RateDate: r.Date, USDToEUR: r.USDToEUR()}
	today := m.Today.Format(day.Format)
	for _, p := range m.DB.Payments {
		if perMonthFactor[p.Cycle] == 0 || (p.End != "" && p.End < today) {
			continue
		}
		est := m.estimate(p)
		eur, ok := r.ToEUR(est.Amount, est.Currency)
		f.Approx = f.Approx || !ok
		f.PerMonth += eur * perMonthFactor[p.Cycle]
	}
	f.PerMonth = round2(f.PerMonth)
	res := m.DB.Reserve
	start, ok := time.Time{}, false
	if res != nil {
		start, ok = day.Parse(res.Date)
	}
	if !ok {
		return f
	}
	f.Set, f.Balance, f.Date = true, res.Amount, res.Date
	var points []ForecastPoint
	for _, p := range m.DB.Payments {
		est := m.estimate(p)
		eur, ok := r.ToEUR(est.Prudent, est.Currency)
		if !ok {
			f.Approx = true
			continue
		}
		for _, d := range p.Dues(start.AddDate(0, 0, 1), start.AddDate(3, 0, 0)) {
			points = append(points, ForecastPoint{Date: d.Format(day.Format), Name: p.Name, Domain: p.Domain, Amount: round2(eur)})
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
