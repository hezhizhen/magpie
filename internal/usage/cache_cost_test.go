package usage

import (
	"testing"
	"time"

	"github.com/yetone/magpie/internal/settings"
)

// Cache traffic still costs money when a call reports no uncached input
// or output. The ledger, its compact page and the session projection agree.
func TestCacheOnlyCosts(t *testing.T) {
	for _, tc := range []struct {
		name, model       string
		read, write, hour int
		cost              float64
		unpriced          int
		rowPriced         bool
	}{
		{"read", "m", 1_000_000, 0, 0, 0.5, 0, true},
		{"write", "m", 0, 1_000_000, 0, 2.5, 0, true},
		{"one-hour write", "m", 0, 1_000_000, 1_000_000, 4, 0, true},
		{"unknown price", "cache-only-unknown", 1_000_000, 0, 0, 0, 1, false},
		{"no usage", "cache-only-unknown", 0, 0, 0, 0, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pageHome(t)
			now := holdClock(t, time.Date(2026, 10, 10, 12, 0, 0, 0, time.Local))
			if err := settings.Save(settings.Settings{ModelPrices: map[string]settings.ModelPrice{
				"relay/m": {Input: new(2.0), Output: new(8.0), CacheRead: new(0.5), CacheWrite: new(2.5), CacheWrite1h: new(4.0)},
			}}); err != nil {
				t.Fatal(err)
			}
			r := Record{Time: now.Add(-time.Hour), Agent: "pi", Session: "cache-only", Provider: "relay", Model: tc.model,
				CacheRead: tc.read, CacheWrite: tc.write, CacheWrite1h: tc.hour, Status: 200}
			Append(r)
			if cost, priced := NewCoster()(r); cost != tc.cost || priced != (tc.unpriced == 0) {
				t.Errorf("coster: cost=%v priced=%v, want %v unpriced=%d", cost, priced, tc.cost, tc.unpriced)
			}
			ledger := LedgerOf(All, Filter{})
			page := QueryPage(Month, Filter{}, 0, 10)
			for name, totals := range map[string]Totals{
				"summary": Summarize(All).Totals, "ledger": ledger.Sum, "page": page.Sum,
			} {
				if totals.Cost != tc.cost || totals.Unpriced != tc.unpriced || totals.Tokens() != 0 || totals.CacheRead != tc.read || totals.CacheWrite != tc.write {
					t.Errorf("%s: %+v, want cost=%v unpriced=%d and only cache traffic", name, totals, tc.cost, tc.unpriced)
				}
			}
			for name, rows := range map[string][]Row{"ledger": ledger.Rows, "page": page.Rows} {
				if len(rows) != 1 || rows[0].Cost != tc.cost || rows[0].Priced != tc.rowPriced {
					t.Errorf("%s rows: %+v, want cost=%v priced=%v", name, rows, tc.cost, tc.rowPriced)
				}
			}
			var series Totals
			for _, p := range page.Series {
				series.Cost += p.Cost
				series.Unpriced += p.Unpriced
			}
			if series.Cost != tc.cost || series.Unpriced != tc.unpriced {
				t.Errorf("series: %+v, want cost=%v unpriced=%d", series, tc.cost, tc.unpriced)
			}
			heat := HeatmapOf(Filter{})
			if len(heat.Days) != 1 || heat.Days[0].Cost != tc.cost || heat.Days[0].Unpriced != tc.unpriced {
				t.Errorf("heatmap: %+v, want cost=%v unpriced=%d", heat, tc.cost, tc.unpriced)
			}
			ss, days := GatewaySessionWindow(time.Time{})
			if len(ss) != 1 || len(ss[0].Models) != 1 || len(days) != 1 {
				t.Fatalf("sessions=%+v days=%+v", ss, days)
			}
			if ss[0].Cost != tc.cost || ss[0].Unpriced != tc.unpriced || ss[0].Models[0].Cost != tc.cost || ss[0].Models[0].Priced != (tc.unpriced == 0) || days[0].Cost != tc.cost || days[0].Priced != (tc.unpriced == 0) {
				t.Errorf("session projection: %+v days=%+v, want cost=%v unpriced=%d", ss, days, tc.cost, tc.unpriced)
			}
		})
	}
}
