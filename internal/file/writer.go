package file

type Writer interface {
	Write(data any) (int, error)
}
