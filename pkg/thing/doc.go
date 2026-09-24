// Package thing implements the Thing domain: business objects that are often
// territorial — parcels, buildings, streets, trees — as first-class THING
// subjects with a type, an optional geometry in the Swiss national frame
// (EPSG:2056, LV95) and, for parcels and buildings, their official identifiers
// (EGRID, EGID). Cases, documents and actors attach through typed core
// relationships; every mutation is audited in the same transaction.
package thing
