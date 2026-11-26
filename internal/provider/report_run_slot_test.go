package provider_test

import (
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sync/semaphore"
)

// TestAcquireReportRunSlots_LimitsConcurrentHolders checks that the units held
// at once never exceed the semaphore's capacity, for single-unit holders and for
// exclusive holders (every unit) mixed together, and that every holder gets its
// turn. An exclusive holder can only keep to the capacity by running alone.
// Each subtest is parallel, as the acceptance tests are, and holds its units for
// the whole subtest.
func TestAcquireReportRunSlots_LimitsConcurrentHolders(t *testing.T) {
	const (
		capacity  = 3
		singles   = 8
		exclusive = 2
	)

	sem := semaphore.NewWeighted(capacity)
	var unitsInFlight, peakUnits, finished atomic.Int64

	hold := func(t *testing.T, units int64) {
		t.Parallel()
		acquireReportRunSlots(t, sem, units)

		now := unitsInFlight.Add(units)
		for {
			seen := peakUnits.Load()
			if now <= seen || peakUnits.CompareAndSwap(seen, now) {
				break
			}
		}
		// Long enough for the other parallel holders to reach the gate.
		time.Sleep(20 * time.Millisecond)
		unitsInFlight.Add(-units)
		finished.Add(1)
	}

	t.Run("holders", func(t *testing.T) {
		for range singles {
			t.Run("single", func(t *testing.T) { hold(t, 1) })
		}
		for range exclusive {
			t.Run("exclusive", func(t *testing.T) { hold(t, capacity) })
		}
	})

	if got := peakUnits.Load(); got > capacity {
		t.Errorf("peak units held at once = %d, want at most %d", got, capacity)
	}
	if got, want := finished.Load(), int64(singles+exclusive); got != want {
		t.Errorf("finished holders = %d, want %d: a slot was never released or a holder starved", got, want)
	}
	if !sem.TryAcquire(capacity) {
		t.Error("units still held after every holder finished")
	}
}
