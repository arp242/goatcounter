// Package cfg contains global application configuration settings.
package cfg

// Configuration variables.
var (
	CertDir      string
	Domain       string
	DomainStatic = []string{""}
	PgSQL        bool
	Plan         string
	Prod         bool
	Version      string
	Saas         bool
	Serve        bool
	Port         string
)
