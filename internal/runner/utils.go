package runner

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
