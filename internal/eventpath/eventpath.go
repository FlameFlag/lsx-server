package eventpath

import (
	"net/url"
	"slices"
)

func Values(path string) url.Values {
	parsed, err := url.Parse(path)
	if err != nil {
		return url.Values{}
	}
	return parsed.Query()
}

func Route(path string) string {
	parsed, err := url.Parse(path)
	if err != nil || parsed.Path == "" {
		return path
	}
	return parsed.Path
}

func CloneValues(values url.Values) url.Values {
	next := make(url.Values, len(values))
	for key, value := range values {
		next[key] = slices.Clone(value)
	}
	return next
}
