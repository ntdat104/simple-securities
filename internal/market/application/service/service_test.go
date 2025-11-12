package service

import (
	"context"
	"fmt"
	"simple-securities/pkg/client"
	"sync"
	"testing"
	"time"
)

func TestPingSvc(t *testing.T) {
	// Create a new public client for the Binance API ping endpoint
	c := client.NewPublicClient("ping")

	// Create the Ping service
	pingSvc := NewPingSvc(c)

	// Execute the Ping request
	err := pingSvc.Execute(context.Background())
	if err == nil {
		fmt.Println("✅ Success: Ping responded correctly")
		return
	}

	// Print error details if something went wrong
	fmt.Println("❌ Error:", client.PrettyPrint(err))
	t.Fail()
}

func TestPingSvcSingleflight(t *testing.T) {
	// Create a new public client
	c := client.NewPublicClient("ping")

	// Create the Ping service
	pingSvc := NewPingSvc(c)

	var wg sync.WaitGroup
	numCalls := 10
	errCh := make(chan error, numCalls)
	start := time.Now()

	// Launch multiple concurrent calls
	for i := 0; i < numCalls; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := pingSvc.ExecuteSf(context.Background())
			errCh <- err
		}(i)
	}

	wg.Wait()
	close(errCh)

	var failed int
	for err := range errCh {
		if err != nil {
			failed++
			fmt.Println("❌ Error:", client.PrettyPrint(err))
		}
	}

	elapsed := time.Since(start)
	if failed == 0 {
		fmt.Printf("✅ Success: All %d concurrent calls completed successfully (took %v)\n", numCalls, elapsed)
	} else {
		t.Errorf("❌ %d/%d calls failed", failed, numCalls)
	}
}

// TestServerTimeSvc tests the /api/v3/time endpoint using the ServerTimeSvc
func TestServerTimeSvc(t *testing.T) {
	// Create a new public client
	c := client.NewPublicClient("servertime")

	// Create the ServerTime service
	serverTimeSvc := NewServerTimeSvc(c)

	// Execute the request
	res, err := serverTimeSvc.Execute(context.Background())
	if err != nil {
		fmt.Println("❌ Error:", client.PrettyPrint(err))
		t.FailNow()
	}

	// Verify response content
	if res == nil {
		t.Fatal("❌ Expected a valid ServerTimeResponse, got nil")
	}

	// Check that the returned server time is reasonable (greater than a recent timestamp)
	if res.ServerTime <= 0 {
		t.Fatalf("❌ Invalid server time: %d", res.ServerTime)
	}

	fmt.Printf("✅ Success: Binance server time is %d\n", res.ServerTime)
}
