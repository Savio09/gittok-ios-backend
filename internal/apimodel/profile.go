package apimodel

// Identity is who the profile belongs to. All fields are optional: a reader can
// use the feed before connecting a GitHub account.
type Identity struct {
	DisplayName string `json:"display_name,omitempty"`
	GitHubLogin string `json:"github_login,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

// LearnMix is how much unfamiliar tech the reader wants per project.
type LearnMix string

const (
	MixMostlyFamiliar LearnMix = "mostly_familiar"
	MixAMix           LearnMix = "a_mix"
	MixMostlyNew      LearnMix = "mostly_new"
)

// Valid reports whether m is a known learn mix. Empty is valid (unset).
func (m LearnMix) Valid() bool {
	switch m {
	case "", MixMostlyFamiliar, MixAMix, MixMostlyNew:
		return true
	default:
		return false
	}
}

// Experience is the reader's open-source contribution history.
type Experience string

const (
	ExpNever         Experience = "never"          // never contributed
	ExpSmallPRs      Experience = "small_prs"      // a few small PRs (docs, typos)
	ExpMergedChanges Experience = "merged_changes" // merged code changes
	ExpMaintainer    Experience = "maintainer"     // maintains a project
)

// Valid reports whether e is a known experience level. Empty is valid (unset).
func (e Experience) Valid() bool {
	switch e {
	case "", ExpNever, ExpSmallPRs, ExpMergedChanges, ExpMaintainer:
		return true
	default:
		return false
	}
}

// TimePerWeek is how much time the reader can give per week.
type TimePerWeek string

const (
	TimeUnder2h TimePerWeek = "under_2h"
	Time2to5h   TimePerWeek = "2_to_5h"
	Time5hPlus  TimePerWeek = "5h_plus"
)

// Valid reports whether t is a known time band. Empty is valid (unset).
func (t TimePerWeek) Valid() bool {
	switch t {
	case "", TimeUnder2h, Time2to5h, Time5hPlus:
		return true
	default:
		return false
	}
}

// Profile is the reader's personalization input, set during onboarding and
// editable later. The feed is ranked from it (and, later, from Events). Known
// and Learning feed the "you know" / "you want to learn" signals on previews;
// Domains and ProjectTypes bias selection.
type Profile struct {
	Identity     Identity    `json:"identity"`
	Known        []string    `json:"known"`    // languages/frameworks worked with
	Learning     []string    `json:"learning"` // want to learn
	Mix          LearnMix    `json:"mix,omitempty"`
	Domains      []string    `json:"domains"`
	ProjectTypes []string    `json:"project_types"`
	Experience   Experience  `json:"experience,omitempty"`
	Goals        []string    `json:"goals"`
	TimePerWeek  TimePerWeek `json:"time_per_week,omitempty"`
}
