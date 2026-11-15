package icons

import "regexp"

// gcp is the Google Cloud product icons: the core products' own icons and the product categories', which every
// other product takes (Google's guide). Each comes in a zip of its own, whose URL holds no version; the legacy
// icons, deprecated, are left out.
var gcp = kit{
	name:  "gcp",
	title: "Google Cloud",
	page:  "https://cloud.google.com/icons",
	host:  "services.google.com",
	size:  "1MB",
	terms: false,
	packages: []pack{
		{
			what:   "the core product icons zip",
			link:   regexp.MustCompile(`^https://[^"'\s<>]+/core-products-icons\.zip$`),
			iconOf: gcpCoreOf,
		},
		{
			what:   "the product category icons zip",
			link:   regexp.MustCompile(`^https://[^"'\s<>]+/category-icons\.zip$`),
			iconOf: gcpCategoryOf,
		},
	},
	version:     nil,
	builtin:     false,
	categorized: true,
	short: map[string][]string{
		"gce": {"compute-engine"},
		"gcs": {"cloud-storage"},
		"k8s": {"gke"},
	},
}

// A product's file names differ in form (CloudRun-512-color-rgb.svg, Cloud_Storage-512-color.svg), so its name
// is its directory's.
var (
	gcpCoreRe     = regexp.MustCompile(`^Unique Icons/([^/]+)/SVG/[^/]+\.svg$`)
	gcpCategoryRe = regexp.MustCompile(`^Category Icons/([^/]+)/SVG/[^/]+\.svg$`)
)

func gcpCoreOf(path string) (packed, bool) {
	if m := gcpCoreRe.FindStringSubmatch(path); m != nil {
		return named(packed{version: "", name: "", category: "Core product", dark: false}, "", m[1])
	}
	return packed{version: "", name: "", category: "", dark: false}, false
}

func gcpCategoryOf(path string) (packed, bool) {
	if m := gcpCategoryRe.FindStringSubmatch(path); m != nil {
		return named(packed{version: "", name: "", category: "Product category", dark: false}, "category/", m[1])
	}
	return packed{version: "", name: "", category: "", dark: false}, false
}
