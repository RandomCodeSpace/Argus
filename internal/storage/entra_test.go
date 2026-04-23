package storage

import (
	"strings"
	"testing"
)

func TestSslmodeFromDSN(t *testing.T) {
	cases := []struct {
		name string
		dsn  string
		want string
	}{
		{"kv-require", "host=x user=y dbname=z sslmode=require", "require"},
		{"kv-disable", "host=x user=y sslmode=disable", "disable"},
		{"kv-prefer", "host=x user=y sslmode=prefer", "prefer"},
		{"kv-missing", "host=x user=y dbname=z", ""},
		{"kv-mixed-case", "HOST=x SSLMODE=VERIFY-FULL", "verify-full"},
		{"kv-with-tabs", "host=x\tsslmode=require", "require"},
		{"url-require", "postgres://u:p@h:5432/db?sslmode=require", "require"},
		{"url-none", "postgres://u:p@h:5432/db", ""},
		{"url-multi-params", "postgresql://u@h/db?application_name=me&sslmode=verify-ca&connect_timeout=5", "verify-ca"},
		{"url-mixed-case-param", "postgres://u@h/db?SslMode=Require", "require"},
		{"empty", "", ""},
		{"sslmode-as-host-substring", "host=sslmodeserver user=y", ""},
		{"quoted-value-not-supported", "host=x sslmode='require'", "'require'"}, // pgx accepts quotes; we strip conservatively
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := sslmodeFromDSN(c.dsn)
			if got != c.want {
				t.Fatalf("sslmodeFromDSN(%q) = %q, want %q", c.dsn, got, c.want)
			}
		})
	}
}

func TestOpenPostgresWithEntra_DSNValidation(t *testing.T) {
	cases := []struct {
		name        string
		dsn         string
		wantErrText string
	}{
		// NOTE: "user absent from DSN" is not reliably testable because pgx.ParseConfig
		// falls back to PGUSER env or the OS login user, which we can't fully isolate
		// in a unit test. The empty-user guard still fires in production when no OS
		// user resolves, or against deployment environments that explicitly clear PGUSER.
		{
			name:        "missing sslmode",
			dsn:         "host=mydb.postgres.database.azure.com user=mi@tenant dbname=otel",
			wantErrText: "requires explicit sslmode",
		},
		{
			name:        "sslmode disable",
			dsn:         "host=mydb.postgres.database.azure.com user=mi@tenant dbname=otel sslmode=disable",
			wantErrText: "sslmode=require|verify-ca|verify-full",
		},
		{
			name:        "sslmode prefer",
			dsn:         "host=mydb.postgres.database.azure.com user=mi@tenant dbname=otel sslmode=prefer",
			wantErrText: "sslmode=require|verify-ca|verify-full",
		},
		{
			name:        "sslmode allow",
			dsn:         "host=mydb.postgres.database.azure.com user=mi@tenant dbname=otel sslmode=allow",
			wantErrText: "sslmode=require|verify-ca|verify-full",
		},
		{
			name:        "completely malformed",
			dsn:         "not a valid dsn at all://",
			wantErrText: "parse postgres DSN",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := openPostgresWithEntra(c.dsn)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", c.wantErrText)
			}
			if !strings.Contains(err.Error(), c.wantErrText) {
				t.Fatalf("error %q should contain %q", err.Error(), c.wantErrText)
			}
		})
	}
}

func TestOpenPostgresWithEntra_AcceptsStrictSslmodes(t *testing.T) {
	// These DSNs pass validation and try to initialise credentials. azidentity may
	// return an error in an unauthenticated test environment — that's OK, we just
	// assert our pre-credential validation path accepts the DSN.
	strictModes := []string{"require", "verify-ca", "verify-full"}
	for _, mode := range strictModes {
		t.Run(mode, func(t *testing.T) {
			dsn := "host=mydb.postgres.database.azure.com user=mi@tenant dbname=otel sslmode=" + mode
			_, err := openPostgresWithEntra(dsn)
			if err != nil {
				// Acceptable failures: our own validation must NOT fire.
				bad := []string{
					"requires explicit sslmode",
					"sslmode=require|verify-ca|verify-full",
					"must specify user",
					"parse postgres DSN",
					"TLSConfig",
				}
				for _, b := range bad {
					if strings.Contains(err.Error(), b) {
						t.Fatalf("pre-credential validation should not reject %s: %v", mode, err)
					}
				}
				// Other errors (credential acquisition etc.) are outside test scope.
			}
		})
	}
}

func TestIsAzureEntraEnabled(t *testing.T) {
	cases := []struct {
		env  string
		want bool
	}{
		{"true", true},
		{"TRUE", true},
		{"1", true},
		{"yes", true},
		{"on", true},
		{"  true  ", true},
		{"false", false},
		{"0", false},
		{"no", false},
		{"off", false},
		{"", false},
		{"garbage", false},
	}
	for _, c := range cases {
		t.Run(c.env, func(t *testing.T) {
			t.Setenv("DB_AZURE_AUTH", c.env)
			if got := isAzureEntraEnabled(); got != c.want {
				t.Fatalf("DB_AZURE_AUTH=%q → %v, want %v", c.env, got, c.want)
			}
		})
	}
}
