package pkgb

// Storage defines the contract for data persistence
type Storage interface {
	Save(data string) error
	Load() (string, error)
}

// B depends on Storage interface
type B struct {
	Store Storage
}
