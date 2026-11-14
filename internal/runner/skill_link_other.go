//go:build !windows

package runner

func createSkillLinkFallback(source, destination string, original error) error { return original }
