package icons

import "regexp"

// azure is the Azure architecture icons. Their page lets the zip be downloaded only after agreeing to its terms
// (architecture diagrams, training and documentation alone), so floe fetches it only from a zip or URL given.
var azure = kit{
	name:  "azure",
	title: "Azure",
	page:  "https://learn.microsoft.com/azure/architecture/icons/",
	host:  "arch-center.azureedge.net",
	size:  "1MB",
	terms: true,
	packages: []pack{{
		what:   "the Azure icons zip",
		link:   nil,
		iconOf: azureIconOf,
	}},
	version: nil,
	short: map[string][]string{
		"acr":   {"container-registries"},
		"afd":   {"front-door-and-cdn-profiles"},
		"agw":   {"application-gateways"},
		"aks":   {"kubernetes-services"},
		"apim":  {"api-management-services"},
		"appgw": {"application-gateways"},
		"k8s":   {"kubernetes-services"},
		"kv":    {"key-vaults"},
		"nsg":   {"network-security-groups"},
		"vm":    {"virtual-machine"},
		"vmss":  {"vm-scale-sets"},
		"vnet":  {"virtual-networks"},
	},
}

// The number in front has 5 or 9 digits, and once a space after it.
var azureRe = regexp.MustCompile(`^Azure_Public_Service_Icons/Icons/([^/]+)/\d+\s*-icon-service-([^/]+)\.svg$`)

func azureIconOf(path string) (packed, bool) {
	if m := azureRe.FindStringSubmatch(path); m != nil {
		return named(packed{version: "", name: "", category: m[1], dark: false}, "", m[2])
	}
	return packed{version: "", name: "", category: "", dark: false}, false
}
