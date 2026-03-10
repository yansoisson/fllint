package download

// DiskInfo holds available and total disk space for a filesystem path.
type DiskInfo struct {
	AvailableBytes int64
	TotalBytes     int64
}
