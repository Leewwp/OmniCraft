package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDefaultConfigDeclaresArchiveScanSkeleton pins the archive malware
// scanning config skeleton (S01, issue #146): the feature flag defaults to
// off, and the archive_scan.* quotas/timeouts/retry schedule carry the spec
// §4 initial values. The skeleton is server-side only: it is not part of the
// public config response.
func TestDefaultConfigDeclaresArchiveScanSkeleton(t *testing.T) {
	cfg := loadDefaultConfigForTest(t)

	require.False(t, cfg.Features.ArchiveMalwareScanEnabled)

	require.Equal(t, 500, cfg.ArchiveScan.MaxUploadSizeMB)
	require.Equal(t, 5000, cfg.ArchiveScan.MaxZipEntries)
	require.Equal(t, 200, cfg.ArchiveScan.MaxEntryUncompressedMB)
	require.Equal(t, 2048, cfg.ArchiveScan.MaxTotalUncompressedMB)
	require.Equal(t, 10, cfg.ArchiveScan.MaxRecursionDepth)
	require.Equal(t, 120, cfg.ArchiveScan.ScanTimeoutSec)
	require.Equal(t, "127.0.0.1:3310", cfg.ArchiveScan.ClamdAddress)
	require.Equal(t, []int{60, 300, 1800}, cfg.ArchiveScan.RetryBackoffSec)
	require.Equal(t, 300, cfg.ArchiveScan.URLTTLSec)
}

// TestValidateReleaseRejectsInvalidArchiveScanQuotas pins the release-mode
// guard that applies once the feature is enabled: quotas, timeout and retry
// schedule must be fully configured.
func TestValidateReleaseRejectsInvalidArchiveScanQuotas(t *testing.T) {
	cfg := &Config{
		Server:   ServerConfig{Mode: "release"},
		Features: FeaturesConfig{ArchiveMalwareScanEnabled: true},
		ArchiveScan: ArchiveScanConfig{
			MaxUploadSizeMB: 500, MaxZipEntries: 5000, MaxEntryUncompressedMB: 200,
			MaxTotalUncompressedMB: 2048, MaxRecursionDepth: 10, ScanTimeoutSec: 120,
		},
	}

	err := cfg.ValidateRelease()
	require.Error(t, err)
	require.Contains(t, err.Error(), "archive_scan.clamd_address")
	require.Contains(t, err.Error(), "archive_scan.retry_backoff_sec")
}
