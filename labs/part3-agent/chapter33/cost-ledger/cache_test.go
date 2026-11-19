package main

import "testing"

func TestCacheWritesNeedEnoughReuseToAmortize(t *testing.T) {
	for _, c := range []struct {
		name          string
		calls         int
		writes        map[int]bool
		plain, cached int64
	}{
		{"first write", 1, map[int]bool{0: true}, 200, 300},
		{"two calls tie", 2, map[int]bool{0: true}, 400, 400},
		{"third call gains", 3, map[int]bool{0: true}, 600, 500},
		{"five calls", 5, map[int]bool{0: true}, 1000, 700},
		{"prefix changed", 5, map[int]bool{0: true, 3: true}, 1000, 900},
		{"every call rewrites", 3, map[int]bool{0: true, 1: true, 2: true}, 600, 900},
	} {
		t.Run(c.name, func(t *testing.T) {
			plain, cached := ledger(t, 10000), ledger(t, 10000)
			for n := 0; n < c.calls; n++ {
				id := sent(t, plain, Bound{Input: 100})
				if err := plain.Settle(id, Usage{Input: 100}); err != nil {
					t.Fatal(err)
				}
				id = sent(t, cached, Bound{Input: 100})
				usage := Usage{CacheRead: 100}
				if c.writes[n] {
					usage = Usage{CacheWrite: 100}
				}
				if err := cached.Settle(id, usage); err != nil {
					t.Fatal(err)
				}
			}
			p, q := plain.Snapshot(), cached.Snapshot()
			if p.Used != c.plain || q.Used != c.cached || p.Held != 0 || q.Held != 0 {
				t.Fatal("amortization or settlement changed", p, q, c)
			}
		})
	}
}
