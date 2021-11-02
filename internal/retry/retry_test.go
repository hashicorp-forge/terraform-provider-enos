package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	error1           = errors.New("this is error 1")
	error2           = errors.New("this is error 2")
	errorUnspecified = errors.New("this is not one of the specified errors")
)

// Test that when the first succeeds we don't retry
func TestRetry_SuccessfulFirstAttempt(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	var attempt int
	var res interface{}

	funcToRun := func(ctx context.Context) (interface{}, error) {
		attempt++

		var err error

		// We always want to succeed on the first attempt and not retry.
		return res, err
	}

	req, err := NewRetrier(
		WithOnlyRetryErrors(error1),
		WithIntervalFunc(IntervalFibonacci(1*time.Second)),
		WithRetrierFunc(funcToRun),
	)
	assert.Nil(t, err)

	_, errRetry := Retry(ctx, req)

	assert.NoError(t, errRetry)
	assert.Equal(t, attempt, 1)
}

// Test that when the first attempt fails, we retry
func TestRetry_RetryAfterFailedFirstAttempt(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	var attempt int

	funcToRun := func(ctx context.Context) (interface{}, error) {
		attempt++
		var res interface{}
		var err error

		// We want to fail on first try and succeed on second try
		if attempt == 1 {
			return res, errors.New("failing purposefully on first try")
		} else if attempt == 2 {
			return res, err
		} else {
			return res, errors.New("not the first or second try")
		}
	}

	req, _ := NewRetrier(
		WithIntervalFunc(IntervalFibonacci(1*time.Nanosecond)),
		WithRetrierFunc(funcToRun),
	)

	_, err := Retry(ctx, req)

	assert.NoError(t, err)
	assert.Equal(t, attempt, 2)
}

// Test that we do not exceed the max number of retries
func TestRetry_DoesntExceedMaxRetries(t *testing.T) {
	ctx := context.Background()

	var attempt int
	maxRetries := 2

	funcToRun := func(ctx context.Context) (interface{}, error) {
		attempt++
		var res interface{}

		// We want to fail on all attempts
		return res, errors.New("purposefully failing on all attempts")
	}

	req, _ := NewRetrier(
		WithMaxRetries(maxRetries),
		WithIntervalFunc(IntervalFibonacci(1*time.Nanosecond)),
		WithRetrierFunc(funcToRun),
	)

	var err error
	_, returnedErr := Retry(ctx, req)

	assert.Error(t, returnedErr, err)
	assert.Equal(t, attempt, maxRetries)
}

// Test that if OnlyRetryErrors are passed, only those errors trigger retry
func TestRetry_OnlyRetryOnSpecifiedErrors(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var attempt int

	funcToRun := func(ctx context.Context) (interface{}, error) {
		attempt++
		var res interface{}

		if attempt == 1 {
			return res, error1
		} else if attempt == 2 {
			return res, error2
		} else {
			return res, errorUnspecified
		}
	}

	req, _ := NewRetrier(
		WithOnlyRetryErrors(error1, error2),
		WithIntervalFunc(IntervalFibonacci(1*time.Nanosecond)),
		WithRetrierFunc(funcToRun),
	)

	_, err := Retry(ctx, req)

	assert.Equal(t, attempt, 3)
	assert.Equal(t, err, errorUnspecified)
}

// Test that if Timeout is set on the context, Retry times out accordingly
func TestRetry_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var attempt int
	var res interface{}

	funcToRun := func(ctx context.Context) (interface{}, error) {
		attempt++

		return res, error1
	}

	req, _ := NewRetrier(
		WithIntervalFunc(IntervalFibonacci(1*time.Nanosecond)),
		WithRetrierFunc(funcToRun),
	)

	_, err := Retry(ctx, req)

	assert.Equal(t, err.Error(), "context deadline exceeded")
}

// Test that the retry function returns the value that we expect
func TestRetry_CorrectValueReturned(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	funcToRun := func(ctx context.Context) (interface{}, error) {
		var valueToReturn interface{}
		var err error

		valueToReturn = 1
		return valueToReturn, err
	}

	req, _ := NewRetrier(
		WithIntervalFunc(IntervalFibonacci(1*time.Nanosecond)),
		WithRetrierFunc(funcToRun),
	)

	res, _ := Retry(ctx, req)
	assert.Equal(t, 1, res.(int))
}

// Test that IntervalExponential returns the mathematically correct value
func TestRetry_IntervalExponential(t *testing.T) {
	for _, testObj := range []struct {
		inputNum       int
		expectedResult time.Duration
	}{
		{0, time.Duration(0)},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 16 * time.Second},
		{5, 32 * time.Second},
		{6, 64 * time.Second},
		{7, 128 * time.Second},
		{8, 256 * time.Second},
	} {
		generatedResult := IntervalExponential(1 * time.Second)(testObj.inputNum)
		assert.Equal(t, generatedResult, testObj.expectedResult)
	}
}

// Test that IntervalFibonacci returns the mathematically correct value
func TestRetry_IntervalFibonacci(t *testing.T) {
	for _, testObj := range []struct {
		inputNum       int
		expectedResult time.Duration
	}{
		{0, 0 * time.Second},
		{1, 1 * time.Second},
		{2, 1 * time.Second},
		{3, 2 * time.Second},
		{4, 3 * time.Second},
		{5, 5 * time.Second},
		{6, 8 * time.Second},
		{7, 13 * time.Second},
		{8, 21 * time.Second},
	} {
		generatedResult := IntervalFibonacci(1 * time.Second)(testObj.inputNum)
		assert.Equal(t, generatedResult, testObj.expectedResult)
	}
}
