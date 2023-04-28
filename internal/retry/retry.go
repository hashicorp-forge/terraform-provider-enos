package retry

import (
	"context"
	"errors"
	"math"
	"time"
)

// RetryInterval is a function that takes an integer that defines the attempt number.
// Given the number of attempts it will calculate and return the `time.Duration` that
// should be waited before attempting the next retry.
type RetryInterval func(int) time.Duration

var ErrNoRetryFunc = errors.New("no retry function has been set")

// Retry is a function that takes a Retryable interface and a context.
// It runs Run(), performing whatever action the caller has defined.
// If Run() returns an error, Retry determines whether it should retry using ShouldRetry(),
// and if it should, sleeps the appropriate duration as specified by Interval() before retrying.
func Retry(ctx context.Context, req Retryable) (interface{}, error) {
	var res interface{}
	var err error

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		res, err = req.Run(ctx)
		if err == nil {
			return res, nil
		}

		if !req.ShouldRetry() {
			return res, err
		}

		time.Sleep(req.Interval())
	}
}

// Retryable is an interface that allows the caller to specify the operation to be executed,
// the interval to wait between retries, and whether to retry or not.
type Retryable interface {
	Interval() time.Duration
	Run(ctx context.Context) (interface{}, error)
	ShouldRetry() bool
}

// Retrier is a struct that allows the caller to define their parameters for retrying an operation.
type Retrier struct {
	attempts int
	// Maximum number of retries (optional)
	MaxRetries int
	// Only retry on these errors (optional)
	OnlyRetryError []error
	// A function that returns a time.Duration for the next interval, based on current attempt number (optional)
	RetryInterval RetryInterval
	// The operation to be run (required)
	Func    func(context.Context) (interface{}, error)
	lastErr error
}

// RetrierOpt is a function that takes a pointer to a Retrier and returns that same pointer,
// allowing some actions to be performed upon the Retrier — for example, setting some of its fields.
type RetrierOpt func(*Retrier) *Retrier

// NewRetrier takes one or more RetrierOpt functions, creates a new Retrier, sets
// the fields specified in the RetrierOpts, and returns a pointer to the new Retrier.
// If it determines that no Func has been specified (the operation to be run or
// retried), it returns an error.
func NewRetrier(opts ...RetrierOpt) (*Retrier, error) {
	r := &Retrier{
		MaxRetries:     -1, // A negative MaxRetries value means unlimited retries
		attempts:       0,
		RetryInterval:  IntervalFibonacci(1 * time.Second),
		OnlyRetryError: []error{},
	}

	for _, opt := range opts {
		r = opt(r)
	}

	if r.Func == nil {
		return r, ErrNoRetryFunc
	}

	return r, nil
}

// WithRetrierFunc allows the caller to define the operation to be run/retried.
// It is required to create a new Retrier.
func WithRetrierFunc(f func(context.Context) (interface{}, error)) RetrierOpt {
	return func(r *Retrier) *Retrier {
		r.Func = f
		return r
	}
}

// WithMaxRetries allows the caller to define a max number of retries. It is optional.
func WithMaxRetries(retries int) RetrierOpt {
	return func(r *Retrier) *Retrier {
		r.MaxRetries = retries
		return r
	}
}

// WithOnlyRetryErrors allows the caller to define a list of errors upon which to retry.
// If one or more errors is provided, Retry() will only retry on those errors.
// It is optional.
func WithOnlyRetryErrors(err ...error) RetrierOpt {
	return func(r *Retrier) *Retrier {
		r.OnlyRetryError = err
		return r
	}
}

// WithIntervalFunc allows the caller to define a function to calculate the interval
// between retries (IntervalExponential and IntervalFibonacci are available for this
// purpose). It is optional.
func WithIntervalFunc(i RetryInterval) RetrierOpt {
	return func(r *Retrier) *Retrier {
		r.RetryInterval = i
		return r
	}
}

// Interval returns the RetryInterval function that has been set on the Retrier,
// passing it the current number of attempts so that the RetryInterval
// function can calculate the interval duration until the next retry.
func (r *Retrier) Interval() time.Duration {
	return r.RetryInterval(r.attempts)
}

// Run is a function that runs the operation that has been set on the Retrier,
// passing along the context and increasing the attempt number with each try.
func (r *Retrier) Run(ctx context.Context) (interface{}, error) {
	r.attempts++
	var res interface{}

	res, r.lastErr = r.Func(ctx)

	return res, r.lastErr
}

// ShouldRetry is a function that determines whether the operation should be retried.
// It checks whether the max number of retries has been reached. It also checks
// whether the caller has submitted any OnlyRetryErrors, in which case retries should
// only occur in the case of those errors.
func (r *Retrier) ShouldRetry() bool {
	// Default value for MaxRetries is -1, indicating that it has not been set by the caller.
	if r.MaxRetries > -1 && r.attempts >= r.MaxRetries {
		return false
	}

	// If the caller has submitted any OnlyRetryErrors, check the lastErr against them to see if we should retry.
	// If the caller has not submitted any OnlyRetryErrors, return true (retry for any error).
	if len(r.OnlyRetryError) > 0 {
		for _, err := range r.OnlyRetryError {
			if errors.Is(r.lastErr, err) {
				return true
			}
		}

		return false
	}

	return true
}

// IntervalExponential returns a function that calculates an interval duration for
// the next retry, based on the current number of attempts and the base number/unit.
// The intervals increase exponentially (^2) with each attempt.
func IntervalExponential(base time.Duration) RetryInterval {
	return func(attempt int) time.Duration {
		if attempt == 0 {
			return 0
		}

		return time.Duration(math.Pow(2, float64(attempt))) * base
	}
}

// IntervalFibonacci returns a function that calculates an interval duration for
// the next retry, based on the current number of attempts and the base number/unit.
// The intervals increase on the basis of the Fibonacci sequence with each attempt.
func IntervalFibonacci(base time.Duration) RetryInterval {
	return func(attempt int) time.Duration {
		fibSequence := make([]int, attempt+1, attempt+2)
		if attempt <= 1 {
			fibSequence = fibSequence[0:2]
		}
		fibSequence[0] = 0
		fibSequence[1] = 1
		for i := 2; i <= attempt; i++ {
			fibSequence[i] = fibSequence[i-1] + fibSequence[i-2]
		}

		return time.Duration(fibSequence[attempt]) * base
	}
}
