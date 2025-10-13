package application

type Error string

func (e Error) Error() string {
	return string(e)
}

const (
	ErrMissingParameters    = Error("missing parameters")
	ErrDatabaseNotAvailable = Error("database not available")
)
