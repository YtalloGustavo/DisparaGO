package logger

import (
	"log"
	"os"

	"disparago/internal/config"
)

func New(cfg config.Config) *log.Logger {
	prefix := "[" + cfg.App.Name + "] "
	return log.New(os.Stdout, prefix, log.Ldate|log.Ltime|log.LUTC|log.Lshortfile)
}
