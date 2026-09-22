package concurrency_test

import (
	"NeoNect/test/harness"
	"sync"
	"testing"
)

func TestConcurrentRegistrations(t *testing.T) {
	h := harness.Setup(t)

	h.RegisterUser(t, "raceuser", "Password1234")
	cookie := h.Login(t, "raceuser", "Password1234")

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			h.RegisterDevice(t, cookie, "dev_race")
		}(i)
	}
	wg.Wait()
}
