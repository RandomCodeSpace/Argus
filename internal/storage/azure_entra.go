package storage

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// entraTokenTimeout bounds token acquisition. sql.DB.Ping and pool-refill paths
// pass context.Background(); without a timeout a stalled Azure IMDS endpoint
// would block every new connection indefinitely.
const entraTokenTimeout = 30 * time.Second

// entraPostgresScope is the AAD scope for Azure Database for PostgreSQL (Flexible Server + Single Server).
const entraPostgresScope = "https://ossrdbms-aad.database.windows.net/.default"

// isAzureEntraEnabled reports whether DB_AZURE_AUTH selects Entra ID authentication.
func isAzureEntraEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("DB_AZURE_AUTH"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// openPostgresWithEntra returns a *sql.DB where each new physical connection
// is authenticated with a fresh short-lived Entra ID access token as the password.
// The DSN should include user (Entra principal name or managed-identity display name),
// host, port, dbname, sslmode — but MUST NOT include a static password.
// DefaultAzureCredential resolves the identity in this order: env vars → workload identity →
// managed identity → Azure CLI → dev tools. Tokens are cached by the credential itself.
func openPostgresWithEntra(dsn string) (*sql.DB, error) {
	base, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse postgres DSN: %w", err)
	}
	if base.User == "" {
		return nil, fmt.Errorf("postgres DSN must specify user (Entra principal) when DB_AZURE_AUTH=true")
	}
	// Azure Database for PostgreSQL requires TLS with verification. Parsing alone is not
	// sufficient because sslmode=prefer/allow produce a non-nil TLSConfig yet downgrade
	// silently if the server doesn't offer TLS. Require an explicit strict mode.
	sslmode := sslmodeFromDSN(dsn)
	switch sslmode {
	case "require", "verify-ca", "verify-full":
		// acceptable
	case "":
		return nil, fmt.Errorf("DB_AZURE_AUTH=true requires explicit sslmode=require|verify-ca|verify-full in DB_DSN")
	default:
		return nil, fmt.Errorf("DB_AZURE_AUTH=true requires sslmode=require|verify-ca|verify-full, got sslmode=%s", sslmode)
	}
	if base.TLSConfig == nil {
		return nil, fmt.Errorf("postgres DSN parsed with nil TLSConfig despite sslmode=%s", sslmode)
	}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("create azure credential: %w", err)
	}
	return sql.OpenDB(&entraConnector{base: base, cred: cred}), nil
}

// sslmodeFromDSN returns the sslmode parameter from a Postgres DSN.
// Handles both key=value and postgres://host?sslmode=... forms. Returns "" if unset.
func sslmodeFromDSN(dsn string) string {
	lower := strings.ToLower(dsn)
	// URL form: look in query string
	if strings.Contains(lower, "://") {
		q := lower
		if i := strings.Index(q, "?"); i >= 0 {
			q = q[i+1:]
		}
		for _, part := range strings.Split(q, "&") {
			k, v, ok := strings.Cut(part, "=")
			if ok && strings.TrimSpace(k) == "sslmode" {
				return strings.TrimSpace(v)
			}
		}
		return ""
	}
	// Key-value form: whitespace-separated tokens
	for _, part := range strings.Fields(lower) {
		k, v, ok := strings.Cut(part, "=")
		if ok && k == "sslmode" {
			return v
		}
	}
	return ""
}

// entraConnector is a database/sql connector that injects a fresh Entra token
// as the Postgres password on every Connect() call.
type entraConnector struct {
	base *pgx.ConnConfig
	cred azcore.TokenCredential
}

func (c *entraConnector) Connect(ctx context.Context) (driver.Conn, error) {
	tokCtx, cancel := context.WithTimeout(ctx, entraTokenTimeout)
	defer cancel()
	tok, err := c.cred.GetToken(tokCtx, policy.TokenRequestOptions{Scopes: []string{entraPostgresScope}})
	if err != nil {
		return nil, fmt.Errorf("acquire entra token: %w", err)
	}
	cfg := c.base.Copy()
	cfg.Password = tok.Token
	return stdlib.GetConnector(*cfg).Connect(ctx)
}

func (c *entraConnector) Driver() driver.Driver {
	return stdlib.GetDefaultDriver()
}
