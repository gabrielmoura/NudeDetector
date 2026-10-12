package cache

type Entry struct {
	Drawings float64
	Hentai   float64
	Neutral  float64
	Porn     float64
	Sexy     float64
}

type Cache interface {
	Get(key string) (*Entry, bool)
	Set(key string, e *Entry)
}
