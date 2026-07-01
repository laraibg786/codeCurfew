package config

type (
	Config struct {
		Addr    string
		Logging LoggingConfig
		// Provider is the name of the VCS provider to use, looked up in the
		// provider registry at startup. It defaults to defaultProvider ("github")
		// so existing deployments keep working with zero configuration changes.
		Provider string
	}

	LoggingConfig struct {
		File    string
		Verbose bool
	}
)

const (
	defaultAddr     = "0.0.0.0:8080"
	defaultProvider = "github"
	// envProvider is the environment variable that selects the VCS provider. It
	// seeds the -provider flag's default so either mechanism works: set PROVIDER
	// in a deployment, or pass -provider on the command line to override it.
	envProvider = "PROVIDER"
)
