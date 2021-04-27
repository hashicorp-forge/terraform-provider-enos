package random

import (
	"math/rand"
	"os"
	"strconv"
	"time"
)

// ID returns a random ID
func ID() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano() * int64(os.Getpid())))
	return strconv.FormatInt(int64(r.Int31()), 10)
}
