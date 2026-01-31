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
	StateDir                string
	IdleTimeout             time.Duration
	BufferBytes             int
	TranscriptMaxBytes      int64
	TranscriptTrimThreshold int64
	TranscriptTailBytes     int64
	SnapshotInterval        time.Duration
	HistoryLines            int
	RecordPty               bool
	StreamCreditTimeout     time.Duration
	StreamInitialCredit     int64
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
		SnapshotInterval:        2 * time.Second,
		HistoryLines:            4000,
		RecordPty:               false,
		StreamCreditTimeout:     30 * time.Second,
		StreamInitialCredit:     256 * 1024,
	}
}
