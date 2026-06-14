package setup

import (
	"fmt"
	"slices"
	"strings"

	"github.com/davidtobonm/heracles/internal/agent"
	"github.com/davidtobonm/heracles/internal/doctor"
	"github.com/davidtobonm/heracles/internal/project"
)

// Availability records whether a provider's CLI executable was found on PATH.
// Detection never starts a paid agent session.
type Availability struct {
	Provider  string
	Available bool
}

// curatedModels offers a starting point per provider; "Other" accepts any model ID.
var curatedModels = map[string][]string{
	"codex":    {"gpt-5.4", "gpt-5.5"},
	"claude":   {"sonnet", "opus", "haiku"},
	"opencode": {"opencode-go/kimi-k2.6", "anthropic/claude-sonnet"},
	"kimi":     {"kimi-k2.6"},
}

const (
	optionOtherModel = "Other (enter a model ID)"
	optionUnset      = "Unset (provider default)"
)

// DetectProviders checks which registered provider executables are on PATH.
func DetectProviders(registry agent.Registry, system doctor.System) []Availability {
	names := registry.Names()
	availability := make([]Availability, 0, len(names))
	for _, name := range names {
		adapter, err := registry.Adapter(name)
		if err != nil {
			continue
		}
		_, lookErr := system.LookPath(adapter.Executable())
		availability = append(availability, Availability{Provider: name, Available: lookErr == nil})
	}
	return availability
}

// ChooseProfile prompts for a provider and its model/effort/variant settings.
func ChooseProfile(io IO, registry agent.Registry, availability []Availability, label string, current project.ProfileConfig) (project.ProfileConfig, error) {
	names := registry.Names()
	options := make([]string, len(names))
	currentIndex := 0
	for index, name := range names {
		option := name
		if !providerAvailable(availability, name) {
			option += " (not installed)"
		}
		if strings.EqualFold(name, current.Provider) {
			currentIndex = index
		}
		options[index] = option
	}

	selected, err := SelectOption(io, fmt.Sprintf("%s provider", label), options, currentIndex)
	if err != nil {
		return project.ProfileConfig{}, err
	}
	provider := names[selected]

	if !strings.EqualFold(provider, current.Provider) {
		current = project.ProfileConfig{}
	}
	profile := project.ProfileConfig{Provider: provider}

	adapter, err := registry.Adapter(provider)
	if err != nil {
		return project.ProfileConfig{}, err
	}
	capabilities := adapter.Capabilities()

	if capabilities.Model {
		model, err := chooseModel(io, label, provider, current.Model)
		if err != nil {
			return project.ProfileConfig{}, err
		}
		profile.Model = model
	}

	switch {
	case len(capabilities.Efforts) > 0:
		effort, err := chooseEffort(io, label, capabilities.Efforts, current.Effort)
		if err != nil {
			return project.ProfileConfig{}, err
		}
		profile.Effort = effort
	case capabilities.Variant:
		variant, err := Text(io, fmt.Sprintf("%s variant (leave blank for provider default)", label), current.Variant)
		if err != nil {
			return project.ProfileConfig{}, err
		}
		profile.Variant = variant
	}

	return profile, nil
}

func chooseModel(io IO, label, provider, current string) (string, error) {
	models := curatedModels[provider]
	options := append(append([]string(nil), models...), optionOtherModel, optionUnset)

	currentIndex := len(options) - 1
	switch {
	case current == "":
		currentIndex = len(options) - 1
	case slices.Contains(models, current):
		currentIndex = slices.Index(models, current)
	default:
		currentIndex = len(options) - 2
	}

	selected, err := SelectOption(io, fmt.Sprintf("%s model (%s)", label, provider), options, currentIndex)
	if err != nil {
		return "", err
	}
	switch {
	case selected == len(options)-1:
		return "", nil
	case selected == len(options)-2:
		defaultValue := ""
		if !slices.Contains(models, current) {
			defaultValue = current
		}
		return Text(io, fmt.Sprintf("%s model ID", label), defaultValue)
	default:
		return options[selected], nil
	}
}

func chooseEffort(io IO, label string, efforts []string, current string) (string, error) {
	options := append(append([]string(nil), efforts...), optionUnset)
	currentIndex := len(options) - 1
	if index := slices.Index(efforts, current); index >= 0 {
		currentIndex = index
	}

	selected, err := SelectOption(io, fmt.Sprintf("%s effort", label), options, currentIndex)
	if err != nil {
		return "", err
	}
	if selected == len(options)-1 {
		return "", nil
	}
	return options[selected], nil
}

func providerAvailable(availability []Availability, provider string) bool {
	for _, entry := range availability {
		if strings.EqualFold(entry.Provider, provider) {
			return entry.Available
		}
	}
	return false
}
