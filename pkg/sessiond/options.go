package sessiond

import (
	"log"
	"time"

	"github.com/strantalis/workset/pkg/unifiedlog"
)

type Options struct {
	SocketPath              string
	TranscriptDir           string
	RecordDir               string
	IdleTimeout             time.Duration
	IdleTimeoutSet          bool
	BufferBytes             int
	TranscriptMaxBytes      int64
	TranscriptTrimThreshold int64
	TranscriptTailBytes     int64
	RecordPty               bool
	Logger                  *log.Logger
	ProtocolLogEnabled      bool
	ProtocolLogDir          string
	ProtocolLogger          *unifiedlog.Logger
}

func DefaultOptions() Options {
	return Options{
		IdleTimeout:             30 * time.Minute,
		BufferBytes:             512 * 1024,
		TranscriptMaxBytes:      5 * 1024 * 1024,
		TranscriptTrimThreshold: 6 * 1024 * 1024,
		TranscriptTailBytes:     512 * 1024,
		RecordPty:               false,
	}
}
