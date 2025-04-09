package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Global variable to store the time taken for the normal call
var normalCallDuration time.Duration

// TestTagsHandlerNormalCall tests the normal execution of the /tags endpoint
func TestTagsHandlerNormalCall(t *testing.T) {
	// Create a new HTTP request to the /tags endpoint
	req, err := http.NewRequest("GET", "/tags", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Create a ResponseRecorder to capture the response
	rr := httptest.NewRecorder()

	// Create a handler for the /tags endpoint
	handler := http.HandlerFunc(tagsHandler)

	// Track the start time of the request
	startTime := time.Now()

	// Start the request
	handler.ServeHTTP(rr, req)

	// Measure the time taken for the request
	duration := time.Since(startTime)

	// Store the duration in the global variable for the second test
	normalCallDuration = duration

	// Check the response code (200 OK)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %v", status)
	}

	// Check if the response body starts with expected JSON format
	expectedPrefix := `{"tags":[`
	if !startsWith(rr.Body.String(), expectedPrefix) {
		t.Errorf("Expected body to start with '%s', got '%s'", expectedPrefix, rr.Body.String())
	}
}

// TestTagsHandlerCancelCallHalfTime tests the cancellation of the /tags endpoint call at half time
func TestTagsHandlerCancelCallHalfTime(t *testing.T) {

	// Ensure that normal test has run and set the duration
	if normalCallDuration == 0 {
		// If normalDuration is not set, run the normal test first
		t.Log("Normal call duration not set. Running TestTagsHandlerNormalCall.")
		TestTagsHandlerNormalCall(t) // Run the normal test to set normalDuration
	}

	// Use the duration of the first test to calculate half-time
	duration := normalCallDuration

	// Create a cancellable context for the request
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure cancellation is called at the end of the test

	// Create a new HTTP request to the /tags endpoint with the cancellable context
	req, err := http.NewRequestWithContext(ctx, "GET", "/tags", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Create a ResponseRecorder to capture the response
	rr := httptest.NewRecorder()

	// Create a handler for the /tags endpoint
	handler := http.HandlerFunc(tagsHandler)

	// Start the request in a goroutine so we can cancel it later
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(rr, req)
		close(done)
	}()

	// Calculate the half-time to cancel the request
	halfTime := duration / 2

	// Wait for half of the expected duration
	time.Sleep(halfTime)

	// Cancel the context to simulate the cancellation after half the time
	cancel()

	// Wait for the goroutine to finish
	<-done

	// Check that the response is not empty
	if rr.Body.Len() == 0 {
		t.Errorf("Expected response body, got empty body")
	}

	// Ensure that there is no error in the status code (since canceling does not result in an error)
	if rr.Code != http.StatusOK && rr.Code != http.StatusRequestTimeout {
		t.Errorf("Expected status OK or RequestTimeout, got %v", rr.Code)
	}
}

// startsWith checks if the given string starts with the expected prefix
func startsWith(str, prefix string) bool {
	return len(str) >= len(prefix) && str[:len(prefix)] == prefix
}
