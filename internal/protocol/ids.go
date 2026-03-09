package protocol

import (
	"fmt"
	"sync/atomic"
	"time"
)

var idCounter atomic.Uint64

func MakeID() string {
	n := idCounter.Add(1)
	return fmt.Sprintf("msg_%d_%d", time.Now().UnixMicro(), n)
}
