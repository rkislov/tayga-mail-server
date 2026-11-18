package chat

import "golang.org/x/sys/windows"

func attachmentFreeBytes(dir string) (uint64, error) {
	path, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return 0, err
	}
	var available uint64
	err = windows.GetDiskFreeSpaceEx(path, &available, nil, nil)
	return available, err
}

// Windows does not expose a directory fsync through os.File.
func syncAttachmentDir(string) error { return nil }
