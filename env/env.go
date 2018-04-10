package env

import "os"

func GitHubToken() string {
	return os.Getenv("METAGODOC_GITHUB_TOKEN")
}

func Root() string {
	root := os.Getenv("METAGODOC_ROOT")
	if root != "" {
		return root
	}

	return "/var/cache/metagodoc"
}

func TraceElastic() bool {
	return os.Getenv("METAGODOC_TRACE_ELASTIC") != ""
}

func IsProd() bool {
	return os.Getenv("METAGODOC_PRODUCTION") != ""
}
