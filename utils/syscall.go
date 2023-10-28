package utils

import (
	"fmt"
	"syscall"
)

// SetMaxFdLimit sets the maximum number of file descriptors that can be opened by this process.
func SetMaxFdLimit() (uint64, error) {
	var rlimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlimit); err != nil {
		return 0, fmt.Errorf("failed to get rlimit: %v", err)
	}
	rlimit.Cur = rlimit.Max
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rlimit); err != nil {
		return 0, fmt.Errorf("failed to set rlimit: %v", err)
	}
	return rlimit.Cur, nil
}
