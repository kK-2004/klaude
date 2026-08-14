package sandbox

import (
	"strconv"
	"strings"
)

// Outcome classifies a finished confined command.
type Outcome string

const (
	OutcomeOK            Outcome = "ok"
	OutcomeDenied        Outcome = "sandbox_denied"
	OutcomeRunnerFailed  Outcome = "sandbox_runner_failed"
	OutcomeCommandFailed Outcome = "command_failed"
)

// Classify inspects exit/stderr using the dialect returned by Confine.
func Classify(exitCode int, stderr string, confined Confined) Outcome {
	if exitCode == 0 {
		return OutcomeOK
	}
	lines := splitLines(stderr)
	if matchRunnerFailure(exitCode, lines, confined.RunnerFailRules) {
		return OutcomeRunnerFailed
	}
	lowered := strings.ToLower(stderr)
	for _, signature := range confined.DenialSignatures {
		if signature != "" && strings.Contains(lowered, strings.ToLower(signature)) {
			return OutcomeDenied
		}
	}
	return OutcomeCommandFailed
}

func matchRunnerFailure(exitCode int, lines []string, rules []RunnerFailureRule) bool {
	for _, rule := range rules {
		if len(rule.AllowedExitCodes) > 0 && !containsInt(rule.AllowedExitCodes, exitCode) {
			continue
		}
		filtered := filterInformational(lines, rule.InformationalLines)
		for _, line := range filtered {
			lower := strings.ToLower(line)
			for _, signature := range rule.FatalSignatures {
				if signature != "" && strings.Contains(lower, strings.ToLower(signature)) {
					return true
				}
			}
		}
	}
	return false
}

func filterInformational(lines, informational []string) []string {
	if len(informational) == 0 {
		return lines
	}
	skip := make(map[string]struct{}, len(informational))
	for _, line := range informational {
		skip[strings.ToLower(strings.TrimSpace(line))] = struct{}{}
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if _, ok := skip[strings.ToLower(strings.TrimSpace(line))]; ok {
			continue
		}
		out = append(out, line)
	}
	return out
}

func splitLines(value string) []string {
	if value == "" {
		return nil
	}
	raw := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

func containsInt(values []int, want int) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// DenialMarker is appended to tool results when OutcomeDenied.
func DenialMarker(mode Mode) string {
	return "[sandbox: file access denied under " + string(mode) + " mode]"
}

// FormatExit helps tests and diagnostics.
func FormatExit(code int) string {
	return strconv.Itoa(code)
}
