// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package remoteflight

// GetLogsResponse interface defining the functions required for any get logs request response.
type GetLogsResponse interface {
	// GetAppName gets the name of the application that the logs pertain to.
	GetAppName() string
	// GetLogFileName creates a unique log file name.
	GetLogFileName() string
	// GetLogs gets the logs that were retrieved by a log file request
	GetLogs() []byte
}
