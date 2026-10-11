package traffic

import (
	"slices"
	"strings"
	"sync"

	"github.com/mrlm-net/simconnect/pkg/dict"
)

// Model providers: where our traffic's models come from (FSLTL, AIG, GSX,
// MSFS stock, a flyable add-on…), each switched on or off (the user: "for
// basics based on provider"). A title's provider is its package's
// (SetModelSources, from the installed packages on disk: addons.TitleSources),
// else the patterns of traffic.modelProviders (a host without the packages:
// a director), else "Other". A title the simulator lists that no package
// on disk holds is MSFS's own (stock packages are packed). Providers whose
// titles are flyable aircraft (the user's add-ons: FlyByWire, Fenix) start
// off; the rest on. A provider off is passed over for the next choice, but
// never so far that nothing is left: then any is taken (PreferProviders).

// ProviderStock is MSFS's own models' provider.
const ProviderStock = "MSFS"

// ModelProviderItem names a provider by package folder and title patterns
// ("fsltl-*", "FSLTL*": a trailing * is a prefix, else exact; any case),
// in traffic.modelProviders.
type ModelProviderItem struct {
	Provider string   `json:"provider"`
	Packages []string `json:"packages,omitempty"`
	Titles   []string `json:"titles,omitempty"`
}

var shippedModelProviders = []ModelProviderItem{
	{Provider: "FSLTL", Packages: []string{"fsltl-*"}, Titles: []string{"FSLTL*"}},
	{Provider: "AIG", Packages: []string{"aig-*", "aigaitraffic*"}, Titles: []string{"AIG *", "AIG_*"}},
	{Provider: "GSX", Packages: []string{"fsdreamteam-*"}, Titles: []string{"FSDT_*"}},
	{Provider: "MyCrew", Packages: []string{"mycrew-*"}, Titles: []string{"MyCrew *"}},
	{Provider: ProviderStock, Packages: []string{"microsoft-*", "asobo-*", "fs-base*"}},
}

var modelProvidersNow dict.Value[[]ModelProviderItem]

func init() {
	dict.Register(dict.Keyed("traffic.modelProviders", "provider", "", "",
		func() []ModelProviderItem { return slices.Clone(shippedModelProviders) },
		func(i ModelProviderItem) string { return i.Provider },
		func(items []ModelProviderItem) { modelProvidersNow.Store(items) }))
	_ = dict.Reset("traffic.modelProviders") // the shipped copy in use
}

// ModelSource is where an installed title comes from: its package folder
// and creator, and whether it is a flyable aircraft (addons.TitleSource).
type ModelSource struct {
	Package string
	Creator string
	Flyable bool
}

var providers = struct {
	mu      sync.Mutex
	sources map[string]ModelSource // lower title → package (nil: not read)
	flyable map[string]bool        // providers with a flyable title
	on      map[string]bool        // switched by the host
}{on: map[string]bool{}}

// SetModelSources gives the installed titles' packages (lower-case title →
// source), read from disk; nil forgets them.
func SetModelSources(sources map[string]ModelSource) {
	fly := map[string]bool{}
	providers.mu.Lock()
	providers.sources = sources
	providers.mu.Unlock()
	for t, s := range sources {
		if s.Flyable {
			fly[ModelProvider(t)] = true
		}
	}
	providers.mu.Lock()
	providers.flyable = fly
	providers.mu.Unlock()
}

// ModelProvider is the provider of a simulator title.
func ModelProvider(title string) string {
	t := strings.ToLower(title)
	providers.mu.Lock()
	src, known := providers.sources[t]
	loaded := providers.sources != nil
	providers.mu.Unlock()
	items := modelProvidersNow.Load()
	if known {
		for _, it := range items {
			if matchAny(it.Packages, src.Package) {
				return it.Provider
			}
		}
		if src.Creator != "" {
			return src.Creator
		}
		return src.Package
	}
	for _, it := range items {
		if matchAny(it.Titles, title) {
			return it.Provider
		}
	}
	if loaded {
		return ProviderStock // listed by the sim, on no package on disk: packed
	}
	return "Other"
}

func matchAny(patterns []string, s string) bool {
	s = strings.ToLower(s)
	for _, p := range patterns {
		p = strings.ToLower(p)
		if pre, ok := strings.CutSuffix(p, "*"); ok {
			if strings.HasPrefix(s, pre) {
				return true
			}
		} else if s == p {
			return true
		}
	}
	return false
}

// ProviderOn reports whether a provider's models are used: as switched, by
// default on unless its titles are flyable add-ons.
func ProviderOn(provider string) bool {
	providers.mu.Lock()
	defer providers.mu.Unlock()
	if on, ok := providers.on[provider]; ok {
		return on
	}
	return !providers.flyable[provider]
}

// ProviderFlyable reports whether a provider's titles are flyable add-ons.
func ProviderFlyable(provider string) bool {
	providers.mu.Lock()
	defer providers.mu.Unlock()
	return providers.flyable[provider]
}

// SetProviders switches providers on or off (the others as they are).
func SetProviders(on map[string]bool) {
	providers.mu.Lock()
	defer providers.mu.Unlock()
	for p, v := range on {
		providers.on[p] = v
	}
}

// OnProviderFallback, when set, is told the titles taken although their
// providers are off, none of a provider on being there (a host logs it).
var OnProviderFallback func(titles []string)

func reportFallback(titles []string) {
	if f := OnProviderFallback; f != nil && len(titles) > 0 {
		f(titles)
	}
}

// PreferProviders keeps the titles whose provider is on, in order; when
// none is, all of them (fellBack true: a host logs it).
func PreferProviders(titles []string) (out []string, fellBack bool) {
	for _, t := range titles {
		if ProviderOn(ModelProvider(t)) {
			out = append(out, t)
		}
	}
	if len(out) == 0 && len(titles) > 0 {
		reportFallback(titles)
		return titles, true
	}
	return out, false
}
