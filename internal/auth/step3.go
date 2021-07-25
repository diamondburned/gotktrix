package auth

import "github.com/diamondburned/gotktrix/internal/components/assistant"

func loginStep(a *Assistant) *assistant.Step {
	step := assistant.NewStep("Log in", "Login")
	return step
}
