package repo

type RepoError string

func (e RepoError) Error() string {
	return string(e)
}

var (
	ErrNotFound = RepoError("entity not found")
)
