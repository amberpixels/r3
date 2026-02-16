// Package r3bson provides a BSON dialect for converting r3 types to MongoDB BSON documents.
//
// Category: Data store dialect.
//
// This is the MongoDB equivalent of the r3sql package for SQL databases.
// It translates r3.FilterSpec, r3.SortSpec, and other query types
// into bson.D documents suitable for use with the MongoDB Go driver.
package r3bson
