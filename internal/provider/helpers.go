package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// objectAsOptions returns default options for Object.As() calls.
func objectAsOptions() basetypes.ObjectAsOptions {
	return basetypes.ObjectAsOptions{UnhandledNullAsEmpty: true, UnhandledUnknownAsEmpty: true}
}

// splitCompositeID splits a composite ID of the form "a/b/c" into exactly n parts.
func splitCompositeID(id string, n int) ([]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	parts := splitString(id, "/")
	if len(parts) != n {
		diags.AddAttributeError(
			path.Root("id"),
			"Invalid Import ID",
			"Expected format with "+formatInt(n)+" parts separated by '/'",
		)
	}
	return parts, diags
}

func splitString(s, sep string) []string {
	result := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if string(s[i]) == sep {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

func formatInt(n int) string {
	switch n {
	case 2:
		return "2"
	case 3:
		return "3"
	default:
		return "N"
	}
}
