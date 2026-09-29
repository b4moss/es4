package recovery

import (
	"context"
	"fmt"

	"github.com/b4moss/es4/packages/go/pkg/options"
)

// OpenFromOptions builds a Recovery Store from Effective Options.
// Returns (nil, nil) when no Recovery backend is configured.
// Caller should Validate opts first (or via Open).
func OpenFromOptions(ctx context.Context, opts options.Options) (Store, error) {
	backend := opts.ResolvedRecoveryBackend()
	if backend == "" {
		return nil, nil
	}
	switch backend {
	case options.RecoveryBackendFile:
		return NewFileTTL(opts.RecoveryPath, opts.RecoveryTTL), nil
	case options.RecoveryBackendLibSQL:
		return OpenLibSQL(opts.RecoveryLibSQLURL, opts.RecoveryLibSQLAuthToken, opts.RecoveryTTL)
	case options.RecoveryBackendObject:
		return OpenObject(ctx, ObjectConfig{
			Bucket:   opts.RecoveryS3Bucket,
			Prefix:   opts.RecoveryS3Prefix,
			Region:   opts.RecoveryS3Region,
			Endpoint: opts.RecoveryS3Endpoint,
			TTL:      opts.RecoveryTTL,
		})
	default:
		return nil, fmt.Errorf("recovery: unknown backend %q", backend)
	}
}
