package remoteflight

// GetLogsResponse interface defining the functions required for any get logs request response.
type GetLogsResponse interface {
	// GetLogFileName given a prefix, creates a log file name that should be unique.
	GetLogFileName(prefix string) string
	// GetLogs gets the logs that were retrieved by a log file request
	GetLogs() []byte
}
