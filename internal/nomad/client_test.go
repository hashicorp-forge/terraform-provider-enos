package nomad

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetTaskLogsResponse_GetLogFileName(t *testing.T) {
	r := &GetTaskLogsResponse{
		Namespace:  "taco-truck",
		Allocation: "make.taco[0]",
		Task:       "chicken",
		Logs:       []byte{},
	}
	logFile := r.GetLogFileName()

	assert.Equal(t, "taco-truck_make.taco[0]_chicken.log", logFile)
}