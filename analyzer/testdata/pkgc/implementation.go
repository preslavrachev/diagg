package pkgc

// C implicitly implements pkgb.Storage interface
type C struct {
	FilePath string
}

func (c *C) Save(data string) error {
	// implementation
	return nil
}

func (c *C) Load() (string, error) {
	// implementation
	return "", nil
}
