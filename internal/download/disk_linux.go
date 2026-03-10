package download

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// CheckSpace returns available disk space at the given path.
func CheckSpace(path string) (*DiskInfo, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return nil, fmt.Errorf("check disk space: %w", err)
	}
	return &DiskInfo{
		AvailableBytes: int64(stat.Bavail) * int64(stat.Bsize),
		TotalBytes:     int64(stat.Blocks) * int64(stat.Bsize),
	}, nil
}
