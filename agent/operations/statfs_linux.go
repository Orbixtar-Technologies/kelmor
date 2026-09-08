package operations

import "syscall"

func statfs(path string) (diskStat, error) {
	var s syscall.Statfs_t
	if err := syscall.Statfs(path, &s); err != nil {
		return diskStat{}, err
	}
	total := uint64(s.Blocks) * uint64(s.Bsize)
	free := uint64(s.Bavail) * uint64(s.Bsize)
	return diskStat{
		total:  total,
		used:   total - free,
		itotal: uint64(s.Files),
		iused:  uint64(s.Files) - uint64(s.Ffree),
	}, nil
}
