// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package systemd

import (
	"fmt"

	"github.com/hashicorp/enos-provider/internal/remoteflight"
)

type GetUnitJournalRequest struct {
	Unit string
	Host string
}

type GetUnitJournalResponse struct {
	Unit string
	Host string
	Logs []byte
}

var _ remoteflight.GetLogsResponse = (*GetUnitJournalResponse)(nil)

// GetAppName implements remoteflight.GetUnitJournalResponse.
func (s *GetUnitJournalResponse) GetAppName() string {
	return s.Unit
}

func (s *GetUnitJournalResponse) GetLogFileName() string {
	return fmt.Sprintf("%s_%s.log", s.Unit, s.Host)
}

func (s *GetUnitJournalResponse) GetLogs() []byte {
	return s.Logs
}
