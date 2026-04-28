package security

import (
	"context"
	"fmt"
	"io"

	"github.com/dutchcoders/go-clamd"
)

// AvScanner defines the interface for antivirus scanning.
type AvScanner interface {
	Scan(ctx context.Context, r io.Reader) error
}

// ClamAVScanner implements AvScanner using a ClamAV daemon.
type ClamAVScanner struct {
	address string
}

func NewClamAVScanner(address string) *ClamAVScanner {
	return &ClamAVScanner{address: address}
}

func (s *ClamAVScanner) Scan(ctx context.Context, r io.Reader) error {
	c := clamd.NewClamd(s.address)
	
	// Check if clamd is available
	if err := c.Ping(); err != nil {
		return fmt.Errorf("antivirus service unavailable: %w", err)
	}

	response, err := c.ScanStream(r, make(chan bool))
	if err != nil {
		return fmt.Errorf("antivirus scan failed: %w", err)
	}

	for res := range response {
		if res.Status == clamd.RES_FOUND {
			return fmt.Errorf("security threat detected: %s", res.Description)
		}
		if res.Status == clamd.RES_ERROR {
			return fmt.Errorf("antivirus engine error: %s", res.Description)
		}
	}

	return nil
}

// NoopScanner is a placeholder for environments without ClamAV.
type NoopScanner struct{}

func (s *NoopScanner) Scan(ctx context.Context, r io.Reader) error {
	return nil
}
