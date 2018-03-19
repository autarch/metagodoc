package logger

import (
	"fmt"
	"log"
	"os"
	"strconv"
)

const maxPrefix = 24

func New(prefix string, withFile bool) *log.Logger {
	flags := log.Ldate | log.Ltime
	if withFile {
		flags |= log.Llongfile
	}
	if len(prefix) > maxPrefix {
		prefix = prefix[0 : maxPrefix-1]
	}

	return log.New(os.Stdout, fmtPrefix(prefix), flags)
}

var prefixFmt string = "%-" + strconv.Itoa(maxPrefix) + "s"

func fmtPrefix(prefix string) string {
	return fmt.Sprintf(prefixFmt+": ", prefix)
}
