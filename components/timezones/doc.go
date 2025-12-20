// Package timezones provides deterministic IANA timezone data, search helpers,
// and a small net/http handler that returns JSON options for form inputs.
//
// The default handler responds to GET and HEAD requests and supports query and
// limit parameters to filter results. The backing data is loaded from the
// embedded IANA timezone list under data/iana_timezones.txt.
package timezones
