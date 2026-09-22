package shutdown_test

import (
	"NeoNect/test/harness"
	"context"
	"sync"
	"testing"
)

func TestShutdown_Concurrent(t *testing.T) {
	h := harness.Setup(t)

	// Create some connections first so there is state to tear down
	h.RegisterUser(t, "user_concurrent", "password123")
	cookie := h.Login(t, "user_concurrent", "password123")
	h.RegisterDevice(t, cookie, "dev1")
	h.RegisterDevice(t, cookie, "dev2")
	conn1 := h.DialWS(t, cookie, "dev1")
	conn2 := h.DialWS(t, cookie, "dev2")
	defer conn1.Close()
	defer conn2.Close()

	var wg sync.WaitGroup
	startChan := make(chan struct{})

	// goroutine A -> WSManager.Shutdown()
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-startChan
		h.App.Handlers.WSManager.Shutdown()
	}()

	// goroutine B -> WSManager.Shutdown()
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-startChan
		h.App.Handlers.WSManager.Shutdown()
	}()

	// goroutine C -> WSManager.Shutdown()
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-startChan
		h.App.Handlers.WSManager.Shutdown()
	}()

	// goroutine D -> App.Close()
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-startChan
		h.App.Close(context.Background())
	}()

	// goroutine E -> RelayService.Shutdown()
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-startChan
		h.App.Handlers.RelayService.Shutdown(context.Background())
	}()

	// Release the barrier
	close(startChan)

	// Wait for all to finish
	wg.Wait()
}
