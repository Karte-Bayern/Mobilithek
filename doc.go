// Package mobilithek provides a small Go client for retrieving Mobilithek
// subscriptions through the public subscription endpoints.
//
// The package focuses on endpoint URL construction, mTLS client-certificate
// setup, response size limits, gzip handling, and basic response metadata.
// It also includes a small generic DATEX II XML event extractor for demos and
// quick prototypes. Production projects should add source-specific DATEX II
// normalization where precise domain semantics matter.
package mobilithek
