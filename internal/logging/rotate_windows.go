package logging

// Windows does not support SIGHUP; its log lifecycle is unchanged.
func watchRotation(file *rotatingFile) func() { return func() {} }
