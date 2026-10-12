package observability

import (
	"log/slog"
	"os"
)

func Setup() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, nil))
}
